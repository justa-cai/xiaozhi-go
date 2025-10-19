#!/bin/bash

# Script to convert keywords using sherpa-onnx-cli

echo "Converting keywords using sherpa-onnx-cli..."

# Check if sherpa-onnx-cli is available
if ! command -v sherpa-onnx-cli &> /dev/null; then
    echo "Error: sherpa-onnx-cli is not available. Please install it first."
    echo "You can install it with: go install github.com/k2-fsa/sherpa-onnx-go/cmd/sherpa-onnx-cli@latest"
    exit 1
fi

# Convert the keywords
echo "Converting keywords_raw.txt to keywords.txt format..."
sherpa-onnx-cli text2token \
  --tokens ./models/sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01/tokens.txt \
  --tokens-type ppinyin \
  ./keywords_raw.txt ./models/sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01/xiaozhi_keywords.txt

if [ $? -eq 0 ]; then
    echo "Keywords converted successfully!"
    echo "Converted keywords saved to: ./models/sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01/xiaozhi_keywords.txt"
    echo "Content:"
    cat ./models/sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01/xiaozhi_keywords.txt
else
    echo "Error: Failed to convert keywords"
    exit 1
fi