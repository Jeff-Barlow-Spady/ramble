#!/bin/bash
set -e

# Set library paths
export LD_LIBRARY_PATH="$(pwd)/whisper.cpp:$(pwd)/whisper.cpp/build:$(pwd)/whisper.cpp/build/src:$(pwd):$HOME/.ramble/lib:$LD_LIBRARY_PATH"

# Check if ramble exists
if [ ! -f "ramble" ]; then
    echo "Error: ramble executable not found. Please build it first."
    echo "Run: export CGO_LDFLAGS=\"-L\$(pwd)/whisper.cpp -L\$(pwd)/whisper.cpp/build -L\$(pwd)/whisper.cpp/build/src -L\$(pwd) -L\$HOME/.ramble/lib\" CGO_CFLAGS=\"-I\$(pwd)/whisper.cpp/include -I\$HOME/.ramble/include\" go build -tags=whisper_go -o ramble ./cmd/ramble"
    exit 1
fi

# Run ramble with any passed arguments
echo "Starting Ramble with proper library paths..."
./ramble "$@"