#!/bin/bash
set -e

# Define variables
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
IMAGE_NAME="ramble-builder"
OUTPUT_DIR="$PROJECT_ROOT/dist"

# Ensure output directory exists
mkdir -p "$OUTPUT_DIR"

echo "Building Docker image..."
docker build -t "$IMAGE_NAME" -f "$PROJECT_ROOT/Dockerfile.build" "$PROJECT_ROOT"

echo "Running build in Docker container..."
docker run --rm \
  -v "$OUTPUT_DIR:/output" \
  --name ramble-build \
  "$IMAGE_NAME" \
  bash -c 'go build -v -tags=whisper_go -o /output/ramble ./cmd/ramble && cp libs/lib/libwhisper.so /output/'

echo "Creating run script..."
cat > "$OUTPUT_DIR/run.sh" << 'EOL'
#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export LD_LIBRARY_PATH="$SCRIPT_DIR:$LD_LIBRARY_PATH"
"$SCRIPT_DIR/ramble" "$@"
EOL
chmod +x "$OUTPUT_DIR/run.sh"

echo "Build complete! Output files in $OUTPUT_DIR:"
ls -la "$OUTPUT_DIR"

echo "To run the application:"
echo "cd $OUTPUT_DIR && ./run.sh"

# Optional: cleanup Docker image to save space
if [ "$1" == "--cleanup" ]; then
  echo "Removing Docker image..."
  docker rmi "$IMAGE_NAME"
fi