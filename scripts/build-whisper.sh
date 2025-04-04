#!/bin/bash
set -e

echo "=== Building Whisper.cpp Library ==="

# Configuration
WHISPER_VERSION=${1:-"v1.5.2"}  # Default version if not specified
TEMP_DIR=$(mktemp -d)
VENDOR_DIR="./vendor/whisper"

echo "Using whisper.cpp version: $WHISPER_VERSION"

# Create vendor directories
mkdir -p $VENDOR_DIR/include
mkdir -p $VENDOR_DIR/lib

# Clone whisper.cpp at the specified version
echo "Cloning whisper.cpp repository..."
git clone --depth 1 --branch $WHISPER_VERSION https://github.com/ggerganov/whisper.cpp.git $TEMP_DIR/whisper-build

# Build the library
echo "Building whisper.cpp library..."
cd $TEMP_DIR/whisper-build
mkdir -p build && cd build

# Configure cmake with minimal options
cmake .. \
  -DCMAKE_BUILD_TYPE=Release \
  -DBUILD_SHARED_LIBS=OFF \
  -DWHISPER_BUILD_EXAMPLES=OFF \
  -DWHISPER_BUILD_TESTS=OFF \
  -DWHISPER_BUILD_SERVER=OFF

# Build the library
cmake --build . --config Release --target whisper

# Copy the necessary files to vendor directory
echo "Copying library files to vendor directory..."
cp -f ../whisper.h $VENDOR_DIR/include/

# Handle different library names based on platform
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
  cp -f src/libwhisper.a $VENDOR_DIR/lib/
  # Also copy shared library if it exists
  if [[ -f "src/libwhisper.so" ]]; then
    cp -f src/libwhisper.so $VENDOR_DIR/lib/
  fi
  if [[ -f "src/libwhisper.so.1" ]]; then
    cp -f src/libwhisper.so.1 $VENDOR_DIR/lib/
  fi
elif [[ "$OSTYPE" == "darwin"* ]]; then
  cp -f src/libwhisper.a $VENDOR_DIR/lib/
  # Also copy shared library if it exists
  if [[ -f "src/libwhisper.dylib" ]]; then
    cp -f src/libwhisper.dylib $VENDOR_DIR/lib/
  fi
elif [[ "$OSTYPE" == "msys"* || "$OSTYPE" == "win32" ]]; then
  cp -f src/Release/whisper.lib $VENDOR_DIR/lib/ 2>/dev/null || cp -f src/whisper.lib $VENDOR_DIR/lib/ 2>/dev/null || echo "Warning: Could not find Windows library"
else
  echo "Unknown platform: $OSTYPE"
  exit 1
fi

# Clean up
echo "Cleaning up temporary files..."
rm -rf $TEMP_DIR

echo "✅ Whisper.cpp library built successfully and installed to $VENDOR_DIR"
echo "You can now build Ramble using: ./scripts/run.sh"