// Package transcription provides speech-to-text functionality
package transcription

import (
	"sync"
)

// Model size options for Whisper
type ModelSize string

const (
	ModelTiny   ModelSize = "tiny"
	ModelBase   ModelSize = "base"
	ModelSmall  ModelSize = "small"
	ModelMedium ModelSize = "medium"
	ModelLarge  ModelSize = "large"
)

// Config holds configuration for the transcriber
type Config struct {
	// Model size to use (tiny, base, small, medium, large)
	ModelSize ModelSize
	// Path to model files (if empty, uses default location)
	ModelPath string
	// Language code (if empty, auto-detects)
	Language string
}

// DefaultConfig returns a reasonable default configuration
func DefaultConfig() Config {
	return Config{
		ModelSize: ModelSmall, // Small is a good balance of accuracy and performance
		ModelPath: "",         // Use default location
		Language:  "",         // Auto-detect language
	}
}

// Transcriber handles speech-to-text conversion
type Transcriber struct {
	config Config
	// TODO: Add Whisper model instance once we have Go bindings imported
	mu sync.Mutex
}

// NewTranscriber creates a new transcription service with the given configuration
func NewTranscriber(config Config) (*Transcriber, error) {
	// TODO: Initialize Whisper model
	// For now, we'll create a placeholder implementation

	transcriber := &Transcriber{
		config: config,
	}

	return transcriber, nil
}

// Transcribe converts audio data to text
// This is a placeholder implementation until we integrate Whisper
func (t *Transcriber) Transcribe(audioData []float32) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// TODO: Implement actual transcription with Whisper
	// For now, return a placeholder message
	return "Speech transcription placeholder (Whisper integration pending)", nil
}

// Close frees resources associated with the transcriber
func (t *Transcriber) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// TODO: Free Whisper model resources
	return nil
}

// ProcessAudioChunk processes a chunk of audio data and returns any transcribed text
// This is meant for streaming audio in real-time
func (t *Transcriber) ProcessAudioChunk(audioData []float32) (string, error) {
	// For simplicity in this initial implementation, we'll just pass to the main Transcribe method
	// In a real implementation, this would handle audio buffering and incremental processing
	return t.Transcribe(audioData)
}

// Implementation note:
// -------------------
// Full Whisper integration requires the Go bindings for whisper.cpp:
// github.com/ggerganov/whisper.cpp/bindings/go
//
// Since those bindings require CGO and the underlying C++ library,
// we're providing this placeholder implementation that can be filled
// in once the project has the proper dependencies set up.
