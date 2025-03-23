#!/bin/bash
set -e

# This script tests that the whisper.cpp stream binary correctly accepts audio via stdin
# and outputs transcription results

# Check if whisper stream executable exists
if [ ! -f "pkg/transcription/embed/binaries/linux-amd64/whisper" ]; then
    echo "Stream executable not found. Run build_whisper_stream.sh first."
    exit 1
fi

# Make a copy of the executable for testing
mkdir -p test_output
cp pkg/transcription/embed/binaries/linux-amd64/whisper test_output/stream-test

# Create a test audio file if it doesn't exist (using sox if available)
# This creates a 3-second silent audio file
if command -v sox &> /dev/null; then
    echo "Creating test audio file with sox..."
    sox -n -r 16000 -c 1 -b 16 test_output/test.wav trim 0.0 3.0
    # Convert to raw PCM
    sox test_output/test.wav -t raw -r 16000 -b 16 -e signed-integer -c 1 test_output/test.pcm
elif [ ! -f "test_output/test.pcm" ]; then
    echo "Sox not found and no test.pcm exists. Creating a dummy PCM file..."
    # Create a 3-second dummy PCM file (48000 bytes = 3 seconds @ 16kHz, 16-bit)
    dd if=/dev/zero of=test_output/test.pcm bs=16000 count=3
fi

# Download a small model for testing if not exists
if [ ! -f "test_output/ggml-tiny.bin" ]; then
    echo "Downloading tiny model for testing..."
    curl -L -o test_output/ggml-tiny.bin \
        https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin
fi

# Test if the stream binary accepts stdin input
echo "Testing stream executable with stdin input..."
cat test_output/test.pcm | test_output/stream-test -m test_output/ggml-tiny.bin --stdin

# Check exit code
if [ $? -eq 0 ]; then
    echo "✅ Test passed: Stream executable correctly accepts stdin input"
else
    echo "❌ Test failed: Stream executable failed to process stdin input"
    exit 1
fi

echo "Cleaning up..."
rm -rf test_output

echo "All tests completed successfully!"