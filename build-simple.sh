#!/bin/bash
set -e

# This script builds the Ramble application using the whisper.cpp submodule
echo "=== Building Ramble with whisper.cpp submodule ==="

WHISPER_DIR="third_party/whisper.cpp"

echo "1. Building whisper.cpp library..."
cd ${WHISPER_DIR}
mkdir -p build
cd build
cmake -DCMAKE_BUILD_TYPE=Release -DWHISPER_BUILD_SHARED=ON ..
cmake --build . --parallel
cd ../../..

echo "2. Creating symlinks..."
# Create symlink for libwhisper.so
if [ -f "${WHISPER_DIR}/build/src/libwhisper.so" ]; then
    ln -sf "${WHISPER_DIR}/build/src/libwhisper.so" libwhisper.so
    echo "✅ Created symlink for libwhisper.so"
fi

# Create symlink for ggml libraries
for lib in ${WHISPER_DIR}/build/ggml/src/libggml*.so; do
    if [ -f "$lib" ]; then
        libname=$(basename "$lib")
        ln -sf "$lib" "$libname"
        echo "✅ Created symlink for $libname"
    fi
done

# Create symlink for whisper.h
if [ -f "${WHISPER_DIR}/include/whisper.h" ]; then
    ln -sf "${WHISPER_DIR}/include/whisper.h" whisper.h
    echo "✅ Created symlink for whisper.h"
fi

echo "3. Building Go application..."
# Set environment variables for building
export CGO_ENABLED=1
export CGO_CFLAGS="-I$(pwd)/${WHISPER_DIR}/include -I$(pwd)/${WHISPER_DIR}/ggml/include"
export CGO_LDFLAGS="-L$(pwd)/${WHISPER_DIR}/build/src -L$(pwd)/${WHISPER_DIR}/build/ggml/src -lwhisper"
export LD_LIBRARY_PATH="$(pwd):$(pwd)/${WHISPER_DIR}/build/src:$(pwd)/${WHISPER_DIR}/build/ggml/src:$LD_LIBRARY_PATH"

# Build the Go application
go build -tags=whisper_go -o ramble ./cmd/ramble

echo "=== Build complete! ==="
echo "Run the application with: ./run-simple.sh"