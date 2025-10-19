#!/bin/bash

# Script to help download required models for wake word detection

echo "Downloading required models for wake word detection..."

# Create models directory if it doesn't exist
mkdir -p models

echo "Note: The KWS model is quite large (~100MB), so the download may take a few minutes."
echo "Downloading KWS model from https://github.com/k2-fsa/sherpa-onnx/releases/tag/kws-models"
echo "Please manually download and extract the model to: ./models/sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01/"
echo "- You need: encoder-epoch-12-avg-2-chunk-16-left-64.onnx"
echo "- You need: decoder-epoch-12-avg-2-chunk-16-left-64.onnx" 
echo "- You need: joiner-epoch-12-avg-2-chunk-16-left-64.onnx"
echo "- You need: tokens.txt"

echo ""
echo "For VAD model, choose one and download to the models/ directory:"
echo "1. silero_vad.onnx: https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/silero_vad.onnx"
echo "2. ten-vad.onnx: https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/ten-vad.onnx"

echo ""
echo "After downloading the models, you can run the wake word detection with:"
echo "  ./bin/wake-word"
echo ""
echo "Or using make:"
echo "  make wake-word-run"