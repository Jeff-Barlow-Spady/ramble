#!/bin/bash
set -e

echo "=== Ramble Audio Transcription Application ==="

# Set environment variables for running the application with Go bindings
export LD_LIBRARY_PATH=$(pwd)/vendor/whisper/lib:$LD_LIBRARY_PATH
export CGO_CFLAGS="-I$(pwd)/vendor/whisper/include"
export CGO_LDFLAGS="-L$(pwd)/vendor/whisper/lib"

# Build the application if needed
if [ ! -f "./ramble" ] || [ "$1" == "--rebuild" ]; then
  echo "Building Ramble..."
  go build -tags=whisper_go -o ramble ./cmd/ramble
fi

# Run the application
./ramble "$@"