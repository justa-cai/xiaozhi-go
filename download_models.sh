#!/bin/bash
# Script to download required models for wake word detection
set -e

# Base URLs
KWS_BASE_URL="https://github.com/k2-fsa/sherpa-onnx/releases/download/kws-models"
VAD_BASE_URL="https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models"

# Temporary directory for downloads
TMP_DIR="/tmp/xiaozhi-models-$$"
MODELS_DIR="./models"

echo "=== 小智模型下载脚本 ==="

# Cleanup function
cleanup() {
    if [ -d "$TMP_DIR" ]; then
        echo "清理临时目录: $TMP_DIR"
        rm -rf "$TMP_DIR"
    fi
}

# Set trap for cleanup on exit
trap cleanup EXIT

# Create directories
mkdir -p "$TMP_DIR"
mkdir -p "$MODELS_DIR"

echo "创建目录: $TMP_DIR, $MODELS_DIR"

# Function to download file
download_file() {
    local url="$1"
    local output="$2"

    echo "下载: $(basename "$url")"
    if command -v wget >/dev/null 2>&1; then
        wget -q -O "$output" "$url"
    elif command -v curl >/dev/null 2>&1; then
        curl -s -L -o "$output" "$url"
    else
        echo "错误: 需要安装 wget 或 curl"
        exit 1
    fi
}

# Download KWS model archive
echo "下载关键词检测模型 (KWS)..."

KWS_ARCHIVE="$TMP_DIR/sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01.tar.bz2"
KWS_ARCHIVE_URL="$KWS_BASE_URL/sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01.tar.bz2"

download_file "$KWS_ARCHIVE_URL" "$KWS_ARCHIVE"

# Extract KWS archive
echo "解压KWS模型..."
cd "$TMP_DIR"
tar xf "$(basename "$KWS_ARCHIVE")"
cd - >/dev/null

KWS_DIR="$TMP_DIR/sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01"
if [ ! -d "$KWS_DIR" ]; then
    echo "错误: KWS模型解压失败"
    exit 1
fi

# Clean up archive
rm -f "$KWS_ARCHIVE"

# Create keywords file for xiaozhi with proper format
cat > "$KWS_DIR/xiaozhi_keywords.txt" << 'EOF'
n ǐ h ǎo x iǎo zh ì @你好小智
x iǎo zh ì t óng x ué @小智同学
EOF

# Download VAD model
echo "下载语音活动检测模型 (VAD)..."

VAD_FILE="$TMP_DIR/silero_vad.onnx"
download_file "$VAD_BASE_URL/silero_vad.onnx" "$VAD_FILE"

# Download backup VAD model
VAD_FILE2="$TMP_DIR/ten-vad.onnx"
download_file "$VAD_BASE_URL/ten-vad.onnx" "$VAD_FILE2"

# Sync models to target directory
echo "同步模型到目标目录..."

# Remove old models
rm -rf "$MODELS_DIR/sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01"
rm -f "$MODELS_DIR/silero_vad.onnx"
rm -f "$MODELS_DIR/ten-vad.onnx"

# Copy new models
cp -r "$KWS_DIR" "$MODELS_DIR/"
cp "$VAD_FILE" "$MODELS_DIR/"
cp "$VAD_FILE2" "$MODELS_DIR/"

# Verify installation
echo "验证模型安装..."

KWS_TARGET_DIR="$MODELS_DIR/sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01"
required_files=("encoder-epoch-12-avg-2-chunk-16-left-64.onnx" "decoder-epoch-12-avg-2-chunk-16-left-64.onnx" "joiner-epoch-12-avg-2-chunk-16-left-64.onnx" "tokens.txt" "xiaozhi_keywords.txt" "silero_vad.onnx" "ten-vad.onnx")

all_files_exist=true
for file in "${required_files[@]}"; do
    if [[ "$file" == *"vad"* ]]; then
        filepath="$MODELS_DIR/$file"
    else
        filepath="$KWS_TARGET_DIR/$file"
    fi

    if [ -f "$filepath" ]; then
        echo "✓ $file"
    else
        echo "✗ 缺失: $file"
        all_files_exist=false
    fi
done

if [ "$all_files_exist" = true ]; then
    echo "模型下载安装成功！"
    echo "运行: ./bin/wake-word"
    echo "或: make wake-word-run"
else
    echo "模型安装失败"
    exit 1
fi