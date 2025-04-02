 ███████████     █████████   ██████   ██████ ███████████  █████       ██████████
░░███░░░░░███   ███░░░░░███ ░░██████ ██████ ░░███░░░░░███░░███       ░░███░░░░░█
 ░███    ░███  ░███    ░███  ░███░█████░███  ░███    ░███ ░███        ░███  █ ░ 
 ░██████████   ░███████████  ░███░░███ ░███  ░██████████  ░███        ░██████   
 ░███░░░░░███  ░███░░░░░███  ░███ ░░░  ░███  ░███░░░░░███ ░███        ░███░░█   
 ░███    ░███  ░███    ░███  ░███      ░███  ░███    ░███ ░███      █ ░███ ░   █
 █████   █████ █████   █████ █████     █████ ███████████  ███████████ ██████████
░░░░░   ░░░░░ ░░░░░   ░░░░░ ░░░░░     ░░░░░ ░░░░░░░░░░░  ░░░░░░░░░░░ ░░░░░░░░░░ 
                                                                                
                                                                                
                                                                                

# Ramble

Ramble is a speech-to-text transcription application that provides real-time transcription using the Whisper model.

## Project Structure

```
ramble/
├── cmd/
│   └── ramble/         # Main application code
├── pkg/                # Package code for reusable components
│   └── transcription/  # Transcription engine implementation
├── scripts/            # Build and installation scripts
│   ├── build-dist.sh   # Main distribution builder
│   ├── linux/          # Linux-specific scripts
│   └── windows/        # Windows-specific scripts
├── dist/               # Distribution output (created by build scripts)
│   ├── linux/          # Linux distribution files
│   └── windows/        # Windows distribution files
├── assets/             # Application assets (icons, etc.)
└── models/             # Default speech recognition models
```

## Building the Application

### Prerequisites

#### For Linux Build
- Go 1.18 or later
- GCC and development libraries
- ALSA development libraries (`libasound2-dev`)
- GTK3 development libraries (`libgtk-3-dev`)

#### For Windows Cross-Compilation
- MinGW-w64 cross-compiler (`x86_64-w64-mingw32-gcc`)
- Windows libraries (placed in `lib/windows/`)

### Build Process

To build distribution packages for all supported platforms:

```bash
./scripts/build-dist.sh
```

To build for a specific platform only:

```bash
./scripts/build-dist.sh linux
# or
./scripts/build-dist.sh windows
```

The resulting distribution packages will be created in the `dist/` directory.

## Installation

### Linux

Extract the Linux distribution package and run:

```bash
./install.sh
```

Alternatively, for portable use without installation:

```bash
./ramble.sh
```

### Windows

Extract the Windows distribution package and run either:
- `install.ps1` (PowerShell, recommended)
- `install.bat` (Command Prompt)

## Development

### Running the Application Locally

```bash
# From project root
./scripts/run.sh
```

### Working with Go Bindings for Whisper.cpp

Ramble uses the Whisper.cpp library for speech-to-text transcription through the official Go bindings from the Whisper.cpp repository, which provides several benefits:

1. **Performance**: Direct integration with the C++ library without subprocess overhead
2. **Reliability**: Avoids many potential failure points of subprocess-based approaches
3. **Real-time streaming**: Native support for streaming audio transcription
4. **Resource efficiency**: No need to launch external processes

#### Setting up Whisper.cpp with Go Bindings

1. Build the Go bindings for Whisper.cpp:
   ```bash
   ./scripts/build-go-bindings.sh
   ```

2. Troubleshooting Go bindings:
   - If you encounter compilation failures, make sure you have the necessary dependencies installed:
     ```bash
     # Ubuntu/Debian
     sudo apt-get install build-essential libomp-dev
     ```
   - If you encounter import errors, rebuild the whisper.cpp components:
     ```bash
     ./scripts/build-go-bindings.sh --clean
     ```

### Adding Libraries

Place platform-specific libraries in:
- `lib/*.so` for Linux shared objects
- `lib/windows/*.dll` for Windows DLLs

## License

This project is licensed under the [MIT License](LICENSE).
