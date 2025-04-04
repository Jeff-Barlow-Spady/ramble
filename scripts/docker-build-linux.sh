#!/bin/bash
set -e

# Script to build Ramble for Linux using Docker
# This avoids dependency issues and ensures consistent builds

echo "=== Ramble Docker Build Script (Linux) ==="
echo ""

# Get script directory and project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
DIST_DIR="$PROJECT_ROOT/dist"
WHISPER_VERSION="1.7.5"  # Set this to match the version you want to use

# Create dist directory
mkdir -p "$DIST_DIR"

# Step 1: Check for Docker
if ! command -v docker &>/dev/null; then
    echo "Error: Docker is required but not installed"
    echo "Please install Docker and try again"
    exit 1
fi

# Step 2: Build the Docker image with the proper context
echo "Building Docker image for Ramble..."
cat > "$PROJECT_ROOT/Dockerfile.linux" << 'EOF'
# === Stage 1: Base Dependencies ===
FROM golang:1.23-bookworm AS base
WORKDIR /app

# Install Linux build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    cmake \
    git \
    curl \
    tar \
    file \
    # Runtime deps for whisper.cpp/Go app (needed for linking/packaging)
    libsdl2-dev \
    portaudio19-dev \
    # AppImage deps
    libfuse3-dev \
    libglib2.0-dev \
    # UI deps
    libayatana-appindicator3-dev \
    libgtk-3-dev \
    pkg-config \
    && rm -rf /var/lib/apt/lists/*

# === Stage 2: Build whisper.cpp for Linux ===
FROM base AS whisper-builder
WORKDIR /app/whisper.cpp-build

# Clone whisper.cpp
RUN git clone --depth 1 https://github.com/ggerganov/whisper.cpp.git .

# Build whisper.cpp shared libraries for Linux
RUN cmake -B build -DWHISPER_BUILD_GO_BINDINGS=1 -DWHISPER_BUILD_SHARED=1 .
RUN cmake --build build --config Release --parallel $(nproc)

# === Stage 3: Build Go App and Package for Linux ===
FROM base AS builder
WORKDIR /app

# Copy source code
COPY . .

# Create libs directory
RUN mkdir -p /app/libs/linux

# Copy built Linux libraries from whisper builder stage
COPY --from=whisper-builder /app/whisper.cpp-build/build/src/libwhisper.so* /app/libs/linux/
COPY --from=whisper-builder /app/whisper.cpp-build/build/ggml/src/libggml*.so* /app/libs/linux/

# Set environment for build
ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=amd64
ENV WHISPER_CPP_DIR=/app/whisper.cpp-build
ENV LIBRARY_PATH=/app/libs/linux
ENV LD_LIBRARY_PATH=/app/libs/linux
ENV C_INCLUDE_PATH=/app/whisper.cpp-build/include:/app/whisper.cpp-build/ggml/include
ENV CGO_CFLAGS="-I/app/whisper.cpp-build/include -I/app/whisper.cpp-build/ggml/include"
ENV CGO_LDFLAGS="-L/app/libs/linux"

# Create output directory structure
RUN mkdir -p /app/dist/linux-pkg/libs && \
    mkdir -p /app/dist/linux-pkg/models

# Build with Go whisper bindings tag and rpath
RUN echo "Building Linux binary..." && \
    go build -v -tags=whisper_go \
    -ldflags="-s -w -extldflags '-Wl,-rpath,\$ORIGIN/libs'" \
    -o /app/dist/linux-pkg/ramble ./cmd/ramble

# --- Package Linux ---
# Copy libraries to package directory
RUN cp /app/libs/linux/* /app/dist/linux-pkg/libs/

# Download models
ARG MODEL_TINY_URL="https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin"
ARG MODEL_TINY_NAME="ggml-tiny.bin"
RUN curl -L -o /app/dist/linux-pkg/models/${MODEL_TINY_NAME} ${MODEL_TINY_URL}

# Copy README
RUN cp README.md /app/dist/linux-pkg/

# Create Linux run script
RUN echo '#!/bin/bash' > /app/dist/linux-pkg/run.sh && \
    echo 'SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"' >> /app/dist/linux-pkg/run.sh && \
    echo 'export LD_LIBRARY_PATH="$SCRIPT_DIR/libs:$LD_LIBRARY_PATH"' >> /app/dist/linux-pkg/run.sh && \
    echo '"$SCRIPT_DIR/ramble" "$@"' >> /app/dist/linux-pkg/run.sh && \
    chmod +x /app/dist/linux-pkg/run.sh

# Create Linux tar.gz package
RUN cd /app/dist && tar -czf ramble-linux-amd64.tar.gz -C linux-pkg .

# Final stage just holds the artifacts
FROM scratch AS final-artifacts
COPY --from=builder /app/dist /app/dist
EOF

# Step 3: Build using Docker
echo "Building Ramble with Docker..."
docker build \
    -t ramble-linux-build \
    -f "$PROJECT_ROOT/Dockerfile.linux" \
    --output type=local,dest="$DIST_DIR" \
    --target final-artifacts \
    "$PROJECT_ROOT"

if [ $? -eq 0 ]; then
    echo ""
    echo "✅ Build completed successfully!"
    echo "Artifact location: $DIST_DIR/ramble-linux-amd64.tar.gz"
    echo ""
    echo "To install the application, extract the tar.gz:"
    echo "  tar -xzf $DIST_DIR/ramble-linux-amd64.tar.gz -C /tmp"
    echo "  cd /tmp"
    echo "  ./run.sh"
else
    echo ""
    echo "❌ Build failed."
    exit 1
fi