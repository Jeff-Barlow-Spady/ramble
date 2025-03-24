//go:build cgo && whisper_go
// +build cgo,whisper_go

// This file contains the implementation of the WhisperGoTranscriber using Go bindings
// It will only be compiled when both cgo and whisper_go tags are provided.

package transcription

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	model      whisper.Model
	context    whisper.Context
	buffer     []float32
	bufferTime time.Time

	// Keep track of transcription history for better context
	previousText   string
	fullTranscript string

	// Processing parameters
	processingRate int // samples per second (16000 Hz)
	minAudioLevel  float32
	minBufferLen   int // min samples before processing
}

// NewGoBindingTranscriber creates a transcriber using the Go bindings
func NewGoBindingTranscriber(config Config) (ConfigurableTranscriber, error) {
	// Find the model path
	modelPath := config.ModelPath
	if modelPath == "" {
		modelPath = getLocalModelPath(config.ModelSize)
		if modelPath == "" {
			return nil, fmt.Errorf("whisper model not found for size: %s", config.ModelSize)
		}
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

	// Set optimal parameters for continuous speech recognition with longer utterances
	context.SetMaxContext(65536)      // Even larger context window for extended rambling (doubled again)
	context.SetMaxTokensPerSegment(0) // No limit on tokens per segment
	context.SetSplitOnWord(true)      // Split on word boundaries - essential for paragraph formation

	// Set voice activity detection parameters for increased sensitivity
	context.SetTokenThreshold(0.01)    // Token probability threshold (equivalent to thold_pt in C params)
	context.SetTokenSumThreshold(0.01) // Token sum probability threshold (equivalent to thold_ptsum)

	// We don't have direct access to vad_thold in Go bindings, but we can
	// compensate with other parameters to make it more sensitive

	logger.Info(logger.CategoryTranscription, "Setting up improved streaming callback for extended speech")
	logger.Info(logger.CategoryTranscription, "Using model size: %s", config.ModelSize)

	// Set appropriate parameters
	transcriber := &WhisperGoTranscriber{
		config:         config,
		model:          model,
		context:        context,
		modelPath:      modelPath,
		processingRate: 16000, // Standard whisper rate
		minAudioLevel:  0.01,
		minBufferLen:   16000,                        // 1 second minimum
		buffer:         make([]float32, 0, 16000*45), // Pre-allocate 45 seconds for extended rambling
		bufferTime:     time.Now(),
		previousText:   "",
		fullTranscript: "",
	}

	logger.Info(logger.CategoryTranscription, "Whisper Go binding transcriber initialized successfully")
	return transcriber, nil
}

// getSegments gets segments from the whisper context after processing
func getSegments(ctx whisper.Context) []whisper.Segment {
	var segments []whisper.Segment
	// Keep getting segments until we hit EOF
	for {
		segment, err := ctx.NextSegment()
		if err != nil {
			break
		}
		segments = append(segments, segment)
	}
	return segments
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
	// 1. Buffer is long enough (at least 6 seconds @ 16kHz for rambling detection)
	// 2. It's been at least 2.5 seconds since last processing (further increased for better sentence completion)
	minBufferSamples := t.processingRate * 6 // 6 seconds of audio data
	if len(t.buffer) >= minBufferSamples && time.Since(t.bufferTime) >= 2500*time.Millisecond {
		t.bufferTime = time.Now()

		if t.config.Debug {
			logger.Debug(logger.CategoryTranscription, "Processing %d audio samples with Go bindings", len(t.buffer))
		}

		// Set optimal parameters for continuous transcription with longer utterances
		t.context.SetMaxContext(65536)      // Even larger context window for rambling
		t.context.SetMaxTokensPerSegment(0) // No limit on tokens per segment
		t.context.SetSplitOnWord(true)      // Split on word boundaries for paragraph handling

		// Set initial prompt with previous text to maintain context
		if t.previousText != "" {
			logger.Debug(logger.CategoryTranscription, "Setting initial prompt from previous text: '%s'", t.previousText)
			t.context.SetInitialPrompt(t.previousText)
		}

		// Process the audio buffer with proper callback parameters
		if err := t.context.Process(t.buffer, nil, nil, nil); err != nil {
			logger.Warning(logger.CategoryTranscription, "Error processing audio with Go bindings: %v", err)
			return "", nil
		}

		// Get the segments
		segments := getSegments(t.context)

		// If we got some text, send it via callback
		if len(segments) > 0 {
			logger.Debug(logger.CategoryTranscription, "Got %d segments from processing", len(segments))

			// Check if we have a callback registered
			if t.streamCallback != nil {
				// Combine all segments into a single text string with improved paragraph formation
				var combinedText string
				for _, segment := range segments {
					text := normalizeTranscriptionText(segment.Text)
					if text != "" {
						if combinedText != "" && !strings.HasSuffix(combinedText, " ") {
							combinedText += " "
						}
						combinedText += text
					}
					logger.Debug(logger.CategoryTranscription, "Segment text: '%s', Combined so far: '%s'",
						text, combinedText)
				}

				// Only send callback if we have text to send
				if combinedText != "" {
					logger.Info(logger.CategoryTranscription, "Sending combined text to callback: '%s'", combinedText)

					// Update full transcript and previous text for context
					if t.fullTranscript == "" {
						t.fullTranscript = combinedText
					} else if !strings.Contains(t.fullTranscript, combinedText) {
						// Only append text that's not already in the transcript
						t.fullTranscript += " " + combinedText
					}

					// Keep the most recent text for initial prompt in next processing
					t.previousText = combinedText

					t.streamCallback(combinedText)
				}
			} else {
				logger.Warning(logger.CategoryTranscription, "No callback registered to receive transcription results")
			}
		} else {
			logger.Debug(logger.CategoryTranscription, "No segments returned from processing")
		}

		// Keep more audio context (10 seconds) for better continuity with rambling
		if len(t.buffer) > t.processingRate*10 {
			t.buffer = t.buffer[len(t.buffer)-(t.processingRate*10):]
		} else {
			// Keep the buffer as is if it's already smaller than 10 seconds
		}
	}

	return "", nil
}

// Transcribe implements complete file transcription
func (t *WhisperGoTranscriber) Transcribe(audioData []float32) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(audioData) == 0 {
		return "", nil
	}

	logger.Info(logger.CategoryTranscription, "Transcribing %d audio samples", len(audioData))

	// Process audio with proper callback parameters
	if err := t.context.Process(audioData, nil, nil, nil); err != nil {
		return "", fmt.Errorf("whisper process failed: %w", err)
	}

	// Collect all segments into a single string
	var result string
	segments := getSegments(t.context)
	for _, segment := range segments {
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
			// Clear buffer and context when starting a new recording
			t.buffer = t.buffer[:0]
			t.previousText = ""
			t.fullTranscript = ""
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
	if prevLanguage != config.Language && config.Language != "" && config.Language != "auto" {
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
		// We don't call Free() directly as it's handled by model.Close()
	}

	if t.model != nil {
		logger.Info(logger.CategoryTranscription, "Closing whisper model")
		t.model.Close()
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

	// Define common model locations
	locations := []string{
		// Current directory
		filepath.Join(".", "models"),
		// Home directory
		filepath.Join(homeDir, ".local", "share", "ramble", "models"),
		filepath.Join(homeDir, ".ramble", "models"),
		// System locations
		"/usr/local/share/ramble/models",
		"/usr/share/ramble/models",
		// Whisper.cpp locations
		filepath.Join("whisper.cpp", "models"),
	}

	// Model filename pattern
	modelFileName := ""
	switch modelSize {
	case ModelTiny:
		modelFileName = "ggml-tiny.bin"
	case ModelBase:
		modelFileName = "ggml-base.bin"
	case ModelSmall:
		modelFileName = "ggml-small.bin"
	case ModelMedium:
		modelFileName = "ggml-medium.bin"
	case ModelLarge:
		modelFileName = "ggml-large.bin"
	}

	if modelFileName == "" {
		return ""
	}

	// Check each location
	for _, loc := range locations {
		path := filepath.Join(loc, modelFileName)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}
