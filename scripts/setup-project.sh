#!/bin/bash
set -e

echo "=== Ramble Project Setup ==="
echo ""
echo "This script will organize the project according to the standard structure."
echo ""

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Create necessary directories
mkdir -p "$PROJECT_ROOT/assets"
mkdir -p "$PROJECT_ROOT/lib/windows"
mkdir -p "$PROJECT_ROOT/models"
mkdir -p "$PROJECT_ROOT/dist"

# Check if we need to move any files
if [ -f "$PROJECT_ROOT/run.sh" ]; then
    echo "Moving run.sh to scripts/"
    cp "$PROJECT_ROOT/run.sh" "$SCRIPT_DIR/run.sh"
    chmod +x "$SCRIPT_DIR/run.sh"
fi

if [ -f "$PROJECT_ROOT/build-dist.sh" ]; then
    echo "Backup the old build-dist.sh"
    mv "$PROJECT_ROOT/build-dist.sh" "$PROJECT_ROOT/build-dist.sh.old"
fi

# Check for libraries and move them
echo "Checking for library files..."
SO_FILES=$(find "$PROJECT_ROOT" -maxdepth 1 -name "*.so*" | wc -l)
if [ "$SO_FILES" -gt 0 ]; then
    echo "Moving shared libraries to lib/"
    find "$PROJECT_ROOT" -maxdepth 1 -name "*.so*" -exec mv {} "$PROJECT_ROOT/lib/" \;
fi

DLL_FILES=$(find "$PROJECT_ROOT" -maxdepth 1 -name "*.dll" | wc -l)
if [ "$DLL_FILES" -gt 0 ]; then
    echo "Moving DLL files to lib/windows/"
    find "$PROJECT_ROOT" -maxdepth 1 -name "*.dll" -exec mv {} "$PROJECT_ROOT/lib/windows/" \;
fi

# Check for model files
echo "Checking for model files..."
BIN_FILES=$(find "$PROJECT_ROOT" -maxdepth 1 -name "*.bin" | wc -l)
if [ "$BIN_FILES" -gt 0 ]; then
    echo "Moving model files to models/"
    find "$PROJECT_ROOT" -maxdepth 1 -name "*.bin" -exec mv {} "$PROJECT_ROOT/models/" \;
fi

# Make build scripts executable
chmod +x "$SCRIPT_DIR/build-dist.sh"
chmod +x "$SCRIPT_DIR/linux/install.sh"

echo ""
echo "✅ Project setup complete!"
echo ""
echo "Directory structure:"
echo "  - $PROJECT_ROOT/assets/         # Application assets (icons, etc.)"
echo "  - $PROJECT_ROOT/cmd/            # Main application code"
echo "  - $PROJECT_ROOT/dist/           # Distribution output"
echo "  - $PROJECT_ROOT/lib/            # Libraries for Linux/Windows"
echo "  - $PROJECT_ROOT/models/         # Default speech recognition models"
echo "  - $PROJECT_ROOT/pkg/            # Package code for reusable components"
echo "  - $PROJECT_ROOT/scripts/        # Build and installation scripts"
echo ""
echo "Next steps:"
echo "  1. Run './scripts/build-dist.sh' to build distribution packages"
echo "  2. Distribute the generated packages to your users"
echo ""
echo "Happy coding!"