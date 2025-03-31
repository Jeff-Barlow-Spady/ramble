#!/bin/bash
set -e

# Super simple script to download models for testing/embedding
# Usage:
#   ./scripts/setup-test-models.sh         # Download model for testing
#   ./scripts/setup-test-models.sh --embed # Also copy to embed dir

# Setup test directory
TEST_DIR="test-models"
mkdir -p "$TEST_DIR"

# Only download if the model doesn't exist
if [ ! -f "$TEST_DIR/ggml-tiny.en.bin" ]; then
    echo "Downloading tiny model..."
    curl -L https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.en.bin -o "$TEST_DIR/ggml-tiny.en.bin"
    echo "Downloaded model to $TEST_DIR/ggml-tiny.en.bin"
else
    echo "Model already exists at $TEST_DIR/ggml-tiny.en.bin"
fi

# Copy to embed directory if requested
if [ "$1" == "--embed" ]; then
    echo "Copying model to embed directory..."
    EMBED_DIR="pkg/transcription/embed/models"
    mkdir -p "$EMBED_DIR"
    cp "$TEST_DIR/ggml-tiny.en.bin" "$EMBED_DIR/tiny.bin"

    # Optionally, download small model too
    if [ ! -f "$TEST_DIR/ggml-small.en.bin" ] && [ "$2" == "--with-small" ]; then
        echo "Downloading small model..."
        curl -L https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.en.bin -o "$TEST_DIR/ggml-small.en.bin"
        cp "$TEST_DIR/ggml-small.en.bin" "$EMBED_DIR/small.bin"
    fi

    echo "Models copied to embed directory"
fi

# Also copy to standard application model path
APP_MODEL_DIR="$HOME/.local/share/ramble/models"
if [ ! -f "$APP_MODEL_DIR/ggml-tiny.en.bin" ]; then
    echo "Copying model to application directory..."
    mkdir -p "$APP_MODEL_DIR"
    cp "$TEST_DIR/ggml-tiny.en.bin" "$APP_MODEL_DIR/ggml-tiny.en.bin"
    echo "Model copied to $APP_MODEL_DIR"
fi

echo "Done!"