#!/bin/bash
set -e

# Define variables
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
OUTPUT_DIR="$PROJECT_ROOT/dist"

# Ensure output directory exists
mkdir -p "$OUTPUT_DIR"

echo "Building Docker image..."
docker build -t ramble-builder -f "$PROJECT_ROOT/Dockerfile.build" "$PROJECT_ROOT"

echo "Extracting build artifacts..."
# Create a temporary container
CONTAINER_ID=$(docker create ramble-builder)

# Extract the binary and library
docker cp $CONTAINER_ID:/app/ramble "$OUTPUT_DIR/"
docker cp $CONTAINER_ID:/app/libs/lib/libwhisper.so "$OUTPUT_DIR/"

# Remove the temporary container
docker rm $CONTAINER_ID

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
  docker rmi ramble-builder
fi