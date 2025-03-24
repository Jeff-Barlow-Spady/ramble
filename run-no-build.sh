#!/bin/bash
set -e

echo "=== Running Ramble (without rebuilding) ==="

# Set environment variables for running the application with Go bindings
export LIBRARY_PATH=$(pwd)/whisper.cpp:${HOME}/.ramble/lib
export LD_LIBRARY_PATH=$(pwd):$(pwd)/whisper.cpp:$(pwd)/whisper.cpp/build:$(pwd)/whisper.cpp/build/src:${HOME}/.ramble/lib:$LD_LIBRARY_PATH

# Check if the executable exists
if [ ! -f "./ramble" ]; then
  echo "Error: 'ramble' executable not found."
  echo "Please build it first with either:"
  echo "  ./run.sh (full build)"
  echo "  or"
  echo "  go build -tags=whisper_go -o ramble ./cmd/ramble (quick build)"
  exit 1
fi

echo "Starting application..."

# Run the application
./ramble "$@"