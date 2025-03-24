#!/bin/bash
set -e

echo "===== Testing Go Bindings for Streaming Transcription ====="

# Check if the ramble executable exists
if [ ! -f "ramble" ]; then
    echo "ERROR: 'ramble' executable not found. Build it first with:"
    echo "  export CGO_LDFLAGS=\"-L\$(pwd)/whisper.cpp -L\$(pwd)/whisper.cpp/build -L\$(pwd)/whisper.cpp/build/src -L\$(pwd) -L\$HOME/.ramble/lib\""
    echo "  export CGO_CFLAGS=\"-I\$(pwd)/whisper.cpp/include -I\$HOME/.ramble/include\""
    echo "  go build -tags=whisper_go -o ramble ./cmd/ramble"
    exit 1
fi

# Verify the executable is linked against libwhisper
if ! ldd ramble | grep -q libwhisper; then
    echo "ERROR: 'ramble' is not linked against libwhisper.so"
    echo "The Go bindings may not be properly built."
    exit 1
fi

echo "Executable correctly linked with libwhisper.so"

# Check for test models
if [ -d "whisper.cpp/models" ]; then
    echo "Found whisper.cpp models directory"
    TINY_MODEL=$(find whisper.cpp/models -name "ggml-tiny*.bin" | head -1)
    if [ -n "$TINY_MODEL" ]; then
        echo "Found model: $TINY_MODEL"
    else
        echo "No tiny model found, but will continue anyway"
    fi
else
    echo "NOTE: whisper.cpp/models directory not found"
    echo "The application may need to download models"
fi

# Look for test audio in the whisper.cpp/samples directory
TEST_AUDIO=""
if [ -d "whisper.cpp/samples" ]; then
    echo "Looking for test audio in whisper.cpp/samples..."
    TEST_AUDIO=$(find whisper.cpp/samples -name "*.wav" | head -1)
    if [ -n "$TEST_AUDIO" ]; then
        echo "Found test audio: $TEST_AUDIO"
    fi
fi

# If no test audio found, create a simple binary file
if [ -z "$TEST_AUDIO" ]; then
    echo "Creating a simple test audio file..."
    TEST_AUDIO="test_audio.raw"
    # Create a 3-second silent PCM file (3 * 16000 * 2 bytes = 96000 bytes)
    dd if=/dev/zero of="$TEST_AUDIO" bs=96000 count=1
    echo "Created silent test audio: $TEST_AUDIO"
fi

echo "Starting the application in text mode with debug enabled..."
echo "Press Ctrl+C to exit after seeing some output."
echo "===== Begin application output ====="

# Run the application in terminal mode with debug enabled
./ramble -t -debug

echo "===== Test complete ====="