#!/bin/bash
set -e

echo "=== Running Ramble Application ==="

# Set the whisper.cpp directory
WHISPER_DIR="third_party/whisper.cpp"

# Check if the binary exists
if [ ! -f "ramble" ]; then
    echo "Binary not found. Building first..."
    ./build-simple.sh
fi

# Check if libwhisper.so exists
if [ ! -f "libwhisper.so" ] && [ -f "${WHISPER_DIR}/build/src/libwhisper.so" ]; then
    echo "Creating symlink for libwhisper.so..."
    ln -sf "${WHISPER_DIR}/build/src/libwhisper.so" libwhisper.so
fi

# Check if GGML libraries exist and create symlinks if needed
for lib in ${WHISPER_DIR}/build/ggml/src/libggml*.so; do
    if [ -f "$lib" ]; then
        libname=$(basename "$lib")
        if [ ! -f "$libname" ]; then
            echo "Creating symlink for $libname..."
            ln -sf "$lib" "$libname"
        fi
    fi
done

# Set up the library path
export LD_LIBRARY_PATH="$(pwd):$(pwd)/${WHISPER_DIR}/build/src:$(pwd)/${WHISPER_DIR}/build/ggml/src:$LD_LIBRARY_PATH"

echo "Starting application..."
./ramble "$@"