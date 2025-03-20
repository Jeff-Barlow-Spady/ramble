// Package transcription provides speech-to-text functionality
package transcription

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jeff-barlow-spady/ramble/pkg/logger"
	"github.com/jeff-barlow-spady/ramble/pkg/transcription/embed"
)

// ExecutableType represents the type of whisper executable
type ExecutableType int

const (
	ExecutableTypeUnknown ExecutableType = iota
	ExecutableTypeWhisperCpp
	ExecutableTypeWhisperGael
	ExecutableTypePython
	ExecutableTypeOpenAI
	// Add other types as needed
)

// detectExecutableType determines the type of whisper executable
func detectExecutableType(execPath string) ExecutableType {
	// Get the executable name without path
	execName := filepath.Base(execPath)

	// Check if we can determine the type from the name
	switch {
	case strings.Contains(execName, "whisper-cpp") ||
		strings.Contains(execName, "main") ||
		strings.Contains(execName, "whisper.exe") && !strings.Contains(execName, "gael"):
		return ExecutableTypeWhisperCpp

	case strings.Contains(execName, "whisper-gael") ||
		strings.Contains(execName, "whisper.py"):
		return ExecutableTypeWhisperGael

	case strings.HasSuffix(execName, ".py"):
		return ExecutableTypePython
	}

	// If we can't determine from name, try to run with --help and look at output
	cmd := exec.Command(execPath, "--help")
	output, err := cmd.CombinedOutput()
	if err == nil {
		outputStr := strings.ToLower(string(output))

		// Check output signatures
		switch {
		case strings.Contains(outputStr, "whisper.cpp"):
			return ExecutableTypeWhisperCpp

		case strings.Contains(outputStr, "gael"):
			return ExecutableTypeWhisperGael

		case strings.Contains(outputStr, "openai") || strings.Contains(outputStr, "whisper-cli"):
			return ExecutableTypeOpenAI

		case strings.Contains(outputStr, "python") || strings.Contains(outputStr, "pytorch"):
			return ExecutableTypePython
		}
	}

	// Default - try to use whisper.cpp style which is most common
	logger.Info(logger.CategoryTranscription, "Could not determine executable type for %s, defaulting to whisper.cpp style", execPath)
	return ExecutableTypeWhisperCpp
}

// ExecutableTranscriber uses an external whisper executable for transcription
type ExecutableTranscriber struct {
	config         Config
	executablePath string
	modelPath      string
	isRunning      bool
	cmd            *exec.Cmd
	stopChan       chan struct{}
	tempWavFile    string
	audioBuffer    []float32
	lastProcessed  time.Time
	mu             sync.Mutex
	// Callback to deliver streaming results as they come in
	streamingCallback func(text string)
}

// NewExecutableTranscriber creates a new transcriber that uses an external whisper executable
func NewExecutableTranscriber(config Config) (ConfigurableTranscriber, error) {
	if config.ExecutablePath == "" {
		return nil, fmt.Errorf("%w: no executable path provided", ErrInvalidExecutablePath)
	}

	if _, err := os.Stat(config.ExecutablePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidExecutablePath, config.ExecutablePath)
	}

	// Check that the model file exists
	if config.ModelPath == "" {
		return nil, fmt.Errorf("%w: no model path provided", ErrModelNotFound)
	}

	if _, err := os.Stat(config.ModelPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("%w: %s", ErrModelNotFound, config.ModelPath)
	}

	// Create the transcriber
	t := &ExecutableTranscriber{
		config:         config,
		executablePath: config.ExecutablePath,
		modelPath:      config.ModelPath,
		isRunning:      false,
		stopChan:       make(chan struct{}),
		audioBuffer:    make([]float32, 0),
		lastProcessed:  time.Now(),
	}

	// Test that the transcriber works
	if err := testTranscriber(t); err != nil {
		t.Close() // Clean up resources
		return nil, fmt.Errorf("%w: %v", ErrTranscriptionFailed, err)
	}

	return t, nil
}

// UpdateConfig updates the transcriber's configuration
func (t *ExecutableTranscriber) UpdateConfig(config Config) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.config = config
	// Update model path based on new configuration
	t.modelPath = getModelPath(config.ModelPath, config.ModelSize)

	return nil
}

// GetModelInfo returns information about the current model
func (t *ExecutableTranscriber) GetModelInfo() (ModelSize, string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.config.ModelSize, t.modelPath
}

// Transcribe converts audio data to text
func (t *ExecutableTranscriber) Transcribe(audioData []float32) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.config.Debug {
		logger.Debug(logger.CategoryTranscription, "Transcribing %d audio samples with executable", len(audioData))
	}

	// Save the original buffer
	originalBuffer := t.audioBuffer

	// Set the buffer to just the incoming audio data
	t.audioBuffer = audioData

	// Process the audio
	result, err := t.processAudioBuffer()

	// Restore the original buffer
	t.audioBuffer = originalBuffer

	return result, err
}

// Close frees resources
func (t *ExecutableTranscriber) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Stop any running processes
	close(t.stopChan)

	// Clear audio buffer
	t.audioBuffer = nil

	// Clean up temp files
	if t.tempWavFile != "" {
		os.Remove(t.tempWavFile)
		t.tempWavFile = ""
	}

	// Call cleanup for embedded files
	embed.CleanupTempFiles()

	return nil
}

// SetStreamingCallback sets a callback to receive streaming transcription results
func (t *ExecutableTranscriber) SetStreamingCallback(callback func(text string)) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.streamingCallback = callback
}

// ProcessAudioChunk processes a chunk of audio data and returns the transcription
func (t *ExecutableTranscriber) ProcessAudioChunk(audioData []float32) (string, error) {
	// Create a copy of the audio data to avoid race conditions
	processedData := make([]float32, len(audioData))
	copy(processedData, audioData)

	// Run the actual processing in a separate goroutine to avoid blocking the UI thread
	go func() {
		t.mu.Lock()

		// Apply any DSP processing if needed
		filteredData := processDspFilters(processedData)

		// Add the new data to our buffer
		t.audioBuffer = append(t.audioBuffer, filteredData...)

		// Calculate and log the buffer size and audio level
		bufferSeconds := float64(len(t.audioBuffer)) / 16000.0
		bufferRMS := calculateRMSLevel(t.audioBuffer)

		// Skip the rest if we're logging at debug level
		if t.config.Debug {
			logger.Info(logger.CategoryTranscription, "Processing audio buffer of %d samples (%.2f seconds)",
				len(t.audioBuffer), bufferSeconds)
			logger.Info(logger.CategoryTranscription, "Audio buffer RMS level: %f", bufferRMS)
		}

		// Set minimum threshold for processing - no need to process silence
		const minRMSThreshold = 0.008 // Lower threshold to pick up more speech
		if bufferRMS < minRMSThreshold && bufferSeconds < 1.0 {
			t.mu.Unlock()
			return
		}

		timeSinceLastProcess := time.Since(t.lastProcessed).Seconds()

		// STREAMING MODE: Process much more frequently with smaller chunks
		// This creates more of a streaming effect even though Whisper isn't truly real-time
		shouldProcess := (bufferSeconds >= 0.8 && timeSinceLastProcess >= 0.5) || // Process very frequently
			(bufferSeconds >= 3.0) || // Safety cap on buffer size
			(len(t.audioBuffer) > 0 && timeSinceLastProcess > 2.0) // Process if it's been a while

		if !shouldProcess {
			t.mu.Unlock()
			return
		}

		// Process the audio - using a local copy to avoid modifying the buffer if processing fails
		bufferCopy := make([]float32, len(t.audioBuffer))
		copy(bufferCopy, t.audioBuffer)

		// Keep less context (0.3 seconds) to reduce latency between chunks
		// This helps with more immediate updates in streaming mode
		contextSamples := int(0.3 * 16000)
		if len(t.audioBuffer) > contextSamples {
			t.audioBuffer = t.audioBuffer[len(t.audioBuffer)-contextSamples:]
		}
		t.lastProcessed = time.Now()

		// We're done with the mutex-protected parts
		t.mu.Unlock()

		// Now process the copy without holding the mutex
		// This allows other audio to be collected while we're processing
		_, err := t.processAudioBuffer()
		if err != nil {
			logger.Error(logger.CategoryTranscription, "Error processing audio buffer: %v", err)
		}
	}()

	// Always return immediately to avoid blocking the UI
	return "", nil
}

// calculateRMSLevel calculates the Root Mean Square of the audio buffer
// which gives a good approximation of perceived volume level
func calculateRMSLevel(buffer []float32) float32 {
	if len(buffer) == 0 {
		return 0
	}

	// Calculate sum of squares
	var sumSquares float64
	for _, sample := range buffer {
		sumSquares += float64(sample * sample)
	}

	// Calculate RMS
	rms := math.Sqrt(sumSquares / float64(len(buffer)))
	return float32(rms)
}

// processAudioBuffer processes the current audio buffer
func (t *ExecutableTranscriber) processAudioBuffer() (string, error) {
	// Save to WAV file
	wavFilePath := filepath.Join(os.TempDir(), fmt.Sprintf("audio_%d.wav", time.Now().UnixNano()))
	if err := saveAudioToWav(t.audioBuffer, wavFilePath); err != nil {
		return "", fmt.Errorf("%w: failed to save audio data: %v", ErrTranscriptionFailed, err)
	}
	t.tempWavFile = wavFilePath

	// Process with whisper executable
	text, err := t.processAudioWithExecutable(wavFilePath)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTranscriptionFailed, err)
	}

	// Update last processed time
	t.lastProcessed = time.Now()

	// Clean up file if not in debug mode
	if !t.config.Debug {
		os.Remove(wavFilePath)
		t.tempWavFile = ""
	}

	return text, nil
}

// processAudioWithExecutable processes audio with an external executable
func (t *ExecutableTranscriber) processAudioWithExecutable(wavFile string) (string, error) {
	// Check if the WAV file exists
	if _, err := os.Stat(wavFile); os.IsNotExist(err) {
		return "", fmt.Errorf("%w: wav file does not exist: %v", ErrTranscriptionFailed, err)
	}

	// Detect the executable type
	execType := detectExecutableType(t.executablePath)

	// Get optimized arguments for the specific executable type
	args := getStreamingArgs(execType, t.modelPath, t.config.Language, wavFile)

	// Log the command in debug mode
	if t.config.Debug {
		logger.Debug(logger.CategoryTranscription, "Executing: %s %v", t.executablePath, args)
	}

	// Create the command
	cmd := exec.Command(t.executablePath, args...)

	// Set environment variables for better performance
	env := os.Environ()
	// OpenMP thread control for better responsiveness
	env = append(env, "OMP_NUM_THREADS=4")
	// Whisper.cpp thread count limit
	env = append(env, "WHISPER_THREAD_COUNT=4")
	cmd.Env = env

	// Create pipes for real-time output processing
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("%w: failed to create stdout pipe: %v", ErrTranscriptionFailed, err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("%w: failed to create stderr pipe: %v", ErrTranscriptionFailed, err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("%w: failed to start process: %v", ErrTranscriptionFailed, err)
	}

	// Signal that processing has started
	if t.streamingCallback != nil {
		t.streamingCallback("Processing audio...")
	}

	// Calculate appropriate timeout based on audio length
	timeout := calculateTimeout(len(t.audioBuffer))
	timeoutChan := time.After(timeout)

	// Process results
	resultChan := make(chan string, 1)
	errChan := make(chan error, 1)
	doneChan := make(chan struct{})

	// Process stdout in goroutine for real-time feedback
	go func() {
		defer close(doneChan)
		scanner := bufio.NewScanner(stdout)
		var result strings.Builder

		for scanner.Scan() {
			line := scanner.Text()

			// Skip empty lines and progress info
			if isProcessingLine(line) {
				continue
			}

			// Clean up the line based on executable type
			cleanedLine := cleanOutputLine(line, execType)
			if cleanedLine == "" {
				continue
			}

			// Add to result
			if result.Len() > 0 {
				result.WriteString(" ")
			}
			result.WriteString(cleanedLine)

			// Send interim results for streaming feedback
			if t.streamingCallback != nil {
				t.streamingCallback(cleanedLine)
			}
		}

		if err := scanner.Err(); err != nil {
			errChan <- fmt.Errorf("error reading output: %v", err)
			return
		}

		// Send the final result
		resultChan <- result.String()
	}()

	// Process stderr for debugging (non-blocking)
	go func() {
		stderrBuffer := new(strings.Builder)
		io.Copy(stderrBuffer, stderr)

		if stderrOutput := stderrBuffer.String(); stderrOutput != "" && t.config.Debug {
			logger.Debug(logger.CategoryTranscription, "STDERR: %s", stderrOutput)
		}
	}()

	// Wait for process completion or timeout
	var finalResult string
	select {
	case <-doneChan:
		// Process completed normally
		if err := cmd.Wait(); err != nil {
			// Check if we have partial results anyway
			select {
			case res := <-resultChan:
				if res != "" {
					// We got something useful despite the error
					finalResult = res
					logger.Warning(logger.CategoryTranscription,
						"Process exited with error but returned partial results: %v", err)
				} else {
					return "", fmt.Errorf("%w: process exited with error: %v", ErrTranscriptionFailed, err)
				}
			default:
				return "", fmt.Errorf("%w: process exited with error: %v", ErrTranscriptionFailed, err)
			}
		} else {
			// Get the result
			select {
			case res := <-resultChan:
				finalResult = res
			case <-time.After(100 * time.Millisecond): // Small timeout to avoid deadlock
				// No result but process exited normally - likely silence
				logger.Info(logger.CategoryTranscription, "No result from process, likely silence")
				finalResult = ""
			}
		}

	case err := <-errChan:
		// Error during output processing
		cmd.Process.Kill() // Ensure process is terminated
		return "", fmt.Errorf("%w: %v", ErrTranscriptionFailed, err)

	case <-timeoutChan:
		// Process timed out - kill it
		cmd.Process.Kill()
		logger.Warning(logger.CategoryTranscription, "Transcription timed out after %v", timeout)

		// Try to get any partial result that might be available
		select {
		case res := <-resultChan:
			// We got partial results before timeout
			finalResult = res
		case <-time.After(100 * time.Millisecond):
			// No partial results
			if t.streamingCallback != nil {
				t.streamingCallback("Transcription timed out - try again")
			}
			return "", fmt.Errorf("%w: transcription timed out", ErrTranscriptionFailed)
		}
	}

	// Signal completion
	if t.streamingCallback != nil {
		if finalResult == "" {
			t.streamingCallback("No speech detected")
		} else {
			t.streamingCallback("Processing complete")
		}
	}

	// Clean and normalize the final result
	finalResult = normalizeTranscriptionText(finalResult)
	return finalResult, nil
}

// getStreamingArgs returns optimized command arguments for streaming based on executable type
func getStreamingArgs(execType ExecutableType, modelPath, language, inputFile string) []string {
	switch execType {
	case ExecutableTypeWhisperCpp:
		// Whisper.cpp optimized for fast streaming response
		args := []string{
			"-m", modelPath,
			"-f", inputFile,
			"-otxt",   // Output to text
			"-nt",     // No timestamps for cleaner output
			"-t", "4", // Use 4 threads for better performance
			"-ml", "1", // Minimal segment length for faster output
			"-su",      // Speed up (2x) audio processing
			"-bo", "1", // Best of 1 for fastest decoding
			"-nf",  // No fallbacks for faster processing
			"-sow", // Split on word boundaries
		}

		// Add language if specified
		if language != "" {
			args = append(args, "-l", language)
		}

		return args

	case ExecutableTypeWhisperGael:
		// Python-based Whisper-Gael optimized for streaming
		args := []string{
			"--model", modelPath,
			"--input", inputFile,
			"--output_format", "txt",
			"--no_timestamps",
			"--threads", "4",
			"--faster",         // Enable faster processing
			"--beam_size", "1", // Smaller beam size for faster results
		}

		// Add language if specified
		if language != "" {
			args = append(args, "--language", language)
		}

		return args

	case ExecutableTypePython:
		// Generic Python Whisper with streaming optimizations
		args := []string{
			"--model", modelPath,
			"--input", inputFile,
			"--output_format", "txt",
			"--task", "transcribe",
			"--best_of", "1", // Fast processing, no beam search
			"--temperature", "0", // No temperature sampling for deterministic results
		}

		// Add language if specified
		if language != "" {
			args = append(args, "--language", language)
		}

		return args

	case ExecutableTypeOpenAI:
		// OpenAI's CLI
		args := []string{
			"--model", filepath.Base(modelPath),
			"--output_format", "txt",
			"--task", "transcribe",
			inputFile,
		}

		// Add language if specified
		if language != "" {
			args = append(args, "--language", language)
		}

		return args

	default:
		// Default to whisper.cpp style
		return getStreamingArgs(ExecutableTypeWhisperCpp, modelPath, language, inputFile)
	}
}

// isProcessingLine identifies lines that are just progress information
func isProcessingLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return true
	}

	return strings.HasPrefix(trimmed, "[") || // Whisper.cpp progress info
		strings.HasPrefix(trimmed, "whisper_") || // Internal debug logs
		strings.HasPrefix(trimmed, "system_info:") || // System information
		strings.Contains(trimmed, "progress") // Progress updates
}

// cleanOutputLine cleans a line of output based on executable type
func cleanOutputLine(line string, execType ExecutableType) string {
	line = strings.TrimSpace(line)

	// Skip empty lines
	if line == "" {
		return ""
	}

	// Remove various artifacts based on the executable type
	switch execType {
	case ExecutableTypeWhisperCpp:
		// Remove timestamp markers like [00:00.000 --> 00:00.500]
		timestampRegex := regexp.MustCompile(`\[\d{2}:\d{2}:\d{2}\.\d{3}\s*-->\s*\d{2}:\d{2}:\d{2}\.\d{3}\]`)
		line = timestampRegex.ReplaceAllString(line, "")

		// Skip lines with only timestamps
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			return ""
		}

	case ExecutableTypeWhisperGael, ExecutableTypePython, ExecutableTypeOpenAI:
		// Remove JSON markers
		line = strings.Trim(line, "\"[]{}():,")
	}

	// Remove noise markers common to all types
	noiseRegex := regexp.MustCompile(`\[(?i)(?:MUSIC|APPLAUSE|LAUGHTER|INAUDIBLE|NOISE|CROSSTALK)\]`)
	line = noiseRegex.ReplaceAllString(line, "")

	return strings.TrimSpace(line)
}

// calculateTimeout returns an appropriate timeout duration based on audio length
func calculateTimeout(sampleCount int) time.Duration {
	// Calculate audio length in seconds (assuming 16kHz sample rate)
	audioLengthSec := float64(sampleCount) / 16000.0

	// Base timeout: 2x audio length + 2 seconds, minimum 3 seconds
	timeout := (audioLengthSec * 2.0) + 2.0
	if timeout < 3.0 {
		timeout = 3.0
	}

	// Cap at 15 seconds for streaming use case
	if timeout > 15.0 {
		timeout = 15.0
	}

	return time.Duration(timeout * float64(time.Second))
}

// normalizeTranscriptionText cleans up transcription text for better quality
func normalizeTranscriptionText(text string) string {
	if text == "" {
		return ""
	}

	// Trim and normalize whitespace
	text = strings.TrimSpace(text)

	// Remove any repeated parenthetical content (often noise markers)
	parenPattern := regexp.MustCompile(`\([^)]*(?i)(music|noise|applause|laughter)[^)]*\)`)
	text = parenPattern.ReplaceAllString(text, "")

	// Remove any bracketed noise markers
	bracketPattern := regexp.MustCompile(`\[(?i)(?:MUSIC|APPLAUSE|LAUGHTER|INAUDIBLE|NOISE|CROSSTALK|SILENCE)\]`)
	text = bracketPattern.ReplaceAllString(text, "")

	// Replace multiple spaces with a single space
	spacePattern := regexp.MustCompile(`\s+`)
	text = spacePattern.ReplaceAllString(text, " ")

	// Fix common punctuation issues
	text = strings.ReplaceAll(text, " .", ".")
	text = strings.ReplaceAll(text, " ,", ",")
	text = strings.ReplaceAll(text, " ?", "?")
	text = strings.ReplaceAll(text, " !", "!")

	// Only capitalize first letter of sentences if we have text
	if len(text) > 0 {
		// Make the first character uppercase
		text = strings.ToUpper(text[:1]) + text[1:]
	}

	return strings.TrimSpace(text)
}
