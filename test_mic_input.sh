#!/bin/bash
set -e

echo "===== Testing Microphone Input with Ramble ====="

# Check if the ramble executable exists
if [ ! -f "ramble" ]; then
    echo "ERROR: 'ramble' executable not found. Build it first with:"
    echo "  export CGO_LDFLAGS=\"-L\$(pwd)/whisper.cpp -L\$(pwd)/whisper.cpp/build -L\$(pwd)/whisper.cpp/build/src -L\$(pwd) -L\$HOME/.ramble/lib\""
    echo "  export CGO_CFLAGS=\"-I\$(pwd)/whisper.cpp/include -I\$HOME/.ramble/include\""
    echo "  go build -tags=whisper_go -o ramble ./cmd/ramble"
    exit 1
fi

echo "Starting the application in terminal mode for 20 seconds..."
echo "Please speak into your microphone to test transcription."
echo "The application will automatically exit after 20 seconds."
echo "===== Begin application output ====="

# Run the application with timeout
timeout 20s ./ramble -t -debug

echo "===== Test complete ====="
echo "If you saw transcription of your speech, microphone input is working correctly."
echo "If no transcription appeared, there may be issues with microphone access or configuration."