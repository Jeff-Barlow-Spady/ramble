# Go Bindings for Whisper.cpp in Ramble

This document explains how to properly build and use the Go bindings for Whisper.cpp in Ramble.

## Overview

Ramble uses the Whisper.cpp library for speech-to-text transcription. The recommended approach is to use the official Go bindings from the Whisper.cpp repository, which provides several benefits:

1. **Performance**: Direct integration with the C++ library without subprocess overhead
2. **Reliability**: Avoids many potential failure points of subprocess-based approaches
3. **Real-time streaming**: Native support for streaming audio transcription
4. **Resource efficiency**: No need to launch external processes

## Building with Go Bindings

### Prerequisites

1. Install build essentials:
   ```bash
   # Ubuntu/Debian
   sudo apt-get install build-essential

   # macOS
   xcode-select --install
   ```

2. Make sure you have Go 1.16+ installed

### Build Steps

1. Clone the repository and navigate to it:
   ```bash
   git clone https://github.com/jeff-barlow-spady/ramble.git
   cd ramble
   ```

2. Build Whisper.cpp with Go bindings:
   ```bash
   make build-whisper
   ```

3. Build the Ramble application:
   ```bash
   make build
   ```

4. Run the application:
   ```bash
   ./ramble
   ```

## Troubleshooting

### Common Issues

1. **Missing whisper.cpp repository**:
   If the whisper.cpp repository is not found, the Makefile will automatically clone it. If you prefer to use a specific version, clone the repository manually before building.

2. **Compilation failures**:
   Make sure you have the necessary dependencies installed:
   ```bash
   # Ubuntu/Debian
   sudo apt-get install build-essential libomp-dev

   # macOS
   brew install libomp
   ```

3. **Go binding import errors**:
   If you encounter import errors, make sure the whisper.cpp repository is properly cloned and built:
   ```bash
   make clean
   make build-whisper
   ```

## Comparison with Subprocess Approach

The previous implementation used a subprocess-based approach, which had several disadvantages:

1. **Performance overhead**: Creating and managing subprocesses for each transcription
2. **Error-prone**: Frequent errors related to streaming not being available
3. **Resource inefficiency**: Higher CPU and memory usage
4. **Complexity**: More complex code to handle subprocess management

The Go bindings approach simplifies the codebase significantly and provides a more efficient and reliable transcription experience.

## Technical Details

The Go bindings work by directly linking to the Whisper.cpp library using CGO. This provides native performance without the overhead of interprocess communication or file I/O.

The implementation is located in:
- `pkg/transcription/whisper_go_binding.go`: The Go binding implementation
- `pkg/transcription/whisper.go`: The main entry point for transcription

The build system ensures that the Whisper.cpp library is properly built and linked with the Go application.