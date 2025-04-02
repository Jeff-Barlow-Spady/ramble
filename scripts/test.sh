#!/bin/bash
set -e

# Parse command line options
FAST_MODE=0
SHORT_TESTS=0
SKIP_SOME_TESTS=0
TEST_ARGS=()

for arg in "$@"; do
  case $arg in
    --fast)
      FAST_MODE=1
      SHORT_TESTS=1
      ;;
    --short)
      SHORT_TESTS=1
      ;;
    --skip-hardware)
      SKIP_SOME_TESTS=1
      ;;
    *)
      TEST_ARGS+=("$arg")
      ;;
  esac
done

echo "=== Running Ramble Tests ==="

# Path to whisper.cpp - check both possible locations
if [ -d "third_party/whisper.cpp" ]; then
  WHISPER_CPP_DIR=third_party/whisper.cpp
elif [ -d "whisper.cpp" ]; then
  WHISPER_CPP_DIR=whisper.cpp
else
  echo "Error: whisper.cpp directory not found"
  exit 1
fi

# Set environment variables for running the tests with Go bindings
# Library paths
LIBRARY_PATH=$(pwd)/${WHISPER_CPP_DIR}:$(pwd)/${WHISPER_CPP_DIR}/build/src:$(pwd)/${WHISPER_CPP_DIR}/build/ggml/src:${HOME}/.ramble/lib
export LIBRARY_PATH

# Runtime library path
LD_LIBRARY_PATH=$(pwd):$(pwd)/${WHISPER_CPP_DIR}:$(pwd)/${WHISPER_CPP_DIR}/build:$(pwd)/${WHISPER_CPP_DIR}/build/src:$(pwd)/${WHISPER_CPP_DIR}/build/ggml/src:${HOME}/.ramble/lib:$LD_LIBRARY_PATH
export LD_LIBRARY_PATH

# C include path
C_INCLUDE_PATH=$(pwd)/${WHISPER_CPP_DIR}/include:${HOME}/.ramble/include
export C_INCLUDE_PATH

# CGO compiler flags
CGO_CFLAGS="-I$(pwd)/${WHISPER_CPP_DIR}/include -I$(pwd)/${WHISPER_CPP_DIR}/ggml/include"
export CGO_CFLAGS

# CGO linker flags
CGO_LDFLAGS="-L$(pwd)/${WHISPER_CPP_DIR} -L$(pwd)/${WHISPER_CPP_DIR}/build -L$(pwd)/${WHISPER_CPP_DIR}/build/src -L$(pwd)/${WHISPER_CPP_DIR}/build/ggml/src -L${HOME}/.ramble/lib"
export CGO_LDFLAGS

# Create symlink to libwhisper.so.1 in current directory for runtime loading
if [ -f "${WHISPER_CPP_DIR}/build/src/libwhisper.so.1" ] && [ ! -f "libwhisper.so.1" ]; then
  echo "Creating symlink to libwhisper.so.1 in current directory..."
  ln -sf "$(pwd)/${WHISPER_CPP_DIR}/build/src/libwhisper.so.1" "$(pwd)/libwhisper.so.1"
fi

# If in fast mode, run only specific packages
if [ "$FAST_MODE" -eq 1 ]; then
  echo "Running in FAST mode - only testing essential packages..."
  # Test only the most important packages that don't require hardware
  go test -v -tags=whisper_go ./pkg/config ./pkg/transcription/embed ./tests/unit
  exit $?
fi

# Determine what flags to pass to go test
GO_TEST_FLAGS="-v -tags=whisper_go"

if [ "$SHORT_TESTS" -eq 1 ]; then
  echo "Running tests in short mode..."
  GO_TEST_FLAGS="$GO_TEST_FLAGS -short"
fi

# Run specific package tests if provided, otherwise run all tests
if [ "${#TEST_ARGS[@]}" -eq 0 ]; then
  if [ "$SKIP_SOME_TESTS" -eq 1 ]; then
    echo "Skipping hardware-dependent tests..."
    # List packages that don't require hardware
    go test $GO_TEST_FLAGS ./pkg/config ./pkg/transcription/embed ./tests/unit ./tests/integration
  else
    echo "Running all tests..."
    go test $GO_TEST_FLAGS ./...
  fi
else
  echo "Running specified tests..."
  go test $GO_TEST_FLAGS "${TEST_ARGS[@]}"
fi