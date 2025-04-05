#!/bin/bash
set -e

# Simple Docker build script that keeps all artifacts contained
# and only extracts the final binary

echo "Building Docker image..."
docker build -t ramble-app -f Dockerfile.build .

echo "Extracting built binary to ./dist/..."
mkdir -p dist
docker create --name temp-container ramble-app
docker cp temp-container:/app/ramble ./dist/
docker cp temp-container:/usr/local/lib/libwhisper.so ./dist/
# Copy GGML libraries
docker cp temp-container:/usr/local/lib/libggml.so ./dist/
docker cp temp-container:/usr/local/lib/libggml-base.so ./dist/
docker cp temp-container:/usr/local/lib/libggml-cpu.so ./dist/
docker rm temp-container

echo "Creating run script..."
cat > ./dist/run.sh << 'EOL'
#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export LD_LIBRARY_PATH="$SCRIPT_DIR:$LD_LIBRARY_PATH"
"$SCRIPT_DIR/ramble" "$@"
EOL
chmod +x ./dist/run.sh

echo "Build complete!"
echo "Run the application with: ./dist/run.sh"