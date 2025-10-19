package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gen2brain/malgo"
	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	
	// Define command-line flags
	vadEnabled := flag.Bool("vad", false, "Enable Voice Activity Detection (default: false)")
	flag.Parse()

	// Setup KWS (Keyword Spotting) configuration
	kwsConfig := sherpa.KeywordSpotterConfig{}

	// Please download the models from
	// https://github.com/k2-fsa/sherpa-onnx/releases/tag/kws-models
	// Models should be placed in ./models/sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01/
	kwsConfig.ModelConfig.Transducer.Encoder = "./models/sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01/encoder-epoch-12-avg-2-chunk-16-left-64.onnx"
	kwsConfig.ModelConfig.Transducer.Decoder = "./models/sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01/decoder-epoch-12-avg-2-chunk-16-left-64.onnx"
	kwsConfig.ModelConfig.Transducer.Joiner = "./models/sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01/joiner-epoch-12-avg-2-chunk-16-left-64.onnx"
	kwsConfig.ModelConfig.Tokens = "./models/sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01/tokens.txt"
	
	// Use the pre-converted keywords file for xiaozhi wake words
	keywordsFile := "./models/sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01/xiaozhi_keywords.txt"
	
	// Check if keywords file exists
	if !FileExists(keywordsFile) {
		log.Fatal("Keywords file does not exist: ", keywordsFile)
	}
	
	kwsConfig.KeywordsFile = keywordsFile
	kwsConfig.ModelConfig.NumThreads = 1
	kwsConfig.ModelConfig.Debug = 1

	// Create the keyword spotter
	spotter := sherpa.NewKeywordSpotter(&kwsConfig)
	if spotter == nil {
		log.Fatal("Failed to create keyword spotter")
	}
	defer sherpa.DeleteKeywordSpotter(spotter)

	var vad *sherpa.VoiceActivityDetector
	var vadConfig sherpa.VadModelConfig
	var windowSize int
	var bufferSizeInSeconds float32 = 5
	var buffer *sherpa.CircularBuffer

	if *vadEnabled {
		// Setup VAD (Voice Activity Detection) configuration
		vadConfig = sherpa.VadModelConfig{}

		// Try to use silero_vad.onnx first, then ten-vad.onnx
		// First check in the models directory, then in the root directory
		if FileExists("./models/silero_vad.onnx") {
			fmt.Println("Using silero-vad model from models directory")
			vadConfig.SileroVad.Model = "./models/silero_vad.onnx"
			vadConfig.SileroVad.Threshold = 0.5
			vadConfig.SileroVad.MinSilenceDuration = 0.5
			vadConfig.SileroVad.MinSpeechDuration = 0.25
			vadConfig.SileroVad.MaxSpeechDuration = 10
			vadConfig.SileroVad.WindowSize = 512
		} else if FileExists("./models/ten-vad.onnx") {
			fmt.Println("Using ten-vad model from models directory")
			vadConfig.TenVad.Model = "./models/ten-vad.onnx"
			vadConfig.TenVad.Threshold = 0.5
			vadConfig.TenVad.MinSilenceDuration = 0.5
			vadConfig.TenVad.MinSpeechDuration = 0.25
			vadConfig.TenVad.MaxSpeechDuration = 10
			vadConfig.TenVad.WindowSize = 256
		} else if FileExists("./silero_vad.onnx") {
			fmt.Println("Using silero-vad model from root directory")
			vadConfig.SileroVad.Model = "./silero_vad.onnx"
			vadConfig.SileroVad.Threshold = 0.5
			vadConfig.SileroVad.MinSilenceDuration = 0.5
			vadConfig.SileroVad.MinSpeechDuration = 0.25
			vadConfig.SileroVad.MaxSpeechDuration = 10
			vadConfig.SileroVad.WindowSize = 512
		} else if FileExists("./ten-vad.onnx") {
			fmt.Println("Using ten-vad model from root directory")
			vadConfig.TenVad.Model = "./ten-vad.onnx"
			vadConfig.TenVad.Threshold = 0.5
			vadConfig.TenVad.MinSilenceDuration = 0.5
			vadConfig.TenVad.MinSpeechDuration = 0.25
			vadConfig.TenVad.MaxSpeechDuration = 10
			vadConfig.TenVad.WindowSize = 256
		} else {
			fmt.Println("Error: Please download either silero_vad.onnx or ten-vad.onnx to the models directory or root directory")
			fmt.Println("You can download them from: https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/")
			return
		}

		vadConfig.SampleRate = 16000
		vadConfig.NumThreads = 1
		vadConfig.Provider = "cpu"
		vadConfig.Debug = 1

		windowSize = int(vadConfig.SileroVad.WindowSize)
		if vadConfig.TenVad.Model != "" {
			windowSize = int(vadConfig.TenVad.WindowSize)
		}

		// Create VAD instance
		vad = sherpa.NewVoiceActivityDetector(&vadConfig, bufferSizeInSeconds)
		if vad == nil {
			log.Fatal("Failed to create voice activity detector")
		}
		defer sherpa.DeleteVoiceActivityDetector(vad)

		// Create circular buffer for audio samples
		buffer = sherpa.NewCircularBuffer(10 * int(vadConfig.SampleRate))
		if buffer == nil {
			log.Fatal("Failed to create circular buffer")
		}
		defer sherpa.DeleteCircularBuffer(buffer)
	}

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

	// Create keyword stream - keywords are already loaded from the file in the spotter config
	stream := sherpa.NewKeywordStream(spotter)
	if stream == nil {
		log.Fatal("Failed to create keyword stream")
	}
	defer sherpa.DeleteOnlineStream(stream)

	fmt.Println("Initializing audio device...")

	// Track speech state (only used when VAD is enabled)
	printed := false
	speechDetected := false

	onRecvFrames := func(_, pSample []byte, framecount uint32) {
		samples := samplesInt16ToFloat(pSample)
		
		if *vadEnabled {
			// VAD-enabled processing
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
						log.Print("Speech detected - starting keyword spotting")
						speechDetected = true
					}

					if !vad.IsSpeech() {
						printed = false
						if speechDetected {
							log.Print("Speech ended - resetting keyword spotter")
							speechDetected = false
							// Reset the keyword stream when speech ends to clear any partial detections
							spotter.Reset(stream)
						}
					}

					// Process speech segments if detected
					for !vad.IsEmpty() {
						speechSegment := vad.Front()
						vad.Pop()

						duration := float32(len(speechSegment.Samples)) / float32(vadConfig.SampleRate)
						log.Printf("Processing speech segment: %.2f seconds", duration)

						// Feed the speech segment to the keyword spotter only when speech is detected
						stream.AcceptWaveform(int(vadConfig.SampleRate), speechSegment.Samples)

						// Process the keyword spotter in a loop to get all possible detections
						for spotter.IsReady(stream) {
							spotter.Decode(stream)
							result := spotter.GetResult(stream)
							
							if result.Keyword != "" {
								log.Printf("WAKE WORD DETECTED: %s", result.Keyword)
								// Reset the stream after detecting a keyword to avoid repeated detections
								spotter.Reset(stream)
								
								// Here you can trigger your wake word response logic
								triggerWakeWordAction(result.Keyword)
							}
						}
					}
				}
			}
		} else {
			// Direct processing without VAD - feed all audio to the keyword spotter
			stream.AcceptWaveform(16000, samples)

			// Process the keyword spotter in a loop to get all possible detections
			for spotter.IsReady(stream) {
				spotter.Decode(stream)
				result := spotter.GetResult(stream)
				
				if result.Keyword != "" {
					log.Printf("WAKE WORD DETECTED: %s", result.Keyword)
					// Reset the stream after detecting a keyword to avoid repeated detections
					spotter.Reset(stream)
					
					// Here you can trigger your wake word response logic
					triggerWakeWordAction(result.Keyword)
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

	if *vadEnabled {
		fmt.Println("Wake word detection started with VAD enabled. Listening for '你好小智' or '小智同学'. Press Ctrl+C to exit.")
	} else {
		fmt.Println("Wake word detection started without VAD. Listening for '你好小智' or '小智同学'. Press Ctrl+C to exit.")
	}
	
	// Wait for interrupt signal to stop
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	
	fmt.Println("\nStopping wake word detection...")
	device.Uninit()
}

func triggerWakeWordAction(keyword string) {
	fmt.Printf("Wake word '%s' detected! Activating assistant...\n", keyword)
	// Add your wake word response logic here
	// For example, you could:
	// - Trigger an audio response
	// - Send a signal to other parts of your application
	// - Start recording a command
	// - etc.
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