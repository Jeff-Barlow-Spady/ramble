#!/bin/bash

# Script to migrate from standalone whisper.cpp directory to using only the third_party/whisper.cpp submodule
set -e

# Check if the standalone directory exists
if [ ! -d "whisper.cpp" ]; then
    echo "Standalone whisper.cpp directory not found. Nothing to migrate."
    exit 0
fi

# Check if the third_party submodule exists
if [ ! -d "third_party/whisper.cpp" ]; then
    echo "Error: third_party/whisper.cpp does not exist. Make sure the submodule is properly initialized."
    echo "Run: git submodule update --init --recursive"
    exit 1
fi

# Create backup of the build artifacts from standalone directory if they exist
echo "Creating backup of build artifacts from standalone directory..."
if [ -d "whisper.cpp/build" ]; then
    echo "Copying whisper.cpp/build directory to third_party/whisper.cpp/"
    # Create the build directory in third_party if it doesn't exist
    mkdir -p "third_party/whisper.cpp/build"
    # Copy the build artifacts
    cp -R "whisper.cpp/build/"* "third_party/whisper.cpp/build/"
else
    echo "No build directory found in standalone whisper.cpp"
fi

# Update symbolic links if needed
if [ -f "libwhisper.so.1" ] && [ -L "libwhisper.so.1" ]; then
    echo "Updating symbolic link for libwhisper.so.1..."
    rm -f "libwhisper.so.1"
    if [ -f "third_party/whisper.cpp/build/src/libwhisper.so.1" ]; then
        ln -sf "$(pwd)/third_party/whisper.cpp/build/src/libwhisper.so.1" "$(pwd)/libwhisper.so.1"
        echo "Symbolic link updated."
    else
        echo "Warning: libwhisper.so.1 not found in third_party/whisper.cpp/build/src/"
    fi
fi

# Create symbolic links for the ggml libraries
echo "Creating symbolic links for ggml libraries..."
if [ -f "third_party/whisper.cpp/build/ggml/src/libggml.so" ]; then
    ln -sf "$(pwd)/third_party/whisper.cpp/build/ggml/src/libggml.so" "$(pwd)/libggml.so"
fi

if [ -f "third_party/whisper.cpp/build/ggml/src/libggml-base.so" ]; then
    ln -sf "$(pwd)/third_party/whisper.cpp/build/ggml/src/libggml-base.so" "$(pwd)/libggml-base.so"
fi

if [ -f "third_party/whisper.cpp/build/ggml/src/libggml-cpu.so" ]; then
    ln -sf "$(pwd)/third_party/whisper.cpp/build/ggml/src/libggml-cpu.so" "$(pwd)/libggml-cpu.so"
fi

# Back up the original whisper.cpp directory (just in case)
echo "Creating backup of standalone whisper.cpp directory..."
mv whisper.cpp whisper.cpp.bak

echo "Migration completed. The go.mod file has already been updated to use the submodule."
echo ""
echo "To complete the process:"
echo "1. Make sure all build scripts work with the new path"
echo "2. Once everything is verified, remove the backup with: rm -rf whisper.cpp.bak"
echo ""
echo "Testing the build now..."

# Try building to verify everything works
bash scripts/run.sh

if [ $? -eq 0 ]; then
    echo "Build successful! The migration appears to be working correctly."
else
    echo "Build failed. You may need to restore from backup and investigate further."
    echo "To restore: mv whisper.cpp.bak whisper.cpp"
fi