# Whisper Executable Integration

This document explains how Ramble integrates with the Whisper speech recognition tool.

## Overview

Ramble uses [whisper.cpp](https://github.com/ggerganov/whisper.cpp) for speech-to-text transcription. Instead of directly linking with the C++ library via CGO, Ramble uses the command-line executable approach for better compatibility across platforms.

## Whisper Executable

The application will attempt to find a Whisper executable in the following order:

1. Check if an embedded executable is available in the application bundle (typically available in official releases)
2. Look for a Whisper executable in the system's PATH
3. Check common installation locations based on the operating system
4. (On supported platforms) Attempt to auto-download and install the executable

### Supported Executables

Ramble looks for various executable names:

- `whisper`
- `whisper-cpp`
- `whisper.exe` (Windows)
- `whisper-cpp.exe` (Windows)
- `whisper-gael` (Python port)
- `whisper-gael.exe` (Windows Python port)
- `whisper.py` (Original Python)

### Common Installation Locations

The application checks these common installation paths:

#### Linux/macOS
- `/usr/local/bin/whisper`
- `/usr/bin/whisper`
- `/opt/whisper/bin/whisper`
- `/opt/whisper.cpp/whisper`
- `/opt/whisper.cpp/main`
- `~/.local/bin/whisper`
- `~/.local/bin/whisper-cpp`

#### Windows
- `C:\Program Files\whisper.cpp\whisper.exe`
- `C:\Program Files (x86)\whisper.cpp\whisper.exe`

## Whisper Models

Ramble can use various Whisper model sizes:

- `tiny`: Fastest, but least accurate
- `base`: Good balance for quick responses
- `small`: Better accuracy, reasonable speed
- `medium`: High accuracy, slower
- `large`: Best accuracy, slowest

Models will be:
1. Extracted from embedded resources if available
2. Downloaded from Hugging Face if not found
3. Stored in `~/.local/share/ramble/models/` (Linux/macOS) or appropriate Windows location

## Configuration

You can configure the Whisper integration in your application through the `Config` struct:

```go
// Create a default configuration
config := transcription.DefaultConfig()

// Or customize it
config := transcription.Config{
    ModelSize:              transcription.ModelTiny, // Choose model size
    ModelPath:              "",                      // Custom model path (optional)
    Language:               "en",                    // Target language
    Debug:                  false,                   // Enable debug output
    ExecutablePath:         "",                      // Custom executable path (optional)
    PreferSystemExecutable: true,                    // Prefer system executable over auto-installation
    Finder:                 &DefaultExecutableFinder{}, // Custom executable finder (optional)
}

transcriber, err := transcription.NewTranscriber(config)
```

## Dependency Injection

The package uses dependency injection to manage the executable finding and installation process. You can provide your own implementation of the `ExecutableFinder` interface for custom behavior:

```go
// Define the interface
type ExecutableFinder interface {
    FindExecutable() string
    InstallExecutable() (string, error)
}

// Use a custom implementation
config := transcription.DefaultConfig()
config.Finder = &MyCustomFinder{}
transcriber, err := transcription.NewTranscriber(config)
```

This approach makes the code more testable and flexible, allowing you to mock the executable finding and installation process in tests or provide custom implementations for different platforms.

## Auto-Installation

On supported platforms (currently Linux AMD64), if no executable is found, Ramble can automatically download and install the Whisper executable to `~/.local/share/ramble/bin/`.

This feature can be disabled by setting `PreferSystemExecutable` to `false` in the configuration.

## CI/CD Integration

When integrating with CI/CD pipelines, ensure the Whisper executable is available or downloaded before testing. The `.gitlab-ci.yml` file in the project demonstrates this setup.

## Troubleshooting

If you encounter issues with the Whisper executable:

1. Check if the Whisper executable is installed and in your PATH
2. Try specifying the full path in the configuration
3. On unsupported platforms, manually install [whisper.cpp](https://github.com/ggerganov/whisper.cpp) and ensure it's in your PATH
4. Check the application logs for specific error messages

## Best Practices

- For development, install whisper.cpp locally and add it to your PATH
- For production, use the auto-installation feature or bundle the executable with your application
- Choose the appropriate model size based on your accuracy vs. speed requirements
- Test with realistic audio samples to ensure proper functionality
- Use dependency injection to provide custom implementations of the ExecutableFinder for different environments