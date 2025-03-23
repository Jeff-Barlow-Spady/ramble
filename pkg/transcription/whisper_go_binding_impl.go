//go:build cgo && whisper_go
// +build cgo,whisper_go

// This file contains the actual implementation of the Go bindings for whisper.cpp.
// It is only compiled when the "whisper_go" tag is specified.

package transcription

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
	"github.com/jeff-barlow-spady/ramble/pkg/logger"
)

// WhisperGoTranscriber implements ConfigurableTranscriber using Go bindings
// for true native streaming without subprocesses
type WhisperGoTranscriber struct {
	config          Config
	mu              sync.Mutex
	recordingActive bool
	streamCallback  func(string)
	modelPath       string

	// Whisper context and state
	model      *whisper.Model
	context    *whisper.Context
	buffer     []float32
	bufferTime time.Time

	// Processing parameters
	processingRate int // samples per second (16000 Hz)
	minAudioLevel  float32
	minBufferLen   int // min samples before processing
}

// NewGoBindingTranscriber creates a transcriber using the Go bindings
func NewGoBindingTranscriber(config Config) (ConfigurableTranscriber, error) {
	// Find the model path
	modelPath := getLocalModelPath(config.ModelSize)
	if modelPath == "" {
		return nil, fmt.Errorf("whisper model not found for size: %s", config.ModelSize)
	}

	logger.Info(logger.CategoryTranscription, "Creating whisper transcriber with Go bindings using model: %s", modelPath)

	// Load the model
	model, err := whisper.New(modelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load whisper model: %w", err)
	}

	// Create context
	context, err := model.NewContext()
	if err != nil {
		model.Close()
		return nil, fmt.Errorf("failed to create whisper context: %w", err)
	}

	// Configure context based on user preferences
	if config.Language != "" && config.Language != "auto" {
		context.SetLanguage(config.Language)
	}

	// Set appropriate parameters
	transcriber := &WhisperGoTranscriber{
		config:         config,
		model:          model,
		context:        context,
		modelPath:      modelPath,
		processingRate: 16000, // Standard whisper rate
		minAudioLevel:  0.01,
		minBufferLen:   16000 / 2,                   // 0.5 seconds minimum
		buffer:         make([]float32, 0, 16000*5), // Pre-allocate 5 seconds
		bufferTime:     time.Now(),
	}

	logger.Info(logger.CategoryTranscription, "Whisper Go binding transcriber initialized successfully")
	return transcriber, nil
}

// ProcessAudioChunk processes a chunk of audio data in real-time
func (t *WhisperGoTranscriber) ProcessAudioChunk(audioData []float32) (string, error) {
	// Skip processing if no data
	if len(audioData) == 0 {
		return "", nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Skip if not recording
	if !t.recordingActive {
		if t.config.Debug {
			logger.Debug(logger.CategoryTranscription, "Skipping audio chunk processing - recording not active")
		}
		return "", nil
	}

	// Add the new audio data to the buffer
	t.buffer = append(t.buffer, audioData...)

	// Process if:
	// 1. Buffer is long enough (at least 0.5 second @ 16kHz)
	// 2. It's been at least 500ms since last processing
	if len(t.buffer) >= t.minBufferLen && time.Since(t.bufferTime) >= 500*time.Millisecond {
		t.bufferTime = time.Now()

		if t.config.Debug {
			logger.Debug(logger.CategoryTranscription, "Processing %d audio samples with Go bindings", len(t.buffer))
		}

		// Process the audio buffer
		segments, err := t.processAudioBuffer(t.buffer)
		if err != nil {
			// Just log the error and continue - don't return error to prevent log spam
			logger.Warning(logger.CategoryTranscription, "Error processing audio with Go bindings: %v", err)
			return "", nil
		}

		// If we got some text, send it via callback
		if len(segments) > 0 && t.streamCallback != nil {
			for _, segment := range segments {
				text := normalizeTranscriptionText(segment.Text)
				if text != "" {
					t.streamCallback(text)
				}
			}
		}

		// Keep the last second of audio for context
		if len(t.buffer) > t.processingRate {
			t.buffer = t.buffer[len(t.buffer)-t.processingRate:]
		} else {
			t.buffer = make([]float32, 0, t.processingRate*5)
		}
	}

	return "", nil
}

// processAudioBuffer processes audio data with whisper
func (t *WhisperGoTranscriber) processAudioBuffer(buffer []float32) ([]whisper.Segment, error) {
	if t.context == nil || t.model == nil {
		return nil, fmt.Errorf("whisper context or model is nil")
	}

	// Process the audio buffer
	if err := t.context.Process(buffer, nil); err != nil {
		return nil, fmt.Errorf("whisper process failed: %w", err)
	}

	// Get the segments
	return t.context.Segments(), nil
}

// Transcribe implements complete file transcription
func (t *WhisperGoTranscriber) Transcribe(audioData []float32) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.context == nil || t.model == nil {
		return "", fmt.Errorf("whisper context or model is nil")
	}

	if len(audioData) == 0 {
		return "", nil
	}

	logger.Info(logger.CategoryTranscription, "Transcribing %d audio samples", len(audioData))

	// Process audio
	if err := t.context.Process(audioData, nil); err != nil {
		return "", fmt.Errorf("whisper process failed: %w", err)
	}

	// Collect all segments into a single string
	var result string
	for _, segment := range t.context.Segments() {
		text := normalizeTranscriptionText(segment.Text)
		if text != "" {
			result += text + " "
		}
	}

	return result, nil
}

// SetStreamingCallback sets callback for streaming results
func (t *WhisperGoTranscriber) SetStreamingCallback(callback func(text string)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.streamCallback = callback
}

// SetRecordingState updates recording state
func (t *WhisperGoTranscriber) SetRecordingState(isRecording bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.recordingActive != isRecording {
		if isRecording {
			logger.Info(logger.CategoryTranscription, "Starting whisper transcription with Go bindings")
			// Clear buffer when starting recording
			t.buffer = t.buffer[:0]
		} else {
			logger.Info(logger.CategoryTranscription, "Stopping whisper transcription with Go bindings")
		}
	}

	t.recordingActive = isRecording
}

// UpdateConfig updates configuration
func (t *WhisperGoTranscriber) UpdateConfig(config Config) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Update the configuration
	prevLanguage := t.config.Language
	t.config = config

	// Update language if changed
	if t.context != nil && prevLanguage != config.Language && config.Language != "" && config.Language != "auto" {
		t.context.SetLanguage(config.Language)
	}

	return nil
}

// GetModelInfo returns model information
func (t *WhisperGoTranscriber) GetModelInfo() (ModelSize, string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.config.ModelSize, t.modelPath
}

// Close releases resources
func (t *WhisperGoTranscriber) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Close context and model to free resources
	if t.context != nil {
		logger.Info(logger.CategoryTranscription, "Closing whisper context")
		t.context.Free()
		t.context = nil
	}

	if t.model != nil {
		logger.Info(logger.CategoryTranscription, "Closing whisper model")
		t.model.Close()
		t.model = nil
	}

	return nil
}

// getLocalModelPath attempts to find a local model file
func getLocalModelPath(modelSize ModelSize) string {
	// Check common locations for models
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// Try standard location for whisper models
	modelName := fmt.Sprintf("ggml-%s.bin", modelSize)

	// Check common paths
	paths := []string{
		filepath.Join(homeDir, ".local", "share", "ramble", "models", modelName),
		filepath.Join(homeDir, ".whisper", "models", modelName),
		filepath.Join("models", modelName),
		filepath.Join("whisper.cpp", "models", modelName),
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}
