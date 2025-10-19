# Wake Word Detection for Xiaozhi

This module implements wake word detection for "你好小智" and "小智同学" using Sherpa-onnx keyword spotting (KWS) with voice activity detection (VAD) to optimize performance.

## Features

- Real-time wake word detection for "你好小智" and "小智同学"
- Voice activity detection (VAD) to only process audio when speech is detected (optional, controlled via command-line)
- Optimized for low-latency response
- Proper resource management and cleanup

## Prerequisites

You need to download the following models:

1. **KWS Model**: Download from [sherpa-onnx kws-models](https://github.com/k2-fsa/sherpa-onnx/releases/tag/kws-models)
   - Extract to: `./models/sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01/`
   - Files needed: `encoder-epoch-12-avg-2-chunk-16-left-64.onnx`, `decoder-epoch-12-avg-2-chunk-16-left-64.onnx`, `joiner-epoch-12-avg-2-chunk-16-left-64.onnx`, `tokens.txt`

2. **VAD Model**: Choose one of the following, place in root directory or `./models/`:
   - [silero_vad.onnx](https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/silero_vad.onnx)
   - [ten-vad.onnx](https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/ten-vad.onnx)

## Building and Running

### Using Make:

```bash
# Build the wake word detection binary
make wake-word

# Run the wake word detection without VAD (default)
make wake-word-run

# Run the wake word detection with VAD enabled
./bin/wake-word -vad
```

### Direct Go commands:

```bash
# Build
go build -o bin/wake-word cmd/wake-word/main.go

# Run without VAD (default behavior)
./bin/wake-word

# Run with VAD enabled
./bin/wake-word -vad
```

## How it Works

1. The system continuously listens to audio input using the microphone
2. When VAD is enabled: VAD (Voice Activity Detection) detects when speech is present, and the KWS (Keyword Spotting) engine processes the audio to identify wake words
3. When VAD is disabled: All audio is processed directly by the KWS engine
4. When a wake word ("你好小智" or "小智同学") is detected, the system logs the event and calls the `triggerWakeWordAction` function
5. When VAD is enabled, after speech ends, the system resets to avoid repeated detections

## Customization

You can modify the wake words by changing the `keywords` variable in the code. The format is:
```
"phoneme_transcription @ keyword"
```

Multiple keywords can be separated with `/`.

## Troubleshooting

- If you get errors about missing models, ensure you've downloaded and placed them in the correct locations
- If the audio device fails to initialize, check that you have microphone permissions
- Wake word detection accuracy may vary depending on the model and acoustic conditions