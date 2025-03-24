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
	"strings"
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

	// Keep track of previous transcription
	previousText   string
	fullTranscript string
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

// ProcessAudioChunk processes a chunk of audio data and returns transcribed text
func (t *CGOWhisperTranscriber) ProcessAudioChunk(audioData []float32) (string, error) {
	// Skip processing if no data
	if len(audioData) == 0 {
		return "", nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Skip if not recording
	if !t.recordingActive {
		return "", nil
	}

	// Start streaming if not already active
	if !t.streamingActive {
		if err := t.startStreaming(); err != nil {
			return "", err
		}
	}

	// Convert the float32 audio data to int16
	pcm16 := make([]int16, len(audioData))
	for i, sample := range audioData {
		pcm16[i] = int16(sample * 32767)
	}

	// Make a C array from Go slice
	pcmData := (*C.short)(unsafe.Pointer(&pcm16[0]))
	pcmLen := C.int(len(pcm16))

	// If we have previous text, use it as initial prompt
	if t.previousText != "" {
		cPrompt := C.CString(t.previousText)
		defer C.free(unsafe.Pointer(cPrompt))
		t.params.initial_prompt = cPrompt
	} else {
		t.params.initial_prompt = nil
	}

	// Process the audio
	if ret := C.whisper_full(t.ctx, t.params, pcmData, pcmLen); ret != 0 {
		return "", fmt.Errorf("failed to process audio with whisper: error code %d", ret)
	}

	// Get the number of segments
	nSegments := int(C.whisper_full_n_segments(t.ctx))
	if nSegments == 0 {
		return "", nil
	}

	// Combine all segments into a single string
	var transcription strings.Builder
	for i := 0; i < nSegments; i++ {
		segment := C.GoString(C.whisper_full_get_segment_text(t.ctx, C.int(i)))
		segment = normalizeTranscriptionText(segment)
		if segment != "" {
			if transcription.Len() > 0 {
				transcription.WriteString(" ")
			}
			transcription.WriteString(segment)
		}
	}

	result := transcription.String()

	// Update the full transcript and previous text
	if t.fullTranscript == "" {
		t.fullTranscript = result
	} else if !strings.Contains(t.fullTranscript, result) {
		// Only append text that's not already in the transcript
		t.fullTranscript += " " + result
	}

	// Keep previous text for context in future calls
	t.previousText = result

	// If we have a callback, send the result
	if t.streamCallbackFn != nil && result != "" {
		logger.Info(logger.CategoryTranscription, "Got streaming text: %s", result)
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
	t.params.single_segment = false   // Process complete segments, not just one at a time
	t.params.print_progress = false   // Don't print progress
	t.params.print_realtime = false   // Don't print in real-time
	t.params.print_timestamps = false // Don't print timestamps
	t.params.translate = false        // Don't translate
	t.params.no_context = false       // USE context from previous segments (equivalent to --keep-context/-kc flag)
	t.params.max_tokens = 0           // No limit on number of tokens per segment
	t.params.n_threads = 4            // Use multiple threads for processing

	// Set voice activity detection parameters
	t.params.vad_thold = 0.3      // More sensitive VAD threshold (default in stream.cpp is 0.6)
	t.params.thold_pt = 0.01      // Token probability threshold
	t.params.thold_ptsum = 0.01   // Token sum probability threshold
	t.params.max_len = 0          // No maximum length restriction
	t.params.split_on_word = true // Split on word boundaries
	t.params.audio_ctx = 3000     // Use larger audio context (doubled from 1500)

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

	// Reset context when switching states
	if t.recordingActive != isRecording {
		if isRecording {
			logger.Info(logger.CategoryTranscription, "Starting whisper.cpp CGO transcription")
			t.previousText = ""
			t.fullTranscript = ""
		} else {
			logger.Info(logger.CategoryTranscription, "Stopping whisper.cpp CGO transcription")
		}
	}

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
