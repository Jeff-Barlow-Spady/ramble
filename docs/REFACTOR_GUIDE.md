# Building a Ramble Application with Proper Go Bindings for Whisper.cpp

## Core Components Overview

### 1. whisper.cpp Go Bindings
- Use `github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper` directly
- These bindings already provide complete access to whisper.cpp functionality
- Key types: `whisper.Model`, `whisper.Context`, and callback mechanisms

### 2. Fyne UI Framework
- Modern, lightweight UI toolkit for Go
- Provides widgets, layouts, event handling, and system tray functionality
- Use directly without additional abstraction layers

## Step-by-Step Implementation Guide

### 1. Project Structure
```
ramble/
├── cmd/
│   └── ramble/
│       └── main.go           # Entry point
├── pkg/
│   ├── audio/                # Audio capture
│   │   └── capture.go
│   ├── transcription/        # Direct usage of whisper bindings
│   │   └── manager.go        # Manages audio buffer and transcription
│   └── ui/                   # UI components
│       ├── app.go            # Main application UI
│       └── systray.go        # System tray functionality
├── go.mod
├── go.sum
└── run.sh                    # Build and run script
```

### 2. Transcription Manager Implementation

The critical part that needs fixing is the transcription manager that directly uses the whisper.cpp Go bindings:

```go
// manager.go
package transcription

import (
    "sync"
    "strings"

    "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
    "github.com/jeff-barlow-spady/ramble/pkg/logger"
)

// Manager handles whisper.cpp transcription with proper buffer management
type Manager struct {
    model          whisper.Model
    context        whisper.Context
    buffer         []float32
    minSamples     int         // Minimum samples (16000 = 1 second at 16kHz)
    recordingActive bool
    textCallback   func(string)
    mu             sync.Mutex
}

// NewManager creates a new transcription manager
func NewManager(modelPath string) (*Manager, error) {
    // Load the whisper model
    model, err := whisper.New(modelPath)
    if err != nil {
        return nil, err
    }

    // Create the whisper context
    context, err := model.NewContext()
    if err != nil {
        model.Close()
        return nil, err
    }

    return &Manager{
        model:       model,
        context:     context,
        buffer:      make([]float32, 0, 16000*5), // Pre-allocate 5 seconds
        minSamples:  16000,                       // 1 second minimum (16kHz)
        textCallback: nil,
    }, nil
}

// SetCallback sets the function to call with transcription results
func (m *Manager) SetCallback(cb func(string)) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.textCallback = cb
}

// ProcessAudio adds audio to the buffer and processes when enough is available
func (m *Manager) ProcessAudio(samples []float32) {
    if len(samples) == 0 {
        return
    }

    m.mu.Lock()
    defer m.mu.Unlock()

    if !m.recordingActive {
        return
    }

    // Add new audio to buffer
    m.buffer = append(m.buffer, samples...)

    // Only process if we have at least 1 second of audio (whisper requirement)
    if len(m.buffer) < m.minSamples {
        logger.Debug(logger.CategoryTranscription,
            "Buffer too small: %d ms < 1000 ms", len(m.buffer)*1000/16000)
        return
    }

    // Define segment callback to receive transcription results
    segmentCallback := func(segment whisper.Segment) {
        if m.textCallback != nil && m.recordingActive {
            text := strings.TrimSpace(segment.Text)
            if text != "" {
                logger.Debug(logger.CategoryTranscription, "Received segment: %s", text)
                m.textCallback(text)
            }
        }
    }

    // Process the audio buffer
    if err := m.context.Process(
        m.buffer,
        nil,                // No encoder begin callback needed
        segmentCallback,    // Handle text segments
        nil,                // No progress callback needed
    ); err != nil {
        logger.Warning(logger.CategoryTranscription,
            "Error processing audio: %v", err)
        return
    }

    // Keep a sliding window of audio for context
    // 30 seconds maximum is a good balance between context and memory usage
    const maxBufferSeconds = 30
    maxBufferLen := 16000 * maxBufferSeconds
    if len(m.buffer) > maxBufferLen {
        m.buffer = m.buffer[len(m.buffer)-maxBufferLen:]
    }
}

// StartRecording begins audio transcription
func (m *Manager) StartRecording() {
    m.mu.Lock()
    defer m.mu.Unlock()

    // Clear buffer and set state
    m.buffer = m.buffer[:0]
    m.recordingActive = true

    // Configure the context for streaming
    m.configureContext()

    logger.Info(logger.CategoryTranscription,
        "Starting whisper transcription")
}

// StopRecording ends audio transcription
func (m *Manager) StopRecording() {
    m.mu.Lock()
    defer m.mu.Unlock()

    m.recordingActive = false
    m.buffer = m.buffer[:0]

    logger.Info(logger.CategoryTranscription,
        "Stopping whisper transcription")
}

// configureContext sets optimal parameters for streaming transcription
func (m *Manager) configureContext() {
    // Basic configuration - language, performance settings
    _ = m.context.SetLanguage("en")
    m.context.SetThreads(8)
    m.context.SetSplitOnWord(true)

    // These settings work well for streaming applications
    m.context.SetMaxSegmentLength(0)    // Don't artificially limit segments
    m.context.SetTokenTimestamps(true)  // Enable timestamps for words
}

// Close releases resources
func (m *Manager) Close() {
    m.mu.Lock()
    defer m.mu.Unlock()

    if m.model != nil {
        m.model.Close()
        m.model = nil
    }
    m.context = nil
    m.buffer = nil
}
```

## Implementation Notes

1. **Proper Buffer Management**
   - Whisper requires a minimum of 1000ms of audio data (16000 samples at 16kHz)
   - Use a sliding window to maintain context while preventing memory overuse
   - Handle threading properly with mutexes

2. **Performance Considerations**
   - Avoid unnecessary abstractions over the whisper.cpp Go bindings
   - Configure context appropriately for streaming applications
   - Release resources properly in Close()

3. **Integration with UI**
   - Use callbacks to decouple the transcription engine from the UI
   - Allow for flexible configuration of models at runtime
   - Handle error cases gracefully with proper logging

4. **Cross-Platform Considerations**
   - Ensure libraries are properly loaded across different platforms
   - Use appropriate directory structure for models and libraries
   - Provide feedback to users on model loading or transcription issues