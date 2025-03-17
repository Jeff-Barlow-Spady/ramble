# Ramble

A cross-platform speech-to-text application built with Go.

## Features

- Global hotkey activation for hands-free operation
- Real-time audio capture and transcription
- Modern Fyne-based user interface
- Cross-platform support (Windows, macOS, Linux)

## Prerequisites

- Go 1.21 or later
- System dependencies:
  - **Ubuntu/Debian**:
    ```bash
    # For audio recording
    sudo apt-get install portaudio19-dev

    # For hotkey detection
    sudo apt-get install libxkbcommon-dev libxkbcommon-x11-dev libx11-dev libxtst-dev

    # For GUI
    sudo apt-get install libpng-dev libgl1-mesa-dev xorg-dev
    ```
  - **macOS**:
    ```bash
    brew install portaudio
    ```
  - **Windows**:
    - MinGW or MSYS2 environment is recommended
    - Install dependencies through the package manager

## Dependencies

- [Fyne](https://fyne.io/) - Cross-platform GUI framework
- [PortAudio](https://github.com/gordonklaus/portaudio) - Audio capture library
- [Whisper](https://github.com/ggerganov/whisper.cpp) - Speech recognition model
- [Gohook](https://github.com/robotn/gohook) - Global hotkey detection

## Development

```bash
# Install dependencies
go mod download

# Run the application
go run ./cmd/ramble

# Build the application
go build ./cmd/ramble
```

## Troubleshooting

### Build Errors
- If you encounter build errors related to missing header files, make sure you've installed all the required system dependencies as listed above.
- For audio-related issues, check that your microphone is properly connected and accessible.

## License

MIT