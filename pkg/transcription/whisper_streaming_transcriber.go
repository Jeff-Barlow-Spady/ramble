package transcription

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jeff-barlow-spady/ramble/pkg/audio"
	"github.com/jeff-barlow-spady/ramble/pkg/logger"
	"github.com/jeff-barlow-spady/ramble/pkg/transcription/embed"
)

// ExecutableType represents the type of whisper executable (different implementations)
type ExecutableType int

const (
	// ExecutableTypeUnknown represents an unknown executable type
	ExecutableTypeUnknown ExecutableType = iota
	// ExecutableTypeWhisperCpp represents the C++ implementation of whisper
	ExecutableTypeWhisperCpp
	// ExecutableTypeWhisperGael represents the Python implementation by Gael
	ExecutableTypeWhisperGael
	// ExecutableTypeWhisperPython represents a generic Python implementation
	ExecutableTypeWhisperPython
	// ExecutableTypeWhisperCppStream represents the stream example from whisper.cpp
	// which explicitly supports stdin streaming
	ExecutableTypeWhisperCppStream
)

// detectExecutableType attempts to determine the type of whisper executable
func detectExecutableType(execPath string) ExecutableType {
	// Get the executable name from the path
	execName := filepath.Base(execPath)

	// Check if it's the embedded executable
	embeddedExePath, _ := embed.GetWhisperExecutable()
	if execPath == embeddedExePath {
		embedType := embed.GetEmbeddedExecutableType()
		switch embedType {
		case embed.WhisperCpp:
			return ExecutableTypeWhisperCpp
		case embed.WhisperGael:
			return ExecutableTypeWhisperGael
		case embed.WhisperCppStream:
			return ExecutableTypeWhisperCppStream
		default:
			return ExecutableTypeUnknown
		}
	}

	// Try to identify by name
	if strings.Contains(execName, "gael") || strings.Contains(execName, "transcribe") {
		return ExecutableTypeWhisperGael
	}

	// Check for stream executable
	if strings.Contains(execName, "stream") {
		return ExecutableTypeWhisperCppStream
	}

	logger.Info(logger.CategoryTranscription, "Could not determine executable type for %s, defaulting to whisper.cpp style", execPath)
	return ExecutableTypeWhisperCpp
}

// WhisperTranscriber uses an external whisper executable for transcription
// with proper pipe-based streaming support
type WhisperTranscriber struct {
	config          Config
	executablePath  string
	modelPath       string
	mu              sync.Mutex
	recordingActive bool

	// Streaming callback for real-time results
	streamingCallback func(text string)

	// Streaming state
	streamingProcess      *exec.Cmd
	stdin                 io.WriteCloser
	stdout                *bufio.Reader
	stderr                *bufio.Reader
	streamingActive       bool
	streamingMu           sync.Mutex
	streamingStartTime    time.Time
	streamingRestartCount int
}

// NewExecutableTranscriber creates a new transcriber that uses an external whisper executable
func NewExecutableTranscriber(config Config) (ConfigurableTranscriber, error) {
	// Validate executable path
	if config.ExecutablePath == "" {
		return nil, fmt.Errorf("no executable path provided")
	}

	// Check if executable exists
	if _, err := os.Stat(config.ExecutablePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("executable not found: %s", config.ExecutablePath)
	}

	// Detect type of executable
	execType := detectExecutableType(config.ExecutablePath)
	logger.Info(logger.CategoryTranscription,
		"Creating transcriber with %s executable: %s", getExecutableTypeName(execType), config.ExecutablePath)

	// Ensure the model exists
	modelPath := config.ModelPath
	if modelPath == "" {
		// Try to use the default model path
		var err error
		modelPath, err = ensureModel(config)
		if err != nil {
			return nil, fmt.Errorf("failed to ensure model: %w", err)
		}
	} else {
		// Verify the provided model path
		if _, err := os.Stat(modelPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("model file not found: %s", modelPath)
		}
	}

	logger.Info(logger.CategoryTranscription, "Using model: %s", modelPath)

	// Create the transcriber
	t := &WhisperTranscriber{
		config:         config,
		executablePath: config.ExecutablePath,
		modelPath:      modelPath,
	}

	// Run a simple test to verify functionality
	if err := testTranscriber(t); err != nil {
		return nil, fmt.Errorf("transcriber test failed: %v", err)
	}

	logger.Info(logger.CategoryTranscription, "Transcriber successfully initialized")
	return t, nil
}

// UpdateConfig updates the transcriber's configuration
func (t *WhisperTranscriber) UpdateConfig(config Config) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.config = config
	t.modelPath = getModelPath(config.ModelPath, config.ModelSize)

	return nil
}

// GetModelInfo returns information about the current model
func (t *WhisperTranscriber) GetModelInfo() (ModelSize, string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.config.ModelSize, t.modelPath
}

// Transcribe transcribes audio data and returns the transcription
func (t *WhisperTranscriber) Transcribe(audioData []float32) (string, error) {
	return t.ProcessAudioChunk(audioData)
}

// Close frees resources
func (t *WhisperTranscriber) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Stop streaming if active
	t.stopStreaming()

	// Call cleanup for embedded files
	embed.CleanupTempFiles()

	return nil
}

// SetStreamingCallback sets a callback to receive streaming transcription results
func (t *WhisperTranscriber) SetStreamingCallback(callback func(text string)) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.streamingCallback = callback
}

// SetRecordingState updates the recording state flag to manage streaming appropriately
func (t *WhisperTranscriber) SetRecordingState(isRecording bool) {
	t.mu.Lock()
	previousState := t.recordingActive
	t.recordingActive = isRecording
	t.mu.Unlock()

	// Only take action if the state is changing
	if previousState != isRecording {
		logger.Info(logger.CategoryTranscription, "Recording state changed to: %v", isRecording)

		// If recording is stopping, terminate any streaming processes immediately
		if !isRecording {
			logger.Info(logger.CategoryTranscription, "Recording stopped - terminating streaming process")
			// Stop streaming in a goroutine to avoid deadlock with mutex, and ensure it's stopped
			go func() {
				t.stopStreaming()

				// Double-check that streaming is indeed stopped
				t.streamingMu.Lock()
				if t.streamingActive {
					logger.Warning(logger.CategoryTranscription,
						"Streaming process still active after stop attempt - forcing termination")
					// Force termination by killing the process if it's still running
					if t.streamingProcess != nil && t.streamingProcess.Process != nil {
						t.streamingProcess.Process.Kill()
					}
					t.streamingActive = false
				}
				t.streamingMu.Unlock()
			}()
		} else {
			logger.Info(logger.CategoryTranscription, "Recording started - streaming will begin with next audio chunk")
		}
	}
}

// ProcessAudioChunk processes a chunk of audio data and attempts to transcribe it
func (t *WhisperTranscriber) ProcessAudioChunk(audioData []float32) (string, error) {
	// Skip if audioData is empty
	if len(audioData) == 0 {
		return "", nil
	}

	// Check if we're actively recording
	if !t.recordingActive {
		if t.config.Debug {
			logger.Debug(logger.CategoryTranscription, "Recording inactive, skipping audio chunk")
		}
		return "", nil
	}

	t.streamingMu.Lock()
	defer t.streamingMu.Unlock()

	// If streaming process is active, send audio to it
	if t.streamingActive && t.stdin != nil {
		if err := t.sendAudioToStreamingProcess(audioData); err != nil {
			logger.Warning(logger.CategoryTranscription, "Error sending audio to streaming process: %v", err)
		}
		return "", nil
	}

	// If streaming has failed multiple times (3+), and we haven't logged a warning
	// in the last 10 seconds, log a more helpful message
	if t.streamingRestartCount >= 3 {
		// Determine if we should log a warning based on time since last restart
		timeSinceRestart := time.Since(t.streamingStartTime)
		if timeSinceRestart > 10*time.Second || t.streamingStartTime.IsZero() {
			// Logging as a warning instead of error, and only once per 10 seconds
			logger.Warning(logger.CategoryTranscription,
				"Real-time streaming is unavailable. To enable proper streaming, build the stream executable: "+
					"run scripts/build_whisper_stream.sh or use Go bindings.")

			// Update the start time to track when we last logged this warning
			t.streamingStartTime = time.Now()
		}

		// Don't return an error to prevent callers from logging additional errors
		if t.config.Debug {
			logger.Debug(logger.CategoryTranscription, "Audio chunk skipped due to unavailable streaming")
		}
		return "", nil
	}

	// Try to start streaming mode (only if we haven't hit retry limit)
	success := t.tryStreamingMode(audioData)
	if !success {
		// For the first failure, log an informative message
		if t.streamingRestartCount <= 1 {
			logger.Error(logger.CategoryTranscription,
				"Failed to start streaming transcription. Will retry %d more times before giving up.",
				3-t.streamingRestartCount)
		}

		// Don't return an error for this - we'll just try again later
		return "", nil
	}

	// If we got here, streaming is now active
	return "", nil
}

// tryStreamingMode attempts to use streaming mode with the Whisper executable
// It returns true if streaming mode was successfully used
func (t *WhisperTranscriber) tryStreamingMode(audioData []float32) bool {
	// Skip empty audio data
	if len(audioData) == 0 {
		return false
	}

	// Check recording state FIRST - this is the most critical check
	t.mu.Lock()
	isRecording := t.recordingActive
	t.mu.Unlock()

	if !isRecording {
		// If recording is not active, ensure streaming is stopped and return immediately
		logger.Debug(logger.CategoryTranscription,
			"Skipping streaming because recording is not active")
		// Stop any streaming process that might still be running
		go t.stopStreaming()
		return false
	}

	// Now check if streaming is already active
	t.streamingMu.Lock()
	isStreaming := t.streamingActive
	restartCount := t.streamingRestartCount
	t.streamingMu.Unlock()

	// If we've already tried to start streaming multiple times and failed,
	// don't keep trying every audio chunk - this prevents log spam
	if !isStreaming && restartCount >= 3 {
		// Only log this occasionally (every ~5 seconds) to avoid flooding logs
		if time.Since(t.streamingStartTime) > 5*time.Second {
			logger.Warning(logger.CategoryTranscription,
				"Streaming unavailable after 3 attempts - embedded whisper executable likely lacks stdin support")

			// Reset the timer so we don't log too frequently
			t.streamingMu.Lock()
			t.streamingStartTime = time.Now()
			t.streamingMu.Unlock()
		}
		return false
	}

	// If not streaming, we need to start the streaming process
	if !isStreaming {
		// Get the executable type to determine if streaming is supported
		embeddedExePath, _ := embed.GetWhisperExecutable()
		var execType ExecutableType

		if t.executablePath == embeddedExePath {
			embedType := embed.GetEmbeddedExecutableType()
			switch embedType {
			case embed.WhisperCpp:
				execType = ExecutableTypeWhisperCpp
			case embed.WhisperGael:
				execType = ExecutableTypeWhisperGael
			case embed.WhisperCppStream:
				execType = ExecutableTypeWhisperCppStream
			default:
				execType = ExecutableTypeUnknown
			}
		} else {
			execType = detectExecutableType(t.executablePath)
		}

		// Only WhisperCpp and WhisperCppStream support our streaming mode currently
		if execType != ExecutableTypeWhisperCpp && execType != ExecutableTypeWhisperCppStream {
			logger.Info(logger.CategoryTranscription,
				"Executable type %s doesn't support pipe-based streaming, cannot process audio",
				getExecutableTypeName(execType))
			return false
		}

		// CRITICAL: Double-check recording is still active before starting stream
		t.mu.Lock()
		isStillRecording := t.recordingActive
		t.mu.Unlock()

		if !isStillRecording {
			logger.Info(logger.CategoryTranscription,
				"Recording state changed to inactive, not starting streaming")
			return false
		}

		// Start streaming mode
		err := t.startStreaming()
		if err != nil {
			// Special case: If the error indicates stdin streaming is not supported,
			// increment the restart count to prevent repeated retries
			if strings.Contains(err.Error(), "does not support stdin streaming") {
				t.streamingMu.Lock()
				t.streamingRestartCount = 3 // Max out the restart count to prevent further attempts
				t.streamingMu.Unlock()

				logger.Error(logger.CategoryTranscription,
					"Streaming mode not available: %v - will not retry", err)
			} else {
				logger.Error(logger.CategoryTranscription,
					"Failed to start streaming mode: %v - cannot process audio", err)
			}
			return false
		}

		logger.Info(logger.CategoryTranscription, "Pipe-based streaming activated - processing audio in real-time")
	}

	// Send audio data to the streaming process
	if err := t.sendAudioToStreamingProcess(audioData); err != nil {
		logger.Warning(logger.CategoryTranscription,
			"Failed to send audio to streaming process: %v - will retry next chunk", err)

		// Only stop streaming if it's a serious error
		if strings.Contains(err.Error(), "streaming not active") {
			go t.stopStreaming()
			return false
		}
	}

	return true
}

// startStreaming starts a persistent whisper process for pipe-based streaming transcription
func (t *WhisperTranscriber) startStreaming() error {
	// Check if we're already streaming
	t.streamingMu.Lock()
	if t.streamingActive {
		t.streamingMu.Unlock()
		return nil // Already streaming, no need to start again
	}

	// Initialize streaming state
	t.streamingStartTime = time.Now()

	// Increment restart count
	t.streamingRestartCount++
	restartCount := t.streamingRestartCount
	t.streamingMu.Unlock()

	// Only log on first attempt or after threshold
	if restartCount <= 1 {
		logger.Info(logger.CategoryTranscription, "Starting streaming whisper connection")
	}

	// First check if the whisper executable exists
	if _, err := os.Stat(t.executablePath); os.IsNotExist(err) {
		return fmt.Errorf("whisper executable not found at %s", t.executablePath)
	}

	// Get executable type
	execType := detectExecutableType(t.executablePath)

	// If this is the embedded executable or known stream variant, skip the stdin test
	embeddedExePath, _ := embed.GetWhisperExecutable()
	if t.executablePath == embeddedExePath {
		embedType := embed.GetEmbeddedExecutableType()
		if embedType == embed.WhisperCppStream {
			logger.Debug(logger.CategoryTranscription,
				"Using embedded WhisperCppStream executable which is known to support stdin streaming")
			// Skip stdin test for known stream variant
		} else if embedType != embed.WhisperCpp {
			// Prevent excessive logging after first attempt
			if restartCount <= 1 {
				logger.Error(logger.CategoryTranscription,
					"The embedded whisper executable type %s does not support stdin streaming",
					embedType)
			}
			return fmt.Errorf("embedded whisper executable type %s does not support stdin streaming",
				embedType)
		}
	} else if execType == ExecutableTypeWhisperCppStream {
		logger.Debug(logger.CategoryTranscription,
			"Using known WhisperCppStream executable which supports stdin streaming")
		// Skip stdin test for known stream variant
	} else {
		// For standard executables, test if they support stdin mode
		// Only test on first attempt to avoid repeated subprocess creation
		if restartCount <= 1 {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			// Try with --stdin flag
			testCmd := exec.CommandContext(ctx, t.executablePath, "--stdin", "--help")
			testOutput, err := testCmd.CombinedOutput()
			stdinSupported := true

			// Check if there was an error related to unknown arguments
			if err != nil {
				outputStr := strings.ToLower(string(testOutput))
				if strings.Contains(outputStr, "unknown argument") ||
					strings.Contains(outputStr, "unrecognized option") ||
					strings.Contains(outputStr, "invalid option") {
					// Also try with -stdin (alternative form)
					testCmd = exec.CommandContext(ctx, t.executablePath, "-stdin", "--help")
					testOutput, err = testCmd.CombinedOutput()
					outputStr = strings.ToLower(string(testOutput))

					// If both forms fail, streaming is not supported
					if err != nil && (strings.Contains(outputStr, "unknown argument") ||
						strings.Contains(outputStr, "unrecognized option") ||
						strings.Contains(outputStr, "invalid option")) {
						stdinSupported = false
					}
				}
			}

			if !stdinSupported {
				logger.Error(logger.CategoryTranscription,
					"The whisper executable (%s) does not support stdin streaming", t.executablePath)
				return fmt.Errorf("whisper executable does not support stdin streaming; " +
					"please use a whisper.cpp binary with pipe-based streaming support")
			}
		}
	}

	// At this point, we've confirmed stdin support is available or skipped the test
	// Only log on first attempt to reduce spam
	if restartCount <= 1 {
		logger.Info(logger.CategoryTranscription, "Whisper executable supports stdin streaming")
	}

	// Configure the whisper command with appropriate streaming arguments
	args := []string{
		"-m", t.modelPath,
		"-t", "4", // Use 4 threads
		"-ml", "1", // Multilingual
		"-su",     // Single utterance mode
		"-otxt",   // Output as text
		"-nt",     // No timestamps
		"--stdin", // Accept input from stdin (pipe)
	}

	// Add language if specified
	if t.config.Language != "" && t.config.Language != "auto" {
		args = append(args, "-l", t.config.Language)
	}

	if t.config.Debug {
		logger.Debug(logger.CategoryTranscription, "Starting whisper streaming process: %s %v",
			t.executablePath, args)
	}

	// Create the command
	cmd := exec.Command(t.executablePath, args...)

	// Set up pipes for stdin, stdout, and stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return fmt.Errorf("failed to start whisper process: %w", err)
	}

	// Store the streaming state
	t.streamingMu.Lock()
	t.streamingProcess = cmd
	t.stdin = stdin
	t.stdout = bufio.NewReader(stdout)
	t.stderr = bufio.NewReader(stderr)
	t.streamingActive = true
	t.streamingStartTime = time.Now()
	t.streamingMu.Unlock()

	// Start a goroutine to read stderr for debugging
	go t.readStderr()

	// Start a goroutine to read stdout for transcription results
	go t.processStreamingOutput()

	// Start a goroutine to monitor the process and handle termination
	go t.monitorStreamingProcess()

	logger.Info(logger.CategoryTranscription, "Pipe-based streaming connection established")
	return nil
}

// stopStreaming is the safe external interface to stop streaming
func (t *WhisperTranscriber) stopStreaming() {
	t.streamingMu.Lock()
	defer t.streamingMu.Unlock()

	if !t.streamingActive {
		return
	}

	logger.Info(logger.CategoryTranscription, "Stopping streaming whisper connection")

	// Close stdin to signal the process
	if t.stdin != nil {
		t.stdin.Close()
		t.stdin = nil
	}

	// Kill the process if needed
	if t.streamingProcess != nil && t.streamingProcess.Process != nil {
		t.streamingProcess.Process.Kill()
		t.streamingProcess = nil
	}

	t.streamingActive = false
	t.stdout = nil
	t.stderr = nil

	logger.Info(logger.CategoryTranscription, "Streaming whisper connection terminated")
}

// sendAudioToStreamingProcess sends audio data directly to the whisper process via its stdin pipe
func (t *WhisperTranscriber) sendAudioToStreamingProcess(audioData []float32) error {
	t.streamingMu.Lock()
	defer t.streamingMu.Unlock()

	// Check streaming state
	if !t.streamingActive || t.stdin == nil {
		return fmt.Errorf("streaming not active")
	}

	// Check if we have valid audio data
	if len(audioData) == 0 {
		return nil // Skip empty audio data, but don't treat as error
	}

	// Calculate audio level for logging
	audioLevel := audio.CalculateRMSLevel(audioData)

	// Log debug information about the audio data
	if t.config.Debug {
		logger.Debug(logger.CategoryTranscription, "Sending audio data to streaming process: %d samples, level: %.6f",
			len(audioData), audioLevel)
	}

	// Convert audio data to the format expected by whisper.cpp (16-bit PCM)
	pcmData := audio.ConvertToPCM16(audioData)

	// Write the audio data directly to the process stdin
	_, err := t.stdin.Write(pcmData)
	if err != nil {
		t.streamingActive = false // Mark as inactive on serious errors
		t.stdin = nil             // Clear the pipe reference
		return fmt.Errorf("failed to write audio data to process: %w", err)
	}

	if t.config.Debug && audioLevel > 0.001 { // Only log meaningful audio
		logger.Debug(logger.CategoryTranscription, "Successfully sent audio data to streaming process")
	}

	return nil
}

// readStderr continuously reads stderr from the whisper process for debugging
func (t *WhisperTranscriber) readStderr() {
	for {
		t.streamingMu.Lock()
		if !t.streamingActive || t.stderr == nil {
			t.streamingMu.Unlock()
			return
		}
		stderr := t.stderr
		t.streamingMu.Unlock()

		line, err := stderr.ReadString('\n')
		if err != nil {
			if t.config.Debug && err != io.EOF {
				logger.Debug(logger.CategoryTranscription, "Error reading from whisper stderr: %v", err)
			}
			return
		}

		if line != "" && t.config.Debug {
			logger.Debug(logger.CategoryTranscription, "Whisper stderr: %s", strings.TrimSpace(line))
		}
	}
}

// processStreamingOutput reads output from the streaming process and parses it
func (t *WhisperTranscriber) processStreamingOutput() {
	for {
		t.streamingMu.Lock()
		if !t.streamingActive || t.stdout == nil {
			t.streamingMu.Unlock()
			return
		}
		stdout := t.stdout
		t.streamingMu.Unlock()

		line, err := stdout.ReadString('\n')
		if err != nil {
			if t.config.Debug && err != io.EOF {
				logger.Debug(logger.CategoryTranscription, "Error reading from whisper process: %v", err)
			}
			return
		}

		if line != "" {
			line = strings.TrimSpace(line)

			// Skip processing status lines
			if isProcessingLine(line) {
				if t.config.Debug {
					logger.Debug(logger.CategoryTranscription, "Whisper output: %s", line)
				}
				continue
			}

			// Process actual transcription result
			t.processTranscriptionLine(line)
		}
	}
}

// processTranscriptionLine processes a single line of transcription output
func (t *WhisperTranscriber) processTranscriptionLine(line string) {
	// Clean the line
	cleanedLine := cleanOutputLine(line, ExecutableTypeWhisperCpp)

	// Skip empty lines
	if cleanedLine == "" {
		return
	}

	// Normalize the result
	result := normalizeTranscriptionText(cleanedLine)

	// Send result via callback if available
	if t.streamingCallback != nil && result != "" {
		logger.Debug(logger.CategoryTranscription, "Streaming result: %s", result)
		t.streamingCallback(result)
	}
}

// monitorStreamingProcess watches the streaming process and handles termination
func (t *WhisperTranscriber) monitorStreamingProcess() {
	// Create a local copy of the process to avoid race conditions
	t.streamingMu.Lock()
	process := t.streamingProcess
	startTime := t.streamingStartTime
	t.streamingMu.Unlock()

	if process == nil {
		return
	}

	// Wait for the process to finish
	err := process.Wait()

	// Check how long the process ran
	processDuration := time.Since(startTime)
	rapidExit := processDuration < 500*time.Millisecond

	// Check exit status
	exitCode := 0
	if err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			exitCode = exiterr.ExitCode()

			if rapidExit {
				logger.Error(logger.CategoryTranscription,
					"Whisper streaming process failed immediately (after %v) with error code %d - likely unsupported flag or configuration",
					processDuration, exitCode)

				// If the process exits very quickly with a standard error code (like 1 or 2),
				// this is typically due to invalid arguments like unsupported --stdin flag
				t.streamingMu.Lock()
				t.streamingRestartCount = 3 // Max out restart count to prevent further attempts
				t.streamingMu.Unlock()

				logger.Error(logger.CategoryTranscription,
					"The embedded whisper executable does not support stdin streaming - streaming will be disabled")
			} else {
				logger.Warning(logger.CategoryTranscription,
					"Whisper streaming process exited with error code %d after running for %v",
					exitCode, processDuration)
			}
		} else {
			logger.Error(logger.CategoryTranscription,
				"Whisper streaming process failed with error: %v", err)
		}
	} else {
		logger.Info(logger.CategoryTranscription,
			"Whisper streaming process completed normally after running for %v", processDuration)
	}

	// Clean up resources
	t.streamingMu.Lock()
	wasActive := t.streamingActive
	t.streamingActive = false
	t.streamingProcess = nil
	t.stdout = nil
	t.stderr = nil
	t.stdin = nil
	t.streamingMu.Unlock()

	// Check if the process exited very quickly, which likely indicates
	// an unsupported feature or configuration error
	if rapidExit {
		// If it's a very quick exit, max out the restart count to prevent further attempts
		t.streamingMu.Lock()
		t.streamingRestartCount = 3
		t.streamingMu.Unlock()

		logger.Error(logger.CategoryTranscription,
			"Streaming process failed immediately - likely configuration incompatibility or missing stdin support, will not restart")
		return
	}

	// Check recording state - CRITICAL: Only restart if still recording
	t.mu.Lock()
	isRecording := t.recordingActive
	t.mu.Unlock()

	// Only attempt restart if:
	// 1. The stream was previously active (not manually stopped)
	// 2. Recording is still active (user hasn't stopped recording)
	// 3. The process exited with an error
	// 4. We haven't attempted too many restarts
	t.streamingMu.Lock()
	restartCount := t.streamingRestartCount
	t.streamingMu.Unlock()

	if wasActive && isRecording && exitCode != 0 && restartCount < 3 {
		logger.Warning(logger.CategoryTranscription,
			"Streaming process terminated unexpectedly, attempting to restart (attempt %d/3)",
			restartCount+1)

		// Add a small delay before restart to prevent rapid cycling
		time.Sleep(200 * time.Millisecond)

		// Increment restart count
		t.streamingMu.Lock()
		t.streamingRestartCount++
		t.streamingMu.Unlock()

		// Attempt to restart the streaming process
		if err := t.startStreaming(); err != nil {
			logger.Error(logger.CategoryTranscription,
				"Failed to restart streaming: %v - will not retry automatically", err)
		}
	} else if !isRecording {
		logger.Info(logger.CategoryTranscription,
			"Not restarting streaming process because recording is inactive")
	} else if restartCount >= 3 {
		logger.Error(logger.CategoryTranscription,
			"Maximum restart attempts reached (3), not attempting further restarts")
	} else {
		logger.Info(logger.CategoryTranscription, "Streaming whisper connection terminated")
	}
}

// isProcessingLine identifies status lines that are not part of the transcription
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

	// Remove artifacts based on executable type
	switch execType {
	case ExecutableTypeWhisperCpp:
		// Remove timestamp markers
		timestampRegex := regexp.MustCompile(`\[\d{2}:\d{2}:\d{2}\.\d{3}\s*-->\s*\d{2}:\d{2}:\d{2}\.\d{3}\]`)
		line = timestampRegex.ReplaceAllString(line, "")

		// Skip lines with only timestamps
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			return ""
		}

	case ExecutableTypeWhisperGael, ExecutableTypeWhisperPython, ExecutableTypeWhisperCppStream:
		// Remove JSON markers
		line = strings.Trim(line, "\"[]{}():,")
	}

	// Remove noise markers common to all types
	noiseRegex := regexp.MustCompile(`\[(?i)(?:MUSIC|APPLAUSE|LAUGHTER|INAUDIBLE|NOISE|CROSSTALK)\]`)
	line = noiseRegex.ReplaceAllString(line, "")

	return strings.TrimSpace(line)
}

// normalizeTranscriptionText cleans up transcription text for better quality
func normalizeTranscriptionText(text string) string {
	if text == "" {
		return ""
	}

	// Trim and normalize whitespace
	text = strings.TrimSpace(text)

	// Remove parenthetical content (noise markers)
	parenPattern := regexp.MustCompile(`\([^)]*(?i)(music|noise|applause|laughter)[^)]*\)`)
	text = parenPattern.ReplaceAllString(text, "")

	// Remove bracketed noise markers
	bracketPattern := regexp.MustCompile(`\[(?i)(?:MUSIC|APPLAUSE|LAUGHTER|INAUDIBLE|NOISE|CROSSTALK|SILENCE)\]`)
	text = bracketPattern.ReplaceAllString(text, "")

	// Normalize spaces
	spacePattern := regexp.MustCompile(`\s+`)
	text = spacePattern.ReplaceAllString(text, " ")

	// Fix punctuation
	text = strings.ReplaceAll(text, " .", ".")
	text = strings.ReplaceAll(text, " ,", ",")
	text = strings.ReplaceAll(text, " ?", "?")
	text = strings.ReplaceAll(text, " !", "!")

	// Capitalize first letter
	if len(text) > 0 {
		text = strings.ToUpper(text[:1]) + text[1:]
	}

	return strings.TrimSpace(text)
}
