#!/bin/bash
set -e

echo "==== Building whisper.cpp with Go bindings ===="

# Create directories if they don't exist
mkdir -p whisper.cpp/models

# Clone whisper.cpp repository if not already cloned
if [ ! -d "whisper.cpp/.git" ]; then
    echo "Cloning whisper.cpp repository..."
    rm -rf whisper.cpp
    git clone --depth 1 https://github.com/ggerganov/whisper.cpp.git
else
    echo "whisper.cpp repository already exists, updating..."
    cd whisper.cpp
    git pull
    cd ..
fi

# Build the main library using CMake
echo "Building whisper.cpp library..."
cd whisper.cpp
cmake -B build
cmake --build build --config Release

# Build the Go bindings
echo "Building Go bindings..."
cd bindings/go
make
cd ../../..

# Download models if needed
if [ ! -f "whisper.cpp/models/ggml-tiny.bin" ]; then
    echo "Downloading tiny model..."
    cd whisper.cpp
    bash ./models/download-ggml-model.sh tiny
    cd ..
fi

if [ ! -f "whisper.cpp/models/ggml-small.bin" ]; then
    echo "Downloading small model..."
    cd whisper.cpp
    bash ./models/download-ggml-model.sh small
    cd ..
fi

# Make the script executable
chmod +x check-go-bindings.sh

# Now verify the Go bindings are working
echo "Verifying Go bindings..."
./check-go-bindings.sh

echo "==== Build complete! ===="
echo "You can now build the application with: go build -tags=whisper_go -o ramble ./cmd/ramble"
echo "Or run tests with: go test -tags=whisper_go ./..."