#!/bin/bash
set -e

echo "=== Ramble Distribution Builder ==="
echo ""

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
DIST_DIR="$PROJECT_ROOT/dist"
PLATFORMS=("linux" "windows")

# Create distribution directories
mkdir -p "$DIST_DIR"
for platform in "${PLATFORMS[@]}"; do
    mkdir -p "$DIST_DIR/$platform"
    mkdir -p "$DIST_DIR/$platform/libs"
    mkdir -p "$DIST_DIR/$platform/models"
done

# Build functions
build_linux() {
    echo "Building for Linux..."

    # Set CGO flags for Linux build
    export CGO_ENABLED=1
    export GOOS=linux
    export GOARCH=amd64

    # Compile the application
    cd "$PROJECT_ROOT"
    go build -tags=whisper_go -o "$DIST_DIR/linux/ramble" ./cmd/ramble

    # Copy required libraries
    if [ -d "$PROJECT_ROOT/lib" ]; then
        cp -r "$PROJECT_ROOT/lib/"*.so* "$DIST_DIR/linux/libs/"
        echo "Copied libraries to $DIST_DIR/linux/libs/"
    else
        echo "Warning: lib directory not found. Manual library setup may be required."
    fi

    # Copy default models if they exist
    if [ -d "$PROJECT_ROOT/models" ]; then
        cp -r "$PROJECT_ROOT/models/"* "$DIST_DIR/linux/models/"
        echo "Copied models to $DIST_DIR/linux/models/"
    else
        echo "No default models found. Models will need to be downloaded by users."
    fi

    # Copy installation scripts
    cp "$SCRIPT_DIR/linux/install.sh" "$DIST_DIR/linux/"
    chmod +x "$DIST_DIR/linux/install.sh"

    # Create portable launch script
    cat > "$DIST_DIR/linux/ramble.sh" << 'EOL'
#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export LD_LIBRARY_PATH="$SCRIPT_DIR/libs:$LD_LIBRARY_PATH"
"$SCRIPT_DIR/ramble" "$@"
EOL
    chmod +x "$DIST_DIR/linux/ramble.sh"

    # Create application icon
    if [ -f "$PROJECT_ROOT/assets/ramble.png" ]; then
        cp "$PROJECT_ROOT/assets/ramble.png" "$DIST_DIR/linux/"
    else
        echo "No icon found, creating a default one..."
        if command -v convert >/dev/null 2>&1; then
            convert -size 256x256 xc:white -font DejaVu-Sans -pointsize 72 \
                -fill black -gravity center -annotate 0 "Ramble" \
                "$DIST_DIR/linux/ramble.png"
        else
            echo "ImageMagick not available, skipping icon creation."
        fi
    fi

    # Create README
    cat > "$DIST_DIR/linux/README.md" << 'EOL'
# Ramble - Speech-to-Text Transcription

## Installation

### Option 1: Easy Install (Recommended)
Run the installer script:
```
./install.sh
```
This will install Ramble to your user directory and create desktop shortcuts.

### Option 2: Portable Mode
If you prefer not to install, you can run directly:
```
./ramble.sh
```

## Default Model
This distribution includes a small default model. For better transcription quality,
you can download larger models through the application preferences.

## Troubleshooting
If you experience library issues, make sure you have the required dependencies:
- ALSA audio libraries (libasound2)
- GTK3 libraries (for the GUI)

For any other issues, please report them to the project repository.
EOL

    # Create tarball
    cd "$DIST_DIR"
    tar -czf "ramble-linux-amd64.tar.gz" linux/
    echo "Linux package created: $DIST_DIR/ramble-linux-amd64.tar.gz"
}

build_windows() {
    echo "Building for Windows..."

    # Check for MinGW or other Windows cross-compile tools
    if ! command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1; then
        echo "Warning: Windows cross-compiler not found (x86_64-w64-mingw32-gcc)"
        echo "Skipping Windows build. Install MinGW for Windows cross-compilation."
        return 1
    fi

    # Set CGO flags for Windows cross-compilation
    export CGO_ENABLED=1
    export GOOS=windows
    export GOARCH=amd64
    export CC=x86_64-w64-mingw32-gcc
    export CXX=x86_64-w64-mingw32-g++

    # Compile the application
    cd "$PROJECT_ROOT"
    go build -tags=whisper_go -o "$DIST_DIR/windows/ramble.exe" ./cmd/ramble

    # Copy required DLLs
    if [ -d "$PROJECT_ROOT/lib/windows" ]; then
        cp -r "$PROJECT_ROOT/lib/windows/"*.dll "$DIST_DIR/windows/libs/"
        echo "Copied Windows DLLs to $DIST_DIR/windows/libs/"
    else
        echo "Warning: Windows libraries not found. Manual library setup will be required."
    fi

    # Copy default models if they exist
    if [ -d "$PROJECT_ROOT/models" ]; then
        cp -r "$PROJECT_ROOT/models/"* "$DIST_DIR/windows/models/"
        echo "Copied models to $DIST_DIR/windows/models/"
    else
        echo "No default models found. Models will need to be downloaded by users."
    fi

    # Copy installation scripts
    cp "$SCRIPT_DIR/windows/install.bat" "$DIST_DIR/windows/"
    cp "$SCRIPT_DIR/windows/install.ps1" "$DIST_DIR/windows/"

    # Create application icon
    if [ -f "$PROJECT_ROOT/assets/ramble.ico" ]; then
        cp "$PROJECT_ROOT/assets/ramble.ico" "$DIST_DIR/windows/"
    elif [ -f "$PROJECT_ROOT/assets/ramble.png" ] && command -v convert >/dev/null 2>&1; then
        echo "Converting PNG icon to ICO format..."
        convert "$PROJECT_ROOT/assets/ramble.png" "$DIST_DIR/windows/ramble.ico"
    else
        echo "No icon found, Windows version will use default icon."
    fi

    # Create README
    cat > "$DIST_DIR/windows/README.md" << 'EOL'
# Ramble - Speech-to-Text Transcription

## Installation Options

### Option 1: Using the PowerShell Installer (Recommended)
1. Right-click on `install.ps1` and select "Run with PowerShell"
2. Follow the on-screen instructions

### Option 2: Using the Batch Installer
1. Double-click on `install.bat`
2. Follow the on-screen instructions

The application will be installed to your user's AppData directory and create shortcuts.

## Default Model
This distribution includes a small default model. For better transcription quality,
you can download larger models through the application preferences.

## Troubleshooting
If you experience any issues with installation:
- Make sure you have administrative rights when running the installer
- Some antivirus software might block the installer - you may need to temporarily disable it
- For any other issues, please report them to the project repository
EOL

    # Create zip archive
    cd "$DIST_DIR"
    if command -v zip >/dev/null 2>&1; then
        zip -r "ramble-windows-amd64.zip" windows/
        echo "Windows package created: $DIST_DIR/ramble-windows-amd64.zip"
    else
        echo "Warning: 'zip' command not found, Windows package not created."
        echo "Windows files are in: $DIST_DIR/windows/"
    fi
}

# Parse command line arguments
SELECTED_PLATFORMS=()
if [ $# -eq 0 ]; then
    # No arguments, build for all platforms
    SELECTED_PLATFORMS=("${PLATFORMS[@]}")
else
    for arg in "$@"; do
        if [[ " ${PLATFORMS[@]} " =~ " ${arg} " ]]; then
            SELECTED_PLATFORMS+=("$arg")
        else
            echo "Error: Unknown platform '$arg'"
            echo "Available platforms: ${PLATFORMS[*]}"
            exit 1
        fi
    done
fi

# Build for selected platforms
for platform in "${SELECTED_PLATFORMS[@]}"; do
    echo ""
    echo "===== Building for $platform ====="
    if [ "$platform" == "linux" ]; then
        build_linux
    elif [ "$platform" == "windows" ]; then
        build_windows || echo "Windows build failed or skipped"
    fi
done

echo ""
echo "Build process completed!"
echo "Distribution packages can be found in: $DIST_DIR"