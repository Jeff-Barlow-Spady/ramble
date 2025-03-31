#!/bin/bash
set -e

echo "Creating symlink to libwhisper.so.1 if needed..."
if [ -f "whisper.cpp/build/src/libwhisper.so.1" ] && [ ! -f "libwhisper.so.1" ]; then
  ln -sf "$PWD/whisper.cpp/build/src/libwhisper.so.1" "$PWD/libwhisper.so.1"
fi

echo "Building with proper library paths..."
export CGO_LDFLAGS="-L$PWD/whisper.cpp/build/src -L$PWD -L$HOME/.ramble/lib"
export CGO_CFLAGS="-I$PWD/whisper.cpp/include -I$HOME/.ramble/include"
export LD_LIBRARY_PATH="$PWD:$PWD/whisper.cpp/build/src:$HOME/.ramble/lib:$LD_LIBRARY_PATH"

go build -tags=whisper_go -ldflags="-extldflags '-Wl,-rpath,$PWD/whisper.cpp/build/src:$HOME/.ramble/lib'" -o ramble ./cmd/ramble

echo "Build complete. Running application with debug mode..."
./ramble -t -debug