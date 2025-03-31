//go:build cgo && whisper_go
// +build cgo,whisper_go

// Package transcription provides speech-to-text functionality
package transcription

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
	"github.com/jeff-barlow-spady/ramble/pkg/logger"
)

// WhisperTranscriber implements direct access to whisper.cpp Go bindings with proper buffer management
type WhisperTranscriber struct {
	model              whisper.Model
	context            whisper.Context
	buffer             []float32
	minSamples         int // Minimum samples needed (16000 = 1 second at 16kHz)
	recordingActive    bool
	textCallback       func(string)
	mu                 sync.Mutex
	lastProcessTime    time.Time
	processingActive   bool
	lastText           string        // Store the last text segment to avoid duplicates
	recentSegments     []string      // Store several recent segments for better deduplication
	maxSegments        int           // Maximum number of segments to remember
	processingInterval time.Duration // Time between processing cycles
}

// NewManager creates a new whisper transcriber
func NewManager(modelPath string) (*WhisperTranscriber, error) {
	// Load the whisper model
	model, err := whisper.New(modelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load whisper model: %w", err)
	}

	// Create the whisper context
	context, err := model.NewContext()
	if err != nil {
		model.Close()
		return nil, fmt.Errorf("failed to create whisper context: %w", err)
	}

	return &WhisperTranscriber{
		model:              model,
		context:            context,
		buffer:             make([]float32, 0, 16000*5), // Pre-allocate 5 seconds
		minSamples:         16000,                       // 1 second minimum (16kHz)
		textCallback:       nil,
		lastProcessTime:    time.Now(),
		recentSegments:     make([]string, 0, 10),
		maxSegments:        10,                      // Remember last 10 segments for deduplication
		processingInterval: 1200 * time.Millisecond, // Process every 1.2 seconds instead of 500ms
	}, nil
}

// ProcessAudioChunk processes a chunk of audio data
func (t *WhisperTranscriber) ProcessAudioChunk(audioData []float32) (string, error) {
	if len(audioData) == 0 {
		return "", nil
	}

	t.mu.Lock()

	// Exit early if not recording
	if !t.recordingActive {
		t.mu.Unlock()
		return "", nil
	}

	// Add new audio to buffer
	t.buffer = append(t.buffer, audioData...)

	// Check if we should process now, if not exit early
	shouldProcess := !t.processingActive &&
		time.Since(t.lastProcessTime) >= t.processingInterval &&
		len(t.buffer) >= t.minSamples

	if !shouldProcess {
		t.mu.Unlock()
		return "", nil
	}

	// We will be processing, so mark as active and update timestamp
	t.processingActive = true
	t.lastProcessTime = time.Now()

	// Make a copy of just the part of the buffer we need to process
	// This is more memory efficient than copying the entire buffer
	// Limit to ~10 seconds maximum to reduce CPU load on long recordings
	maxProcessingSamples := 16000 * 10 // 10 seconds
	processLen := len(t.buffer)
	if processLen > maxProcessingSamples {
		processLen = maxProcessingSamples
	}

	bufferToProcess := make([]float32, processLen)
	// Copy from the most recent part of the buffer
	copy(bufferToProcess, t.buffer[len(t.buffer)-processLen:])

	t.mu.Unlock() // Release lock before starting async processing

	// Process the audio buffer in a goroutine to avoid blocking
	go func() {
		// Define segment callback to receive transcription results
		segmentCallback := func(segment whisper.Segment) {
			if segment.Text == "" {
				return
			}

			// Process text outside the lock when possible
			text := strings.TrimSpace(segment.Text)

			// Skip empty text or very short segments
			if text == "" || len(text) < 3 {
				return
			}

			// Now lock to check against recent segments and update state
			t.mu.Lock()
			defer t.mu.Unlock()

			// Skip if no callback or not recording anymore
			if t.textCallback == nil || !t.recordingActive {
				return
			}

			// More sophisticated deduplication logic
			shouldSend := true

			// Convert to lowercase for better matching
			textLower := strings.ToLower(text)

			// Check against recent segments for duplicates or significant overlaps
			for _, prevSegment := range t.recentSegments {
				prevLower := strings.ToLower(prevSegment)

				// More aggressive similarity threshold (0.6 vs 0.7)
				if textLower == prevLower || similarityScore(textLower, prevLower) > 0.6 {
					shouldSend = false
					break
				}

				// Skip if this segment is mostly contained in a previous segment
				// More aggressive containment check (70% vs 75%)
				if containsSubstantialOverlap(prevLower, textLower, 0.7) {
					shouldSend = false
					break
				}
			}

			if shouldSend {
				// Add to recent segments before sending
				t.recentSegments = append(t.recentSegments, text)

				// Trim if exceeded max size
				if len(t.recentSegments) > t.maxSegments {
					t.recentSegments = t.recentSegments[1:]
				}

				// Log the segment for debugging
				logger.Debug(logger.CategoryTranscription, "Sending segment: %s", text)

				// Send text to UI
				t.textCallback(text)
			} else {
				logger.Debug(logger.CategoryTranscription, "Skipping duplicate segment: %s", text)
			}
		}

		// Process the audio buffer
		err := t.context.Process(
			bufferToProcess,
			nil,             // No encoder begin callback needed
			segmentCallback, // Handle text segments
			nil,             // No progress callback needed
		)

		t.mu.Lock()
		defer t.mu.Unlock()

		// Mark that we're done processing
		t.processingActive = false

		if err != nil {
			logger.Warning(logger.CategoryTranscription,
				"Error processing audio: %v", err)
			return
		}

		// Keep a sliding window of audio for context
		// 15 seconds maximum instead of 30 to reduce memory usage
		const maxBufferSeconds = 15
		maxBufferLen := 16000 * maxBufferSeconds
		if len(t.buffer) > maxBufferLen {
			t.buffer = t.buffer[len(t.buffer)-maxBufferLen:]
		}
	}()

	return "", nil // Results are sent via callback
}

// similarityScore calculates how similar two strings are (0-1 scale)
// Uses a simple word overlap approach for efficiency
func similarityScore(a, b string) float64 {
	// Split into words
	wordsA := strings.Fields(a)
	wordsB := strings.Fields(b)

	// If either text has very few words, require higher match threshold
	if len(wordsA) <= 3 || len(wordsB) <= 3 {
		return 0.4 // Return a lower similarity for very short phrases
	}

	// Count matching words
	matches := 0

	// Create map of words in B for faster lookup
	wordMapB := make(map[string]int)
	for _, word := range wordsB {
		wordMapB[word]++
	}

	// Count words from A that appear in B
	for _, word := range wordsA {
		if count, exists := wordMapB[word]; exists && count > 0 {
			matches++
			wordMapB[word]--
		}
	}

	// Calculate similarity based on relative match percentage
	totalWords := len(wordsA)
	if totalWords == 0 {
		return 0
	}

	return float64(matches) / float64(totalWords)
}

// containsSubstantialOverlap checks if string a contains a substantial part of string b
// Uses a threshold parameter to adjust sensitivity
func containsSubstantialOverlap(a, b string, threshold float64) bool {
	// If one string is much longer than the other, they're probably different
	if len(a) > 2*len(b) || len(b) > 2*len(a) {
		return false
	}

	// Split into words
	wordsA := strings.Fields(a)
	wordsB := strings.Fields(b)

	// If B is very short, be more conservative to avoid losing content
	if len(wordsB) < 4 {
		return false
	}

	// Count words from B that appear in A
	matches := 0

	// Create map of words in A for faster lookup
	wordMapA := make(map[string]int)
	for _, word := range wordsA {
		wordMapA[word]++
	}

	// Count words from B that appear in A
	for _, word := range wordsB {
		if count, exists := wordMapA[word]; exists && count > 0 {
			matches++
			wordMapA[word]--
		}
	}

	// Return true if overlap exceeds threshold
	return float64(matches)/float64(len(wordsB)) > threshold
}

// SetStreamingCallback sets the function to call with transcription results
func (t *WhisperTranscriber) SetStreamingCallback(callback func(string)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.textCallback = callback
}

// SetRecordingState updates internal state for recording
func (t *WhisperTranscriber) SetRecordingState(isRecording bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.recordingActive = isRecording

	if isRecording {
		// Clear buffer and set up for new recording
		t.buffer = t.buffer[:0]
		t.processingActive = false
		t.lastText = ""
		t.recentSegments = t.recentSegments[:0] // Clear segment history

		// Configure the context with optimal settings
		t.configureContext()

		logger.Info(logger.CategoryTranscription, "Starting whisper transcription")
	} else {
		// Clear buffer when stopping
		t.buffer = t.buffer[:0]
		t.processingActive = false
		t.lastText = ""
		t.recentSegments = t.recentSegments[:0] // Clear segment history

		logger.Info(logger.CategoryTranscription, "Stopping whisper transcription")
	}
}

// configureContext sets optimal parameters for streaming transcription
func (t *WhisperTranscriber) configureContext() {
	// Basic configuration - language, performance settings
	_ = t.context.SetLanguage("en")

	// Lower thread count to reduce CPU usage (adjust based on your CPU)
	t.context.SetThreads(4) // Reduced from 8 to 4

	t.context.SetSplitOnWord(true)

	// Configure to reduce CPU usage
	t.context.SetMaxSegmentLength(0)   // Don't artificially limit segments
	t.context.SetTokenTimestamps(true) // Enable timestamps for words
}

// Close releases resources
func (t *WhisperTranscriber) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.model != nil {
		t.model.Close()
		t.model = nil
	}
	t.context = nil
	t.buffer = nil

	return nil
}
