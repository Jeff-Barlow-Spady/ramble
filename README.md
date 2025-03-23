# Ramble

Ramble is a speech transcription and note-taking application focused on simplicity and privacy.

## Features

- Real-time speech-to-text transcription
- Note organization and editing
- Privacy-focused (all processing happens locally)
- Cross-platform support
- Customizable transcription models

## Installation

### Pre-built Binaries

Download the latest release for your platform from the [Releases](https://gitlab.com/username/ramble/-/releases) page.

### From Source

Requirements:
- Go 1.21 or later
- C compiler (for building whisper.cpp)

```bash
# Clone the repository
git clone https://gitlab.com/username/ramble.git
cd ramble

# Build with Go bindings for optimal performance
./build-go-bindings.sh
make build

# Or build without Go bindings (less efficient)
go build -o ramble ./cmd/ramble
```

## Speech Recognition

Ramble uses [whisper.cpp](https://github.com/ggerganov/whisper.cpp) for speech recognition. The application supports multiple integration methods in order of preference:

1. **Go Bindings** (recommended): Direct integration with whisper.cpp through its Go bindings
2. Executable-based approach: Using whisper.cpp executables

### Using Go Bindings (Recommended)

For optimal performance and reliability, we recommend using the Go bindings for whisper.cpp:

```bash
# Build with Go bindings
./build-go-bindings.sh
make build
```

The Go bindings provide:
- Better performance (no subprocess overhead)
- Improved reliability (no IPC issues)
- True streaming support
- Lower resource usage

See [README-GO-BINDINGS.md](README-GO-BINDINGS.md) for detailed information about using the Go bindings.

### Executable-based Approach (Legacy)

The application will also work with whisper executables:

1. Use embedded executables if available
2. Search for whisper.cpp in your PATH
3. Check common installation locations
4. Attempt to auto-download the executable (on supported platforms)

For detailed information, see [Whisper Usage Documentation](docs/WHISPER_USAGE.md).

### Whisper Models

Ramble supports multiple model sizes from tiny to large, trading off between speed and accuracy. The models are automatically downloaded if not found locally.

## Usage

```bash
# Start the application
./ramble

# Specify a custom model path
./ramble --model-path /path/to/model.bin

# Use a larger model for better accuracy
./ramble --model-size medium
```

## Configuration

Ramble can be configured through command-line flags or a configuration file.

```bash
# View available options
./ramble --help
```

## Development

### Setting up the Development Environment

```bash
# Install development dependencies
go get -d ./...

# Build whisper.cpp with Go bindings (for best performance)
./build-go-bindings.sh

# Run tests
go test ./...
```

### Testing with Different Models

```bash
# Run with a specific model
go run ./cmd/ramble --model-size tiny
```

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## License

This project is licensed under the [MIT License](LICENSE).