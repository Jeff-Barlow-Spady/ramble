#!/bin/bash
set -e

echo "=== Ramble Audio Transcription Application ==="

# Check for flags
SKIP_BUILD=0
EMBED_MODELS=0

for arg in "$@"; do
  if [ "$arg" == "--no-build" ]; then
    SKIP_BUILD=1
    # Remove this argument from the ones passed to the application
    set -- "${@/$arg/}"
  elif [ "$arg" == "--embed-models" ]; then
    EMBED_MODELS=1
    # Remove this argument from the ones passed to the application
    set -- "${@/$arg/}"
  fi
done

# Set environment variables for running the application with Go bindings
# Declare and assign separately to avoid masking return values
LIBRARY_PATH=$(pwd)/whisper.cpp:$(pwd)/whisper.cpp/build/src:${HOME}/.ramble/lib
export LIBRARY_PATH

LD_LIBRARY_PATH=$(pwd):$(pwd)/whisper.cpp:$(pwd)/whisper.cpp/build:$(pwd)/whisper.cpp/build/src:${HOME}/.ramble/lib:$LD_LIBRARY_PATH
export LD_LIBRARY_PATH

C_INCLUDE_PATH=$(pwd)/whisper.cpp/include:${HOME}/.ramble/include
export C_INCLUDE_PATH

CGO_CFLAGS="-I$(pwd)/whisper.cpp/include -I${HOME}/.ramble/include"
export CGO_CFLAGS

CGO_LDFLAGS="-L$(pwd)/whisper.cpp -L$(pwd)/whisper.cpp/build -L$(pwd)/whisper.cpp/build/src -L${HOME}/.ramble/lib"
export CGO_LDFLAGS

if [ "$SKIP_BUILD" -eq 0 ]; then
  # Check for and build the Go bindings
  if [ ! -f "whisper.cpp/build/src/libwhisper.so" ] && [ ! -f "whisper.cpp/build/src/libwhisper.a" ] && [ ! -f "whisper.cpp/libwhisper.so" ] && [ ! -f "whisper.cpp/libwhisper.a" ]; then
    echo "Go bindings libraries not found! Run ./build-go-bindings.sh first."
    echo "Building Go bindings is required - running build-go-bindings.sh now..."
    if ! ./build-go-bindings.sh; then
      echo "Failed to build Go bindings. Please fix the issues and try again."
      exit 1
    fi
  fi

  # Verify required libraries for Go bindings
  if [ -f "whisper.cpp/build/src/libwhisper.so" ] || [ -f "whisper.cpp/build/src/libwhisper.a" ] || [ -f "whisper.cpp/libwhisper.so" ] || [ -f "whisper.cpp/libwhisper.a" ]; then
    echo "✅ Found whisper.cpp libraries"
  else
    echo "❌ Missing whisper.cpp libraries - Go bindings are required"
    echo "Please run ./build-go-bindings.sh to set up the required libraries"
    exit 1
  fi

  if [ -f "whisper.cpp/include/whisper.h" ]; then
    echo "✅ Found whisper.h header file"
  else
    echo "❌ Missing whisper.h header file - Go bindings are required"
    echo "Please run ./build-go-bindings.sh to set up the required header files"
    exit 1
  fi

  # If embedding models is requested, download them first
  if [ "$EMBED_MODELS" -eq 1 ]; then
    echo "Setting up models for embedding..."
    if ! ./scripts/setup-test-models.sh --embed; then
      echo "❌ Failed to setup models for embedding."
      exit 1
    fi
    echo "✅ Models prepared for embedding"
  fi

  # Build the application
  echo "Building Ramble..."
  if ! go build -tags=whisper_go -o ramble ./cmd/ramble; then
    echo "❌ Build failed! Please ensure Go bindings are properly set up."
    exit 1
  fi
else
  echo "Skipping build step..."

  # Check if the executable exists
  if [ ! -f "./ramble" ]; then
    echo "Error: 'ramble' executable not found."
    echo "Please run without the --no-build flag first to build the application."
    exit 1
  fi
fi

# Create symlink to libwhisper.so.1 in current directory for runtime loading
if [ -f "whisper.cpp/build/src/libwhisper.so.1" ] && [ ! -f "libwhisper.so.1" ]; then
  echo "Creating symlink to libwhisper.so.1 in current directory..."
  ln -sf "$(pwd)/whisper.cpp/build/src/libwhisper.so.1" "$(pwd)/libwhisper.so.1"
fi

echo "Starting application..."

# Run the application
./ramble "$@"