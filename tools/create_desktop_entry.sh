#!/bin/bash
set -e

# Get the directory where Ramble is installed
if [ -d "${HOME}/.local/share/ramble" ]; then
  RAMBLE_DIR="${HOME}/.local/share/ramble"
else
  read -p "Enter the path where Ramble is installed: " RAMBLE_DIR

  if [ ! -d "$RAMBLE_DIR" ]; then
    echo "❌ Error: $RAMBLE_DIR is not a valid directory"
    exit 1
  fi

  if [ ! -f "$RAMBLE_DIR/ramble" ]; then
    echo "❌ Error: Ramble executable not found in $RAMBLE_DIR"
    exit 1
  fi
fi

# Create bin directory if it doesn't exist
BIN_DIR="${HOME}/.local/bin"
mkdir -p "$BIN_DIR"

# Create applications directory if it doesn't exist
APPS_DIR="${HOME}/.local/share/applications"
mkdir -p "$APPS_DIR"

# Create launcher script
echo "Creating launcher script in $BIN_DIR..."
cat > "${BIN_DIR}/ramble" << EOL
#!/bin/bash
export LD_LIBRARY_PATH="${RAMBLE_DIR}/libs:\$LD_LIBRARY_PATH"
cd "${RAMBLE_DIR}" && ./ramble "\$@"
EOL

chmod +x "${BIN_DIR}/ramble"

# Create desktop entry
echo "Creating desktop entry..."
cat > "${APPS_DIR}/ramble.desktop" << EOL
[Desktop Entry]
Name=Ramble
Comment=Speech-to-Text Transcription
Exec=${BIN_DIR}/ramble
Icon=${RAMBLE_DIR}/ramble.png
Terminal=false
Type=Application
Categories=Utility;AudioVideo;
Keywords=speech;transcription;audio;
EOL

# Check if icon exists, create a simple one if it doesn't
if [ ! -f "${RAMBLE_DIR}/ramble.png" ]; then
  echo "Creating a simple icon..."

  # Check if ImageMagick is installed
  if command -v convert >/dev/null 2>&1; then
    convert -size 256x256 xc:navy -fill white -gravity center -pointsize 72 -annotate 0 "R" "${RAMBLE_DIR}/ramble.png"
  else
    echo "⚠️ ImageMagick not found, skipping icon creation"
    # Update desktop entry to use a system icon instead
    sed -i 's|Icon=.*|Icon=audio-input-microphone|g' "${APPS_DIR}/ramble.desktop"
  fi
fi

echo "✅ Desktop entry created successfully!"
echo "You can now launch Ramble from your applications menu"
echo "or by typing 'ramble' in a terminal."