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

# Path to whisper.cpp - use third_party submodule
WHISPER_CPP_DIR=third_party/whisper.cpp

# Set environment variables for running the application with Go bindings
# Declare and assign separately to avoid masking return values
LIBRARY_PATH=$(pwd)/${WHISPER_CPP_DIR}:$(pwd)/${WHISPER_CPP_DIR}/build/src:$(pwd)/${WHISPER_CPP_DIR}/build/ggml/src:${HOME}/.ramble/lib
export LIBRARY_PATH

LD_LIBRARY_PATH=$(pwd):$(pwd)/${WHISPER_CPP_DIR}:$(pwd)/${WHISPER_CPP_DIR}/build:$(pwd)/${WHISPER_CPP_DIR}/build/src:$(pwd)/${WHISPER_CPP_DIR}/build/ggml/src:${HOME}/.ramble/lib:$LD_LIBRARY_PATH
export LD_LIBRARY_PATH

C_INCLUDE_PATH=$(pwd)/${WHISPER_CPP_DIR}/include:${HOME}/.ramble/include
export C_INCLUDE_PATH

CGO_CFLAGS="-I$(pwd)/${WHISPER_CPP_DIR}"
export CGO_CFLAGS

CGO_LDFLAGS="-L$(pwd)/${WHISPER_CPP_DIR} -L$(pwd)/${WHISPER_CPP_DIR}/build -L$(pwd)/${WHISPER_CPP_DIR}/build/src -L$(pwd)/${WHISPER_CPP_DIR}/build/ggml/src -L${HOME}/.ramble/lib"
export CGO_LDFLAGS

# Check if whisper libraries exist
if [ ! -f "${WHISPER_CPP_DIR}/build/src/libwhisper.so" ] && [ ! -f "${WHISPER_CPP_DIR}/build/src/libwhisper.a" ] && [ ! -f "${WHISPER_CPP_DIR}/libwhisper.so" ] && [ ! -f "${WHISPER_CPP_DIR}/libwhisper.a" ]; then
    # Check for both locations to support migration period
    if [ -f "whisper.cpp/build/src/libwhisper.so" ] || [ -f "whisper.cpp/build/src/libwhisper.a" ]; then
        echo "Found whisper libraries in standalone directory. Consider migrating to third_party with scripts/migrate-whisper.sh"
        WHISPER_CPP_DIR=whisper.cpp

        # Update paths for backwards compatibility
        LIBRARY_PATH=$(pwd)/${WHISPER_CPP_DIR}:$(pwd)/${WHISPER_CPP_DIR}/build/src:$(pwd)/${WHISPER_CPP_DIR}/build/ggml/src:${HOME}/.ramble/lib
        LD_LIBRARY_PATH=$(pwd):$(pwd)/${WHISPER_CPP_DIR}:$(pwd)/${WHISPER_CPP_DIR}/build:$(pwd)/${WHISPER_CPP_DIR}/build/src:$(pwd)/${WHISPER_CPP_DIR}/build/ggml/src:${HOME}/.ramble/lib:$LD_LIBRARY_PATH
        export LD_LIBRARY_PATH
        CGO_CFLAGS="-I$(pwd)/${WHISPER_CPP_DIR}"
        export CGO_CFLAGS
        CGO_LDFLAGS="-L$(pwd)/${WHISPER_CPP_DIR} -L$(pwd)/${WHISPER_CPP_DIR}/build -L$(pwd)/${WHISPER_CPP_DIR}/build/src -L$(pwd)/${WHISPER_CPP_DIR}/build/ggml/src -L${HOME}/.ramble/lib"
        export CGO_LDFLAGS
    else
        echo "Whisper libraries not found, running build script..."
        ./scripts/build-go-bindings.sh
    fi
fi

if [ "$SKIP_BUILD" -eq 0 ]; then
  # Check for and build the Go bindings
  if [ ! -f "${WHISPER_CPP_DIR}/build/src/libwhisper.so" ] && [ ! -f "${WHISPER_CPP_DIR}/build/src/libwhisper.a" ] && [ ! -f "${WHISPER_CPP_DIR}/libwhisper.so" ] && [ ! -f "${WHISPER_CPP_DIR}/libwhisper.a" ]; then
    echo "Go bindings libraries not found! Run ./build-go-bindings.sh first."
    echo "Building Go bindings is required - running build-go-bindings.sh now..."
    if ! ./build-go-bindings.sh; then
      echo "Failed to build Go bindings. Please fix the issues and try again."
      exit 1
    fi
  fi

  # Verify required libraries for Go bindings
  if [ -f "${WHISPER_CPP_DIR}/build/src/libwhisper.so" ] || [ -f "${WHISPER_CPP_DIR}/build/src/libwhisper.a" ] || [ -f "${WHISPER_CPP_DIR}/libwhisper.so" ] || [ -f "${WHISPER_CPP_DIR}/libwhisper.a" ]; then
    echo "✅ Found whisper.cpp libraries"
  else
    echo "❌ Missing whisper.cpp libraries - Go bindings are required"
    echo "Please run ./build-go-bindings.sh to set up the required libraries"
    exit 1
  fi

  if [ -f "${WHISPER_CPP_DIR}/include/whisper.h" ]; then
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
if [ -f "${WHISPER_CPP_DIR}/build/src/libwhisper.so.1" ] && [ ! -f "libwhisper.so.1" ]; then
  echo "Creating symlink to libwhisper.so.1 in current directory..."
  ln -sf "$(pwd)/${WHISPER_CPP_DIR}/build/src/libwhisper.so.1" "$(pwd)/libwhisper.so.1"
fi

echo "Starting application..."

# Run the application
./ramble "$@"