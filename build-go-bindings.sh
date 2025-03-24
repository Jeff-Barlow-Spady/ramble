#!/bin/bash
set -e

echo "==== Building whisper.cpp with Go bindings ===="

# Clone whisper.cpp repository if it doesn't exist
if [ ! -d "whisper.cpp" ]; then
    echo "whisper.cpp repository not found, cloning..."
    git clone https://github.com/ggerganov/whisper.cpp.git
else
    echo "whisper.cpp repository already exists, updating..."
    cd whisper.cpp && git pull && cd ..
fi

echo "Building whisper.cpp library with stream support..."
cd whisper.cpp
mkdir -p build
cd build
# Enable SDL2 to build the stream example
cmake -DWHISPER_BUILD_GO_BINDINGS=1 -DWHISPER_BUILD_SHARED=1 -DWHISPER_SDL2=ON ..
cmake --build . --parallel
# Also explicitly build 'stream' target to ensure it's available
cmake --build . --target stream --parallel
cd ../..

echo "Creating symbolic links for libraries..."
# Find the library that was built
LIBRARY_PATH=""
if [ -f "whisper.cpp/build/src/libwhisper.so" ]; then
    LIBRARY_PATH="whisper.cpp/build/src/libwhisper.so"
    ln -sf "$LIBRARY_PATH" ./libwhisper.so
    ln -sf "$LIBRARY_PATH" whisper.cpp/libwhisper.so
elif [ -f "whisper.cpp/build/src/libwhisper.a" ]; then
    LIBRARY_PATH="whisper.cpp/build/src/libwhisper.a"
    ln -sf "$LIBRARY_PATH" ./libwhisper.a
    ln -sf "$LIBRARY_PATH" whisper.cpp/libwhisper.a
elif [ -f "whisper.cpp/build/src/libwhisper.dylib" ]; then
    LIBRARY_PATH="whisper.cpp/build/src/libwhisper.dylib"
    ln -sf "$LIBRARY_PATH" ./libwhisper.dylib
    ln -sf "$LIBRARY_PATH" whisper.cpp/libwhisper.dylib
fi

# Create symlinks for stream executable if built
if [ -f "whisper.cpp/build/bin/whisper-stream" ]; then
    echo "Creating symbolic link for whisper-stream executable"
    ln -sf "$(pwd)/whisper.cpp/build/bin/whisper-stream" ./whisper-stream
fi

# Create symlinks for all GGML header files
echo "Creating symbolic links for header files..."

# Create symbolic link for ggml.h
if [ -f "whisper.cpp/ggml/include/ggml.h" ]; then
    echo "Creating symbolic link for ggml.h from ggml/include"
    ln -sf "$(pwd)/whisper.cpp/ggml/include/ggml.h" whisper.cpp/include/ggml.h
elif [ -f "whisper.cpp/build/src/ggml.h" ]; then
    echo "Creating symbolic link for ggml.h from build/src"
    ln -sf "$(pwd)/whisper.cpp/build/src/ggml.h" whisper.cpp/include/ggml.h
elif [ -f "whisper.cpp/src/ggml.h" ]; then
    echo "Creating symbolic link for ggml.h from src"
    ln -sf "$(pwd)/whisper.cpp/src/ggml.h" whisper.cpp/include/ggml.h
else
    echo "Warning: ggml.h not found"
fi

# Create symbolic links for all GGML header files from ggml/include
for header in whisper.cpp/ggml/include/*.h; do
    if [ -f "$header" ]; then
        filename=$(basename "$header")
        echo "Creating symbolic link for $filename from ggml/include"
        ln -sf "$(pwd)/$header" whisper.cpp/include/$filename
    fi
done

# Create symbolic links for common headers used by streaming functionality
for header in whisper.cpp/examples/common*.h; do
    if [ -f "$header" ]; then
        filename=$(basename "$header")
        echo "Creating symbolic link for $filename from examples"
        ln -sf "$(pwd)/$header" whisper.cpp/include/$filename
    fi
done

# Create symbolic link for whisper.h
if [ -f "whisper.cpp/include/whisper.h" ]; then
    echo "Found whisper.h in include directory"
    ln -sf "$(pwd)/whisper.cpp/include/whisper.h" whisper.h
else
    echo "Warning: whisper.h not found in include directory"
fi

echo "Building Go bindings..."
cd whisper.cpp/bindings/go
make whisper
cd ../../..

# Create ~/.ramble directory for system-wide installation
echo "Setting up headers and libraries in ~/.ramble directory..."
mkdir -p ~/.ramble/include
mkdir -p ~/.ramble/lib
mkdir -p ~/.ramble/bin

# Copy all GGML headers to ~/.ramble/include
echo "Copying headers to ~/.ramble/include"
cp -f whisper.cpp/include/whisper.h ~/.ramble/include/
cp -f whisper.cpp/include/ggml*.h ~/.ramble/include/
cp -f whisper.cpp/ggml/include/*.h ~/.ramble/include/

# Copy all common headers used by streaming functionality
echo "Copying streaming-related headers to ~/.ramble/include"
cp -f whisper.cpp/examples/common*.h ~/.ramble/include/

# Copy the libraries to ~/.ramble/lib
echo "Copying libraries to ~/.ramble/lib"
if [ -f "whisper.cpp/build/src/libwhisper.so" ]; then
    cp -f whisper.cpp/build/src/libwhisper.so ~/.ramble/lib/
    # Also create so.1 symlink for better compatibility
    ln -sf ~/.ramble/lib/libwhisper.so ~/.ramble/lib/libwhisper.so.1
fi
if [ -f "whisper.cpp/build/src/libwhisper.a" ]; then
    cp -f whisper.cpp/build/src/libwhisper.a ~/.ramble/lib/
fi
if [ -f "whisper.cpp/build/src/libwhisper.dylib" ]; then
    cp -f whisper.cpp/build/src/libwhisper.dylib ~/.ramble/lib/
fi

# Copy stream executable to ~/.ramble/bin if available
if [ -f "whisper.cpp/build/bin/whisper-stream" ]; then
    echo "Copying whisper-stream executable to ~/.ramble/bin"
    cp -f whisper.cpp/build/bin/whisper-stream ~/.ramble/bin/
    chmod +x ~/.ramble/bin/whisper-stream
fi

echo "Headers and libraries installed to ~/.ramble"

echo "==== Downloading basic models ===="
# Download basic models if they don't exist
if [ ! -d "whisper.cpp/models" ]; then
    mkdir -p whisper.cpp/models
fi

# Download tiny and small models if they don't exist
if [ ! -f "whisper.cpp/models/ggml-tiny.bin" ] && [ ! -f "whisper.cpp/models/ggml-tiny.en.bin" ]; then
    cd whisper.cpp && bash ./models/download-ggml-model.sh tiny && bash ./models/download-ggml-model.sh tiny.en && cd ..
fi

if [ ! -f "whisper.cpp/models/ggml-small.bin" ] && [ ! -f "whisper.cpp/models/ggml-small.en.bin" ]; then
    cd whisper.cpp && bash ./models/download-ggml-model.sh small.en && cd ..
fi

echo "==== Additional Models ===="
echo "Basic models (tiny, small) have been downloaded."
echo "If you want to pre-download other models, you can run:"
echo "  cd whisper.cpp && bash ./models/download-ggml-model.sh [size]"
echo "Where [size] is one of: base, medium, large"
echo "Models will be stored in whisper.cpp/models/"

# Verify Go bindings
./check-go-bindings.sh

echo "==== Build complete! ===="
echo "You can now build the application with: go build -tags=whisper_go -o ramble ./cmd/ramble"
echo "Or run tests with: go test -tags=whisper_go ./..."

# Build the application with go bindings
CGO_LDFLAGS="-L$(pwd)/whisper.cpp -L$(pwd)/whisper.cpp/build -L$(pwd)/whisper.cpp/build/src -L$(pwd) -L${HOME}/.ramble/lib" CGO_CFLAGS="-I$(pwd)/whisper.cpp/include -I${HOME}/.ramble/include" go build -tags=whisper_go -ldflags="-extldflags '-Wl,-rpath,$(pwd) -Wl,-rpath,${HOME}/.ramble/lib'" -o ramble ./cmd/ramble