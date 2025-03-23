//go:build cgo && whisper_cgo
// +build cgo,whisper_cgo

// Package transcription provides speech-to-text functionality
package transcription

// This CGO implementation provides direct access to the whisper.cpp library
// This is a more efficient and reliable approach than using subprocess-based
// solutions, especially for real-time streaming transcription.

/*
#cgo CFLAGS: -I${SRCDIR}/../../whisper.cpp/include
#cgo LDFLAGS: -L${SRCDIR}/../../whisper.cpp/build -lwhisper

#include <stdlib.h>
#include <whisper.h>

void whisper_log_callback(enum whisper_log_level level, const char * text, void * user_data) {
    // Called from C when whisper outputs a log message
}
*/
import "C"
import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/jeff-barlow-spady/ramble/pkg/logger"
)

// CGOWhisperTranscriber implements the Transcriber interface using CGO
// to directly interface with the whisper.cpp library
type CGOWhisperTranscriber struct {
	config           Config
	modelPath        string
	mu               sync.Mutex
	recordingActive  bool
	ctx              *C.struct_whisper_context
	params           C.struct_whisper_full_params
	streamingActive  bool
	streamingMu      sync.Mutex
	streamCallbackFn func(text string)
	frameCount       int
}

// NewCGOTranscriber creates a new transcriber that directly interfaces with the whisper.cpp library
func NewCGOTranscriber(config Config) (ConfigurableTranscriber, error) {
	// Ensure we have a valid model path
	if config.ModelPath == "" {
		modelPath, err := ensureModel(config)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize model: %w", err)
		}
		config.ModelPath = modelPath
	}

	// Create the whisper context by loading the model
	cModelPath := C.CString(config.ModelPath)
	defer C.free(unsafe.Pointer(cModelPath))

	logger.Info(logger.CategoryTranscription, "Loading whisper model from %s", config.ModelPath)

	// Initialize whisper context with the model
	ctx := C.whisper_init_from_file(cModelPath)
	if ctx == nil {
		return nil, fmt.Errorf("failed to initialize whisper context with model: %s", config.ModelPath)
	}

	// Set up default parameters
	params := C.whisper_full_default_params(C.WHISPER_SAMPLING_GREEDY)

	// Set language if specified
	if config.Language != "" && config.Language != "auto" {
		cLang := C.CString(config.Language)
		defer C.free(unsafe.Pointer(cLang))
		params.language = cLang
	}

	// Create the transcriber
	t := &CGOWhisperTranscriber{
		config:          config,
		modelPath:       config.ModelPath,
		ctx:             ctx,
		params:          params,
		streamingActive: false,
	}

	// Set up finalizer to ensure whisper context is freed when transcriber is garbage collected
	runtime.SetFinalizer(t, func(t *CGOWhisperTranscriber) {
		t.Close()
	})

	return t, nil
}

// ProcessAudioChunk processes a chunk of audio data using the whisper.cpp streaming API
func (t *CGOWhisperTranscriber) ProcessAudioChunk(audioData []float32) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Early exit if transcription is explicitly disabled
	if !t.recordingActive {
		return "", nil
	}

	// If not already in streaming mode, try to start it
	if !t.streamingActive {
		t.startStreaming()
	}

	// Skip empty audio data
	if len(audioData) == 0 {
		return "", nil
	}

	// Create a new C array for the audio data
	numSamples := len(audioData)
	cAudioData := (*C.float)(unsafe.Pointer(&audioData[0]))

	// Process the audio with whisper
	rc := C.whisper_full(t.ctx, t.params, cAudioData, C.int(numSamples))
	if rc != 0 {
		return "", fmt.Errorf("whisper_full() failed with error code %d", rc)
	}

	// Get the number of segments in the result
	numSegments := int(C.whisper_full_n_segments(t.ctx))
	if numSegments == 0 {
		return "", nil // No transcription yet
	}

	// Extract text from all segments
	var result string
	for i := 0; i < numSegments; i++ {
		segmentText := C.GoString(C.whisper_full_get_segment_text(t.ctx, C.int(i)))
		result += segmentText
	}

	// If we have a callback and got text, call it
	if t.streamCallbackFn != nil && result != "" {
		t.streamCallbackFn(result)
	}

	return result, nil
}

// startStreaming initializes the streaming mode for whisper
func (t *CGOWhisperTranscriber) startStreaming() error {
	t.streamingMu.Lock()
	defer t.streamingMu.Unlock()

	logger.Info(logger.CategoryTranscription, "Starting whisper.cpp streaming mode via CGO")

	// Mark streaming as active
	t.streamingActive = true
	t.frameCount = 0

	// Initialize the model for streaming
	// Set parameters specific to streaming
	t.params.single_segment = true    // Process one segment at a time
	t.params.print_progress = false   // Don't print progress
	t.params.print_realtime = false   // Don't print in real-time
	t.params.print_timestamps = false // Don't print timestamps
	t.params.translate = false        // Don't translate
	t.params.no_context = true        // Don't use context from previous segments
	t.params.max_tokens = 32          // Limit number of tokens per segment
	t.params.n_threads = 4            // Use multiple threads for processing

	return nil
}

// SetStreamingCallback sets a callback for receiving streaming transcription results
func (t *CGOWhisperTranscriber) SetStreamingCallback(callback func(text string)) {
	t.streamingMu.Lock()
	defer t.streamingMu.Unlock()
	t.streamCallbackFn = callback
}

// SetRecordingState updates the transcriber's internal state based on recording status
func (t *CGOWhisperTranscriber) SetRecordingState(isRecording bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.recordingActive = isRecording
}

// GetModelInfo returns information about the loaded model
func (t *CGOWhisperTranscriber) GetModelInfo() (ModelSize, string) {
	return t.config.ModelSize, t.modelPath
}

// UpdateConfig updates the transcriber's configuration
func (t *CGOWhisperTranscriber) UpdateConfig(config Config) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Check if we need to reload the model
	if config.ModelPath != t.config.ModelPath || config.ModelSize != t.config.ModelSize {
		// Close the old context
		if t.ctx != nil {
			C.whisper_free(t.ctx)
			t.ctx = nil
		}

		// Handle model changes
		if config.ModelPath == "" {
			var err error
			config.ModelPath, err = ensureModel(config)
			if err != nil {
				return fmt.Errorf("failed to update model: %w", err)
			}
		}

		// Load the new model
		cModelPath := C.CString(config.ModelPath)
		defer C.free(unsafe.Pointer(cModelPath))

		ctx := C.whisper_init_from_file(cModelPath)
		if ctx == nil {
			return fmt.Errorf("failed to initialize whisper context with model: %s", config.ModelPath)
		}

		t.ctx = ctx
		t.modelPath = config.ModelPath
	}

	// Update language if it changed
	if config.Language != t.config.Language {
		if config.Language != "" && config.Language != "auto" {
			cLang := C.CString(config.Language)
			defer C.free(unsafe.Pointer(cLang))
			t.params.language = cLang
		} else {
			t.params.language = nil
		}
	}

	// Update the config
	t.config = config
	return nil
}

// Close frees resources used by the transcriber
func (t *CGOWhisperTranscriber) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Free the whisper context
	if t.ctx != nil {
		C.whisper_free(t.ctx)
		t.ctx = nil
	}

	return nil
}

// Transcribe processes a complete audio sample and returns the transcription
func (t *CGOWhisperTranscriber) Transcribe(audioData []float32) (string, error) {
	// Just delegate to ProcessAudioChunk as it handles both cases
	return t.ProcessAudioChunk(audioData)
}
