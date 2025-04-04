#!/bin/bash
set -e

echo "=== Setting up vendor directory for Ramble ==="

# Create the vendor directory structure
mkdir -p vendor/whisper/include
mkdir -p vendor/whisper/lib

# Copy the whisper headers
if [ -f "whisper.h" ]; then
  echo "Copying whisper.h..."
  cp -v whisper.h vendor/whisper/include/
else
  echo "Warning: whisper.h not found in the root directory."
  # Try to find it elsewhere
  WHISPER_H=$(find . -name "whisper.h" -not -path "./vendor/*" | head -n1)
  if [ -n "$WHISPER_H" ]; then
    echo "Found whisper.h at $WHISPER_H"
    cp -v "$WHISPER_H" vendor/whisper/include/

    # Find the GGML include directory (it's in the same location as whisper.h but under ggml/include)
    WHISPER_DIR=$(dirname "$WHISPER_H")
    GGML_INCLUDE_DIR=$(find $(dirname "$WHISPER_DIR") -name "include" -path "*/ggml/include" | head -n1)

    if [ -n "$GGML_INCLUDE_DIR" ]; then
      echo "Found GGML headers at $GGML_INCLUDE_DIR"
      # Copy all GGML headers
      cp -v "$GGML_INCLUDE_DIR"/*.h vendor/whisper/include/
    else
      echo "Error: Could not find GGML include directory"
      exit 1
    fi
  else
    echo "Error: Could not find whisper.h"
    exit 1
  fi
fi

# Copy the whisper libraries
echo "Copying whisper libraries..."
WHISPER_LIBS=$(find ./dist -path "*/libs/libwhisper.so*" 2>/dev/null | head -n3)
if [ -n "$WHISPER_LIBS" ]; then
  # Remove existing files first to avoid issues with symlinks
  rm -f vendor/whisper/lib/libwhisper.so*
  for lib in $WHISPER_LIBS; do
    cp -v "$lib" vendor/whisper/lib/
  done
else
  echo "Warning: Could not find libwhisper.so files in dist directory."
  echo "Checking alternative locations..."

  if [ -f "libwhisper.so" ] && [ ! -L "libwhisper.so" ]; then
    # Remove existing files first to avoid issues with symlinks
    rm -f vendor/whisper/lib/libwhisper.so*
    cp -v libwhisper.so* vendor/whisper/lib/
  else
    echo "Error: Could not find valid libwhisper.so files"
    exit 1
  fi
fi

# Copy the ggml libraries
echo "Copying ggml libraries..."
GGML_LIBS=$(find ./dist -path "*/libs/libggml*.so" 2>/dev/null | head -n3)
if [ -n "$GGML_LIBS" ]; then
  # Remove existing files first to avoid issues with symlinks
  rm -f vendor/whisper/lib/libggml*.so
  for lib in $GGML_LIBS; do
    cp -v "$lib" vendor/whisper/lib/
  done
else
  echo "Warning: Could not find libggml*.so files in dist directory."
  echo "Checking alternative locations..."

  if [ -f "libggml.so" ] && [ ! -L "libggml.so" ]; then
    # Remove existing files first to avoid issues with symlinks
    rm -f vendor/whisper/lib/libggml*.so
    cp -v libggml*.so vendor/whisper/lib/
  else
    echo "Warning: Could not find valid libggml*.so files"
    # Not exiting here as whisper might work without these
  fi
fi

echo "✅ Vendor directory setup complete! You can now build and run Ramble using:"
echo "./scripts/run.sh"