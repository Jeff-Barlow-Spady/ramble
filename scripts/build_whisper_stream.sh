#!/bin/bash
set -e

# Build whisper.cpp stream executable for embedding in Ramble application
# This script builds the stream example from whisper.cpp which supports stdin streaming

# Create a temporary working directory
TEMP_DIR=$(mktemp -d)
echo "Working in temporary directory: $TEMP_DIR"
cd "$TEMP_DIR"

# Clone whisper.cpp repository
echo "Cloning whisper.cpp repository..."
git clone --depth 1 https://github.com/ggerganov/whisper.cpp.git
cd whisper.cpp

# Build for Linux x64
echo "Building whisper stream for Linux (x64)..."
make
make stream
mkdir -p ../../pkg/transcription/embed/binaries/linux-amd64
cp stream ../../pkg/transcription/embed/binaries/linux-amd64/whisper
echo "Linux stream binary built and copied to embed directory"

# Build for Windows (requires mingw)
if command -v x86_64-w64-mingw32-gcc &> /dev/null; then
    echo "Building whisper stream for Windows (x64)..."
    rm -f stream stream.exe ./*.o  # Clean up manually with safer pattern
    CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ make stream
    mkdir -p ../../pkg/transcription/embed/binaries/windows-amd64
    cp stream.exe ../../pkg/transcription/embed/binaries/windows-amd64/whisper.exe
    echo "Windows stream binary built and copied to embed directory"
else
    echo "MinGW not found - skipping Windows build. Install with apt-get install gcc-mingw-w64"
fi

# For macOS, we could cross-compile but it's better to build natively on a macOS runner
# This is a placeholder that will be skipped when running on Linux
if [[ "$(uname)" == "Darwin" ]]; then
    echo "Building whisper stream for macOS (x64)..."
    rm -f stream stream.exe ./*.o  # Clean up manually with safer pattern
    make stream
    mkdir -p ../../pkg/transcription/embed/binaries/darwin-amd64
    cp stream ../../pkg/transcription/embed/binaries/darwin-amd64/whisper
    echo "macOS stream binary built and copied to embed directory"
else
    echo "Not running on macOS - skipping macOS native build."
    mkdir -p ../../pkg/transcription/embed/binaries/darwin-amd64
    touch ../../pkg/transcription/embed/binaries/darwin-amd64/placeholder
fi

# Create placeholder model files (these will be replaced by the CI process)
mkdir -p ../../pkg/transcription/embed/models
touch ../../pkg/transcription/embed/models/tiny.bin
touch ../../pkg/transcription/embed/models/small.bin

echo "Cleaning up..."
cd ../..
rm -rf "$TEMP_DIR"

echo "Stream binaries built successfully!"