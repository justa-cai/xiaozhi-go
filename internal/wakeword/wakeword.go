package wakeword

import (
	"fmt"
	"os"
	"sync"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
	"github.com/sirupsen/logrus"
)

// WakeWordDetector handles wake word detection
type WakeWordDetector struct {
	spotter           *sherpa.KeywordSpotter
	stream            *sherpa.OnlineStream
	callback          func(string) // Callback when wake word is detected
	mu                sync.Mutex
	running           bool
	stopCh            chan struct{}
}

// NewWakeWordDetector creates a new wake word detector instance
func NewWakeWordDetector(callback func(string)) (*WakeWordDetector, error) {
	w := &WakeWordDetector{
		callback: callback,
		stopCh:   make(chan struct{}),
	}

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
		return nil, fmt.Errorf("keywords file does not exist: %s", keywordsFile)
	}

	kwsConfig.KeywordsFile = keywordsFile
	kwsConfig.ModelConfig.NumThreads = 1
	kwsConfig.ModelConfig.Debug = 1

	// Create the keyword spotter
	spotter := sherpa.NewKeywordSpotter(&kwsConfig)
	if spotter == nil {
		return nil, fmt.Errorf("failed to create keyword spotter")
	}

	w.spotter = spotter

	// Create keyword stream - keywords are already loaded from the file in the spotter config
	stream := sherpa.NewKeywordStream(spotter)
	if stream == nil {
		w.cleanup()
		return nil, fmt.Errorf("failed to create keyword stream")
	}

	w.stream = stream

	return w, nil
}

// Start starts the wake word detection (just sets internal state)
func (w *WakeWordDetector) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		return fmt.Errorf("wake word detector already running")
	}

	w.running = true

	logrus.Info("Wake word detection initialized. Listening for '你好小智' or '小智同学'.")

	return nil
}

// Stop stops the wake word detection
func (w *WakeWordDetector) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return nil
	}

	close(w.stopCh)
	w.running = false

	return nil
}

// IsRunning returns whether the detector is currently running
func (w *WakeWordDetector) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

// ProcessAudioData processes audio data for wake word detection
func (w *WakeWordDetector) ProcessAudioData(pcmData []int16) {
	samples := make([]float32, len(pcmData))
	for i, sample := range pcmData {
		samples[i] = float32(sample) / 32768.0
	}

	// Direct processing - feed all audio to the keyword spotter
	w.stream.AcceptWaveform(16000, samples)

	// Process the keyword spotter in a loop to get all possible detections
	for w.spotter.IsReady(w.stream) {
		w.spotter.Decode(w.stream)
		result := w.spotter.GetResult(w.stream)

		if result.Keyword != "" {
			logrus.Infof("WAKE WORD DETECTED: %s", result.Keyword)
			// Reset the stream after detecting a keyword to avoid repeated detections
			w.spotter.Reset(w.stream)

			// Call the callback function
			if w.callback != nil {
				w.callback(result.Keyword)
			}
		}
	}
}


// onRecvFrames handles incoming audio frames (for backward compatibility if needed)
func (w *WakeWordDetector) onRecvFrames(_, pSample []byte, framecount uint32) {
	samples := samplesInt16ToFloat(pSample)

	// Direct processing - feed all audio to the keyword spotter
	w.stream.AcceptWaveform(16000, samples)

	// Process the keyword spotter in a loop to get all possible detections
	for w.spotter.IsReady(w.stream) {
		w.spotter.Decode(w.stream)
		result := w.spotter.GetResult(w.stream)

		if result.Keyword != "" {
			logrus.Infof("WAKE WORD DETECTED: %s", result.Keyword)
			// Reset the stream after detecting a keyword to avoid repeated detections
			w.spotter.Reset(w.stream)

			// Call the callback function
			if w.callback != nil {
				w.callback(result.Keyword)
			}
		}
	}
}

// cleanup releases all resources
func (w *WakeWordDetector) cleanup() {
	if w.spotter != nil {
		sherpa.DeleteKeywordSpotter(w.spotter)
		w.spotter = nil
	}

	if w.stream != nil {
		sherpa.DeleteOnlineStream(w.stream)
		w.stream = nil
	}
}

// samplesInt16ToFloat converts int16 samples to float32
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

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	return false
}

// TriggerWakeWordAction is a helper function to trigger wake word response logic
func TriggerWakeWordAction(keyword string) {
	logrus.Infof("Wake word '%s' detected! Activating assistant...", keyword)
	// Add your wake word response logic here
	// For example, you could:
	// - Trigger an audio response
	// - Send a signal to other parts of your application
	// - Start recording a command
	// - etc.
}