#!/bin/bash
set -e

echo "==== Checking Go bindings for whisper.cpp ===="

# Check if whisper.cpp directory exists
if [ ! -d "whisper.cpp" ]; then
    echo "ERROR: whisper.cpp directory not found"
    echo "Run ./build-go-bindings.sh first"
    exit 1
fi

# Create models directory if it doesn't exist
if [ ! -d "whisper.cpp/models" ]; then
    mkdir -p whisper.cpp/models
fi

# Check if Go bindings are built
if [ ! -d "whisper.cpp/bindings/go/pkg/whisper" ]; then
    echo "ERROR: Go bindings not found"
    echo "Run ./build-go-bindings.sh to build the Go bindings"
    exit 1
fi

# Check for essential binding files
ESSENTIAL_FILES=(
  "whisper.cpp/bindings/go/pkg/whisper/model.go"
  "whisper.cpp/bindings/go/pkg/whisper/context.go"
  "whisper.cpp/bindings/go/pkg/whisper/interface.go"
)

for file in "${ESSENTIAL_FILES[@]}"; do
  if [ ! -f "$file" ]; then
    echo "ERROR: Essential binding file not found: $file"
    echo "Run ./build-go-bindings.sh to rebuild the Go bindings"
    exit 1
  fi
done

# Check for the C header files
ESSENTIAL_HEADERS=(
  "whisper.cpp/include/whisper.h"
  "whisper.cpp/include/ggml.h"
  "whisper.cpp/include/ggml-alloc.h"
  "whisper.cpp/include/ggml-backend.h"
  "whisper.cpp/include/ggml-cpu.h"
)

for header in "${ESSENTIAL_HEADERS[@]}"; do
  if [ ! -f "$header" ]; then
    echo "WARNING: Header file not found: $header"
    echo "Run ./build-go-bindings.sh to ensure all headers are properly linked"
  fi
done

# Check for common headers used by streaming
STREAM_HEADERS=(
  "whisper.cpp/include/common.h"
  "whisper.cpp/include/common-sdl.h"
  "whisper.cpp/include/common-whisper.h"
)

for header in "${STREAM_HEADERS[@]}"; do
  if [ ! -f "$header" ]; then
    echo "WARNING: Streaming header file not found: $header"
    echo "Run ./build-go-bindings.sh to ensure all streaming headers are properly linked"
  fi
done

# Check for the stream executable
if [ -f "whisper.cpp/build/bin/whisper-stream" ]; then
  echo "✅ Found whisper-stream executable"
else
  echo "WARNING: whisper-stream executable not found"
  echo "Run ./build-go-bindings.sh with SDL2 support to build the stream executable"
fi

# Check for whisper.h symlink in project root for IDE detection
if [ ! -f "whisper.h" ]; then
  echo "Creating symbolic link for whisper.h in project root for IDE detection"
  ln -sf whisper.cpp/include/whisper.h whisper.h
fi

# Check for libwhisper symlinks
if [ ! -f "whisper.cpp/libwhisper.a" ] && [ ! -f "whisper.cpp/libwhisper.so" ] && [ ! -f "whisper.cpp/libwhisper.dylib" ]; then
  echo "WARNING: Library symlink not found. Run ./build-go-bindings.sh to rebuild the library"
fi

# Check for headers and libraries in ~/.ramble
USER_RAMBLE_DIR="${HOME}/.ramble"
if [ ! -d "${USER_RAMBLE_DIR}/include" ] || [ ! -d "${USER_RAMBLE_DIR}/lib" ]; then
  echo "WARNING: ~/.ramble directory structure not found"
  echo "Run ./build-go-bindings.sh to set up the ~/.ramble directory"
else
  echo "Checking ~/.ramble directory..."

  # Required headers to check
  RAMBLE_HEADERS=(
    "whisper.h"
    "ggml.h"
    "ggml-alloc.h"
    "ggml-backend.h"
    "ggml-cpu.h"
  )

  # Check each required header
  for header in "${RAMBLE_HEADERS[@]}"; do
    if [ -f "${USER_RAMBLE_DIR}/include/$header" ]; then
      echo "✅ Found $header in ~/.ramble/include"
    else
      echo "WARNING: $header not found in ~/.ramble/include"
      echo "Run ./build-go-bindings.sh to install all headers"
    fi
  done

  # Check for stream-related headers
  for header in common*.h; do
    if [ -f "${USER_RAMBLE_DIR}/include/$header" ]; then
      echo "✅ Found $header in ~/.ramble/include"
    fi
  done

  # Check libraries
  LIB_FOUND=0
  if [ -f "${USER_RAMBLE_DIR}/lib/libwhisper.so" ]; then
    echo "✅ Found libwhisper.so in ~/.ramble/lib"
    LIB_FOUND=1
  fi

  if [ -f "${USER_RAMBLE_DIR}/lib/libwhisper.so.1" ]; then
    echo "✅ Found libwhisper.so.1 in ~/.ramble/lib"
    LIB_FOUND=1
  fi

  if [ -f "${USER_RAMBLE_DIR}/lib/libwhisper.a" ]; then
    echo "✅ Found libwhisper.a in ~/.ramble/lib"
    LIB_FOUND=1
  fi

  if [ -f "${USER_RAMBLE_DIR}/lib/libwhisper.dylib" ]; then
    echo "✅ Found libwhisper.dylib in ~/.ramble/lib"
    LIB_FOUND=1
  fi

  if [ $LIB_FOUND -eq 0 ]; then
    echo "WARNING: No libwhisper libraries found in ~/.ramble/lib"
    echo "Run ./build-go-bindings.sh to install libraries"
  fi

  # Check for stream executable in ~/.ramble/bin
  if [ -f "${USER_RAMBLE_DIR}/bin/whisper-stream" ]; then
    echo "✅ Found whisper-stream executable in ~/.ramble/bin"
  else
    echo "NOTE: whisper-stream executable not found in ~/.ramble/bin"
    echo "This is optional but recommended for advanced streaming features"
  fi
fi

# Simple Go test for binding structure - optional
if command -v go &> /dev/null; then
  echo "Testing Go bindings with simple import..."
  # Create a simple temp test file
  TEST_DIR=$(mktemp -d)
  cat > "$TEST_DIR/main.go" << 'EOF'
package main

import (
  "fmt"
  "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

func main() {
  fmt.Println("Import successful")
}
EOF

  # Test compilation
  cd "$TEST_DIR"
  go mod init test
  go mod edit -replace github.com/ggerganov/whisper.cpp/bindings/go=/home/toasty/projects/Ramble/whisper.cpp/bindings/go
  if go build -tags=whisper_go; then
    echo "✅ Go bindings import test successful"
  else
    echo "⚠️ Go bindings import test failed"
  fi
  cd - > /dev/null
  rm -rf "$TEST_DIR"
fi

echo "==== Go bindings verified ===="
echo "Go bindings are available and properly structured."
echo "You can now build the application with: go build -tags=whisper_go -o ramble ./cmd/ramble"