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

	onRecvFrames := func(_, pSample []byte, framecount uint32) {
		samples := samplesInt16ToFloat(pSample)

		// Direct processing - feed all audio to the keyword spotter
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