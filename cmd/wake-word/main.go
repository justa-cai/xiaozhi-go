package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gen2brain/malgo"
	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

var (
	verbose = flag.Bool("verbose", false, "Enable verbose debug output")
	audioDebug = flag.Bool("audio-debug", false, "Enable audio level debugging")
	showAll = flag.Bool("show-all", false, "Show all recognized text (not just keywords)")
	fastSpeech = flag.Bool("fast-speech", false, "Enable fast speech detection optimizations")
	modelPath = flag.String("model-path", "./models/sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01", "Path to model directory")
	keywordsFile = flag.String("keywords", "./models/sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01/xiaozhi_keywords.txt", "Path to keywords file")
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	flag.Parse()

	if *verbose {
		log.Println("Verbose mode enabled")
	}

	// Setup KWS (Keyword Spotting) configuration
	kwsConfig := sherpa.KeywordSpotterConfig{}

	// Please download the models from
	// https://github.com/k2-fsa/sherpa-onnx/releases/tag/kws-models
	// Models should be placed in the directory specified by -model-path flag
	modelDir := *modelPath
	kwsConfig.ModelConfig.Transducer.Encoder = fmt.Sprintf("%s/encoder-epoch-12-avg-2-chunk-16-left-64.onnx", modelDir)
	kwsConfig.ModelConfig.Transducer.Decoder = fmt.Sprintf("%s/decoder-epoch-12-avg-2-chunk-16-left-64.onnx", modelDir)
	kwsConfig.ModelConfig.Transducer.Joiner = fmt.Sprintf("%s/joiner-epoch-12-avg-2-chunk-16-left-64.onnx", modelDir)
	kwsConfig.ModelConfig.Tokens = fmt.Sprintf("%s/tokens.txt", modelDir)

	// Use the keywords file specified by -keywords flag
	keywordsFilePath := *keywordsFile

	// Fast speech optimization: adjust keywords threshold for better sensitivity
	if *fastSpeech {
		log.Println("Fast speech mode enabled - using optimized keywords file")
		// Use fast speech keywords file when fast-speech flag is enabled
		fastKeywordsPath := *modelPath + "/xiaozhi_keywords_fast_speech.txt"
		if FileExists(fastKeywordsPath) {
			keywordsFilePath = fastKeywordsPath
			log.Printf("Using fast speech keywords: %s", fastKeywordsPath)
		} else {
			log.Printf("Fast speech keywords file not found: %s", fastKeywordsPath)
		}
	}

	// Check if keywords file exists
	if !FileExists(keywordsFilePath) {
		log.Fatal("Keywords file does not exist: ", keywordsFilePath)
	}

	kwsConfig.KeywordsFile = keywordsFilePath
	kwsConfig.ModelConfig.NumThreads = 2 // Increase for better performance
	kwsConfig.ModelConfig.Debug = 1
	kwsConfig.ModelConfig.Provider = "cpu"

	// Create the keyword spotter
	spotter := sherpa.NewKeywordSpotter(&kwsConfig)
	if spotter == nil {
		log.Fatal("Failed to create keyword spotter")
	}
	defer sherpa.DeleteKeywordSpotter(spotter)

	
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

	// Print diagnostic information
	fmt.Println("=== Wake Word Detection Diagnostics ===")
	fmt.Printf("Keywords file: %s\n", keywordsFilePath)
	fmt.Printf("Model files:\n")
	fmt.Printf("  - Encoder: %s\n", kwsConfig.ModelConfig.Transducer.Encoder)
	fmt.Printf("  - Decoder: %s\n", kwsConfig.ModelConfig.Transducer.Decoder)
	fmt.Printf("  - Joiner: %s\n", kwsConfig.ModelConfig.Transducer.Joiner)
	fmt.Printf("  - Tokens: %s\n", kwsConfig.ModelConfig.Tokens)
	fmt.Printf("Audio settings: 16kHz, 1 channel, 16-bit\n")
	fmt.Printf("Debug mode: enabled\n")
	fmt.Printf("Provider: %s\n", kwsConfig.ModelConfig.Provider)
	fmt.Printf("Verbose logging: %v\n", *verbose)
	fmt.Printf("Audio debugging: %v\n", *audioDebug)
	fmt.Printf("Show all recognized text: %v\n", *showAll)
	fmt.Printf("Fast speech mode: %v\n", *fastSpeech)
	fmt.Println("=========================================")

	// Test keywords file loading and display keywords for debugging
	fmt.Printf("Loading keywords from: %s\n", keywordsFilePath)
	if *verbose || *showAll {
		displayKeywordsFile(keywordsFilePath)
	}

	fmt.Println("Initializing audio device...")

	var frameCount uint32

	var speechDetected bool

	onRecvFrames := func(_, pSample []byte, framecount uint32) {
		frameCount++
		samples := samplesInt16ToFloat(pSample)

		// Calculate audio level for debugging
		var sum float32
		for _, sample := range samples {
			sum += float32(sample * sample)
		}
		rms := float32(0)
		if len(samples) > 0 {
			rms = float32(math.Sqrt(float64(sum / float32(len(samples)))))
		}
		
		// Detect speech activity (simple threshold-based)
		isSpeech := rms > 0.01 // Adjust threshold as needed
		speechStateChanged := speechDetected != isSpeech
		speechDetected = isSpeech

		// Enhanced audio debugging
		if *audioDebug && frameCount%100 == 0 {
			log.Printf("Audio level: %.4f, Frame: %d, Samples: %d, Speech: %v", rms, frameCount, len(samples), isSpeech)
		}

		// Log speech state changes when show-all is enabled
		if *showAll && speechStateChanged {
			if isSpeech {
				log.Printf("Speech started (level: %.4f, frame: %d)", rms, frameCount)
			} else {
				log.Printf("Speech ended (level: %.4f, frame: %d)", rms, frameCount)
			}
		}

		// Direct processing - feed all audio to the keyword spotter
		stream.AcceptWaveform(16000, samples)

		// Process the keyword spotter in a loop to get all possible detections
		detectionCount := 0
		for spotter.IsReady(stream) {
			spotter.Decode(stream)
			result := spotter.GetResult(stream)

			// Show detection details when show-all flag is enabled
			if *showAll && *verbose {
				if result.Keyword != "" {
					log.Printf("DETECTION: Keyword='%s' (frame: %d, audio_level: %.4f)", result.Keyword, frameCount, rms)
				} else {
					log.Printf("DETECTION: Processing audio... (frame: %d, audio_level: %.4f, speech: %v)", frameCount, rms, isSpeech)
				}
			}

			// Fast speech detection: add more frequent processing for rapid speech
			if detectionCount > 0 && *verbose {
				log.Printf("FAST DETECTION: %d processing cycles in current speech segment (frame: %d)", detectionCount, frameCount)
			}

			if result.Keyword != "" {
				log.Printf("WAKE WORD DETECTED: %s (frame: %d, audio_level: %.4f)", result.Keyword, frameCount, rms)
				// Reset the stream after detecting a keyword to avoid repeated detections
				spotter.Reset(stream)

				// Here you can trigger your wake word response logic
				triggerWakeWordAction(result.Keyword)
			}
			detectionCount++
		}

		// Log processing activity every 1000 frames
		if *verbose && frameCount%1000 == 0 {
			log.Printf("Processed %d frames, %d detection cycles in this batch, audio_level: %.4f", frameCount, detectionCount, rms)
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

	fmt.Println("Wake word detection started. Listening for '你好小智' or '小智同学'. Press Ctrl+C to exit.")
	
	// Wait for interrupt signal to stop
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	
	fmt.Println("\nStopping wake word detection...")
	device.Uninit()
}

func displayKeywordsFile(filePath string) {
	fmt.Println("\n=== Keywords Configuration ===")
	content, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading keywords file: %v\n", err)
		return
	}

	lines := strings.Split(string(content), "\n")
	fmt.Printf("Found %d keyword configurations:\n", len(lines))
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			fmt.Printf("  %d. %s\n", i+1, strings.TrimSpace(line))
		}
	}
	fmt.Println("================================\n")
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