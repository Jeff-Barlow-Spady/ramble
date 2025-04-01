#!/bin/bash
set -e

echo "=== Ramble Speech-to-Text Installer for Linux ==="
echo ""

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARENT_DIR="$(dirname "$(dirname "$SCRIPT_DIR")")"

# Create application directory
APP_DIR="${HOME}/.local/share/ramble"
MODELS_DIR="$APP_DIR/models"
LOGS_DIR="$APP_DIR/logs"
BIN_DIR="${HOME}/.local/bin"

# Check for dependencies
echo "Checking dependencies..."
missing_deps=0

if ! command -v ldd >/dev/null 2>&1; then
    echo "⚠️ Warning: 'ldd' not found - cannot check library dependencies"
fi

echo "Creating application directories..."
mkdir -p "$APP_DIR"
mkdir -p "$MODELS_DIR"
mkdir -p "$LOGS_DIR"
mkdir -p "$BIN_DIR"

# Copy files
echo "Installing Ramble application..."
cp -r "$PARENT_DIR/dist/libs" "$APP_DIR/"
cp "$PARENT_DIR/dist/ramble" "$APP_DIR/"

# Test executable
if [ -f "$APP_DIR/ramble" ]; then
    chmod +x "$APP_DIR/ramble"
    echo "✓ Executable installed successfully"
else
    echo "❌ Error: Failed to install executable"
    exit 1
fi

# Copy models if they exist
if [ -d "$PARENT_DIR/dist/models" ] && [ "$(ls -A "$PARENT_DIR/dist/models")" ]; then
    echo "Installing default speech models..."
    cp "$PARENT_DIR/dist/models/"* "$MODELS_DIR/"
    echo "✓ Models installed successfully"
else
    echo "ℹ️ No bundled models found. You'll need to download models through the application preferences."
fi

# Create desktop shortcut
echo "Creating desktop shortcut..."
mkdir -p "${HOME}/.local/share/applications"
cat > "${HOME}/.local/share/applications/ramble.desktop" << EOL
[Desktop Entry]
Name=Ramble
Comment=Speech-to-Text Transcription
Exec=${HOME}/.local/bin/ramble
Icon=${APP_DIR}/ramble.png
Terminal=false
Type=Application
Categories=Utility;AudioVideo;
Keywords=speech;transcription;audio;
EOL

# Create launcher script
echo "Creating launcher script..."
cat > "${HOME}/.local/bin/ramble" << EOL
#!/bin/bash
export LD_LIBRARY_PATH="${APP_DIR}/libs:\$LD_LIBRARY_PATH"
cd "${APP_DIR}" && ./ramble "\$@"
EOL

chmod +x "${HOME}/.local/bin/ramble"

# Copy icon if it exists
if [ -f "$PARENT_DIR/dist/ramble.png" ]; then
    cp "$PARENT_DIR/dist/ramble.png" "$APP_DIR/"
    echo "✓ Application icon installed"
else
    echo "⚠️ Icon not found, application will use system default"
    # Update desktop entry to use a system icon
    sed -i 's|Icon=.*|Icon=audio-input-microphone|g' "${HOME}/.local/share/applications/ramble.desktop"
fi

# Check PATH
if ! echo "$PATH" | grep -q "${HOME}/.local/bin"; then
    echo "⚠️ Warning: ${HOME}/.local/bin is not in your PATH"
    echo "   Add the following line to your ~/.bashrc or ~/.profile:"
    echo "   export PATH=\"\$HOME/.local/bin:\$PATH\""

    # Ask if user wants to add it automatically
    read -p "Would you like to add it automatically? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        # Check if .bashrc exists, otherwise use .profile
        if [ -f "${HOME}/.bashrc" ]; then
            echo "export PATH=\"\$HOME/.local/bin:\$PATH\"" >> "${HOME}/.bashrc"
            echo "✓ Added to ~/.bashrc - you'll need to log out and back in or source the file"
        elif [ -f "${HOME}/.profile" ]; then
            echo "export PATH=\"\$HOME/.local/bin:\$PATH\"" >> "${HOME}/.profile"
            echo "✓ Added to ~/.profile - you'll need to log out and back in for changes to take effect"
        else
            echo "❌ Could not find ~/.bashrc or ~/.profile, please add it manually"
        fi
    fi
fi

echo ""
echo "✅ Ramble has been successfully installed!"
echo "You can now run it from your applications menu or by typing 'ramble' in a terminal."
echo ""
echo "Models are stored in: $MODELS_DIR"
echo "Logs are stored in:   $LOGS_DIR"
echo ""
echo "Enjoy using Ramble!"