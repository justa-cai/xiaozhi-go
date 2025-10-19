# Main Makefile for xiaozhi

.PHONY: build run clean deps wake-word wake-word-run

# Default target
all: build

# Build all binaries
build:
	go build -o bin/xiaozhi cmd/client/main.go

# Run the main application
run: build
	./bin/xiaozhi

# Build the wake word detection binary
wake-word:
	go build -o bin/wake-word cmd/wake-word/main.go

# Run the wake word detection
wake-word-run: wake-word
	./bin/wake-word

# Install dependencies
deps:
	go mod tidy

# Clean build artifacts
clean:
	rm -f bin/xiaozhi
	rm -f bin/wake-word

# Download required models (you may need to adjust these URLs)
models:
	@echo "Please download the required models manually:"
	@echo "1. KWS model: https://github.com/k2-fsa/sherpa-onnx/releases/tag/kws-models"
	@echo "2. VAD model: https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/"
	@echo "Extract KWS model to: ./sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01/"
	@echo "Place either silero_vad.onnx or ten-vad.onnx in the root directory"