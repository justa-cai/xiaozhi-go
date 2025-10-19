package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gen2brain/malgo"
	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// Define command-line flags for VAD configuration
	model := flag.String("model", "", "VAD model path (silero_vad.onnx or ten-vad.onnx)")
	threshold := flag.Float64("threshold", 0.5, "VAD threshold (default: 0.5)")
	minSilenceDuration := flag.Float64("min-silence", 0.5, "Minimum silence duration in seconds (default: 0.5)")
	minSpeechDuration := flag.Float64("min-speech", 0.25, "Minimum speech duration in seconds (default: 0.25)")
	maxSpeechDuration := flag.Float64("max-speech", 10, "Maximum speech duration in seconds (default: 10)")
	windowSizeFlag := flag.Int("window-size", 0, "Window size (default: 512 for silero, 256 for ten)")
	outputDir := flag.String("output-dir", "./speech_segments", "Directory to save speech segments as WAV files")
	saveAudio := flag.Bool("save-audio", false, "Enable saving speech segments to WAV files (default: false)")
	flag.Parse()

	// Setup VAD (Voice Activity Detection) configuration
	vadConfig := sherpa.VadModelConfig{}

	// Determine which model to use
	modelPath := *model
	if modelPath == "" {
		// Default to silero_vad.onnx, check different locations
		if FileExists("./models/silero_vad.onnx") {
			modelPath = "./models/silero_vad.onnx"
			fmt.Println("Using default silero-vad model from models directory")
		} else if FileExists("./silero_vad.onnx") {
			modelPath = "./silero_vad.onnx"
			fmt.Println("Using default silero-vad model from root directory")
		} else if FileExists("./models/ten-vad.onnx") {
			modelPath = "./models/ten-vad.onnx"
			fmt.Println("Using ten-vad model from models directory (silero-vad not found)")
		} else if FileExists("./ten-vad.onnx") {
			modelPath = "./ten-vad.onnx"
			fmt.Println("Using ten-vad model from root directory (silero-vad not found)")
		} else {
			fmt.Println("Error: No VAD model found. Please download silero_vad.onnx (recommended) or ten-vad.onnx")
			fmt.Println("You can download them from: https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/")
			fmt.Println("Or specify the model path with -model flag")
			fmt.Println("Example usage:")
			fmt.Println("  go run ./cmd/vad/ -model ./models/silero_vad.onnx -save-audio -output-dir ./my_speech_segments")
			return
		}
	}

	// Configure the appropriate VAD model based on the filename
	if contains(modelPath, "silero") {
		vadConfig.SileroVad.Model = modelPath
		vadConfig.SileroVad.Threshold = float32(*threshold)
		vadConfig.SileroVad.MinSilenceDuration = float32(*minSilenceDuration)
		vadConfig.SileroVad.MinSpeechDuration = float32(*minSpeechDuration)
		vadConfig.SileroVad.MaxSpeechDuration = float32(*maxSpeechDuration)
		if *windowSizeFlag > 0 {
			vadConfig.SileroVad.WindowSize = *windowSizeFlag
		} else {
			vadConfig.SileroVad.WindowSize = 512
		}
		fmt.Printf("Configured Silero VAD: threshold=%.2f, window_size=%d\n",
			vadConfig.SileroVad.Threshold, vadConfig.SileroVad.WindowSize)
	} else {
		vadConfig.TenVad.Model = modelPath
		vadConfig.TenVad.Threshold = float32(*threshold)
		vadConfig.TenVad.MinSilenceDuration = float32(*minSilenceDuration)
		vadConfig.TenVad.MinSpeechDuration = float32(*minSpeechDuration)
		vadConfig.TenVad.MaxSpeechDuration = float32(*maxSpeechDuration)
		if *windowSizeFlag > 0 {
			vadConfig.TenVad.WindowSize = *windowSizeFlag
		} else {
			vadConfig.TenVad.WindowSize = 256
		}
		fmt.Printf("Configured Ten VAD: threshold=%.2f, window_size=%d\n",
			vadConfig.TenVad.Threshold, vadConfig.TenVad.WindowSize)
	}

	vadConfig.SampleRate = 16000
	vadConfig.NumThreads = 1
	vadConfig.Provider = "cpu"
	vadConfig.Debug = 1

	var bufferSizeInSeconds float32 = 5
	var windowSize int
	if vadConfig.SileroVad.Model != "" {
		windowSize = int(vadConfig.SileroVad.WindowSize)
	} else {
		windowSize = int(vadConfig.TenVad.WindowSize)
	}

	// Create VAD instance
	vad := sherpa.NewVoiceActivityDetector(&vadConfig, bufferSizeInSeconds)
	if vad == nil {
		log.Fatal("Failed to create voice activity detector")
	}
	defer sherpa.DeleteVoiceActivityDetector(vad)

	// Create circular buffer for audio samples
	buffer := sherpa.NewCircularBuffer(10 * int(vadConfig.SampleRate))
	if buffer == nil {
		log.Fatal("Failed to create circular buffer")
	}
	defer sherpa.DeleteCircularBuffer(buffer)

	// Initialize audio context
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, func(message string) {
		fmt.Printf("Audio LOG <%v>\n", message)
	})
	if err != nil {
		log.Fatal("Failed to initialize audio context:", err)
	}
	defer func() {
		_ = ctx.Uninit()
		ctx.Free()
	}()

	// Configure audio device
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Duplex)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = 1
	deviceConfig.Playback.Format = malgo.FormatS16
	deviceConfig.Playback.Channels = 1
	deviceConfig.SampleRate = 16000
	deviceConfig.Alsa.NoMMap = 1

	// Create output directory for speech segments if audio saving is enabled
	if *saveAudio {
		if err := CreateOutputDirectory(*outputDir); err != nil {
			log.Fatal("Failed to create output directory:", err)
		}
		fmt.Printf("Speech segments will be saved to: %s\n", *outputDir)
	} else {
		fmt.Println("Audio saving is disabled. Use -save-audio to enable.")
	}

	fmt.Println("Initializing VAD audio device...")

	// Track speech state
	printed := false
	speechDetected := false
	speechSegmentCount := 0
	currentSpeechBuffer := make([]float32, 0)

	onRecvFrames := func(_, pSample []byte, framecount uint32) {
		samples := samplesInt16ToFloat(pSample)

		if buffer != nil {
			buffer.Push(samples)

			// Process audio in window chunks
			for buffer.Size() >= windowSize {
				head := buffer.Head()
				s := buffer.Get(head, windowSize)
				buffer.Pop(windowSize)

				// Feed audio to VAD
				vad.AcceptWaveform(s)

				// Check if speech is detected
				if vad.IsSpeech() && !printed {
					printed = true
					log.Print("Speech detected - starting to record")
					speechDetected = true
					currentSpeechBuffer = make([]float32, 0)
				}

				if !vad.IsSpeech() {
					printed = false
					if speechDetected {
						if *saveAudio {
							log.Print("Speech ended - saving speech segment")
						} else {
							log.Print("Speech ended")
						}
						speechDetected = false

						// Save the collected speech buffer to WAV file if audio saving is enabled
						if *saveAudio && len(currentSpeechBuffer) > 0 {
							speechSegmentCount++
							duration := float32(len(currentSpeechBuffer)) / float32(vadConfig.SampleRate)
							filename := GenerateFilename(*outputDir, speechSegmentCount)

							log.Printf("Saving speech segment #%d: %.2f seconds, %d samples to %s",
								speechSegmentCount, duration, len(currentSpeechBuffer), filename)

							if err := SaveFloat32ToWAV(filename, currentSpeechBuffer, int(vadConfig.SampleRate)); err != nil {
								log.Printf("Error saving WAV file: %v", err)
							} else {
								log.Printf("Successfully saved: %s", filename)
							}
						}

						currentSpeechBuffer = make([]float32, 0)
					}
				}

				// Add current audio samples to speech buffer if speech is detected
				if speechDetected {
					currentSpeechBuffer = append(currentSpeechBuffer, s...)
				}

				// Process speech segments if detected
				for !vad.IsEmpty() {
					speechSegment := vad.Front()
					vad.Pop()

					speechSegmentCount++
					duration := float32(len(speechSegment.Samples)) / float32(vadConfig.SampleRate)

					log.Printf("VAD speech segment #%d: %.2f seconds, %d samples",
						speechSegmentCount, duration, len(speechSegment.Samples))

					// Save VAD-detected speech segment to WAV file if audio saving is enabled
					if *saveAudio {
						filename := GenerateFilename(*outputDir, speechSegmentCount)
						if err := SaveFloat32ToWAV(filename, speechSegment.Samples, int(vadConfig.SampleRate)); err != nil {
							log.Printf("Error saving VAD WAV file: %v", err)
						} else {
							log.Printf("Successfully saved VAD segment: %s", filename)
						}
					}
				}
			}
		}
	}

	captureCallbacks := malgo.DeviceCallbacks{
		Data: onRecvFrames,
	}

	device, err := malgo.InitDevice(ctx.Context, deviceConfig, captureCallbacks)
	if err != nil {
		log.Fatal("Failed to initialize audio device:", err)
	}

	err = device.Start()
	if err != nil {
		log.Fatal("Failed to start audio device:", err)
	}

	fmt.Printf("VAD testing started. Model: %s\n", modelPath)
	fmt.Printf("Configuration: threshold=%.2f, min_silence=%.2fs, min_speech=%.2fs, max_speech=%.2fs\n",
		*threshold, *minSilenceDuration, *minSpeechDuration, *maxSpeechDuration)
	if *saveAudio {
		fmt.Printf("Output directory: %s\n", *outputDir)
		fmt.Println("Listening for speech and saving segments to WAV files... Press Ctrl+C to exit.")
	} else {
		fmt.Println("Listening for speech... (Audio saving disabled. Use -save-audio to enable.) Press Ctrl+C to exit.")
	}

	// Wait for interrupt signal to stop
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Printf("\nVAD testing completed. Processed %d speech segments.\n", speechSegmentCount)
	device.Uninit()
}

func samplesInt16ToFloat(inSamples []byte) []float32 {
	numSamples := len(inSamples) / 2
	outSamples := make([]float32, numSamples)

	for i := 0; i != numSamples; i++ {
		// Decode two bytes into an int16 using bit manipulation
		s16 := int16(inSamples[2*i]) | int16(inSamples[2*i+1])<<8
		outSamples[i] = float32(s16) / 32768
	}

	return outSamples
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		   (s == substr ||
		    s[:len(substr)] == substr ||
		    s[len(s)-len(substr):] == substr ||
		    indexOfSubstring(s, substr) >= 0)
}

func indexOfSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// WAV header structure
type WAVHeader struct {
	ChunkID       [4]byte
	ChunkSize     uint32
	Format        [4]byte
	Subchunk1ID   [4]byte
	Subchunk1Size uint32
	AudioFormat   uint16
	NumChannels   uint16
	SampleRate    uint32
	ByteRate      uint32
	BlockAlign    uint16
	BitsPerSample uint16
	Subchunk2ID   [4]byte
	Subchunk2Size uint32
}

// SaveFloat32ToWAV saves float32 audio samples to a WAV file
func SaveFloat32ToWAV(filename string, samples []float32, sampleRate int) error {
	// Convert float32 samples to int16
	int16Samples := make([]int16, len(samples))
	for i, sample := range samples {
		// Clamp to [-1.0, 1.0] and convert to int16
		if sample > 1.0 {
			sample = 1.0
		} else if sample < -1.0 {
			sample = -1.0
		}
		int16Samples[i] = int16(sample * 32767)
	}

	// Calculate sizes
	dataSize := len(int16Samples) * 2 // 16-bit samples = 2 bytes per sample
	fileSize := 36 + dataSize

	// Create WAV header
	header := WAVHeader{
		ChunkID:       [4]byte{'R', 'I', 'F', 'F'},
		ChunkSize:     uint32(fileSize),
		Format:        [4]byte{'W', 'A', 'V', 'E'},
		Subchunk1ID:   [4]byte{'f', 'm', 't', ' '},
		Subchunk1Size: 16,
		AudioFormat:   1, // PCM
		NumChannels:   1, // Mono
		SampleRate:    uint32(sampleRate),
		ByteRate:      uint32(sampleRate * 2), // SampleRate * NumChannels * BitsPerSample/8
		BlockAlign:    2,                     // NumChannels * BitsPerSample/8
		BitsPerSample: 16,
		Subchunk2ID:   [4]byte{'d', 'a', 't', 'a'},
		Subchunk2Size: uint32(dataSize),
	}

	// Create file
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write header
	if err := binary.Write(file, binary.LittleEndian, header); err != nil {
		return err
	}

	// Write data
	for _, sample := range int16Samples {
		if err := binary.Write(file, binary.LittleEndian, sample); err != nil {
			return err
		}
	}

	return nil
}

// CreateOutputDirectory creates the output directory if it doesn't exist
func CreateOutputDirectory(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// GenerateFilename generates a unique filename for a speech segment
func GenerateFilename(outputDir string, segmentIndex int) string {
	timestamp := time.Now().Format("20060102_150405_000000")
	return filepath.Join(outputDir, fmt.Sprintf("speech_%s_%03d.wav", timestamp, segmentIndex))
}