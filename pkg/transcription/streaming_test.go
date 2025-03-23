package transcription

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jeff-barlow-spady/ramble/pkg/transcription/embed"
)

// Generate sine wave audio for testing
func generateTestAudio(seconds float64, frequency float64) []float32 {
	sampleRate := 16000.0
	numSamples := int(seconds * sampleRate)
	samples := make([]float32, numSamples)

	for i := 0; i < numSamples; i++ {
		t := float64(i) / sampleRate
		samples[i] = float32(math.Sin(2*math.Pi*frequency*t) * 0.5)
	}

	return samples
}

// TestExecutableFinder is a real implementation for testing
type TestExecutableFinder struct {
	returnExecutable     string
	returnAllExecutables []string
	installationPath     string
	shouldInstallSucceed bool
}

func NewTestExecutableFinder(exePath string, allExes []string, installPath string, installSucceeds bool) *TestExecutableFinder {
	return &TestExecutableFinder{
		returnExecutable:     exePath,
		returnAllExecutables: allExes,
		installationPath:     installPath,
		shouldInstallSucceed: installSucceeds,
	}
}

func (f *TestExecutableFinder) FindExecutable() string {
	return f.returnExecutable
}

func (f *TestExecutableFinder) FindAllExecutables() []string {
	return f.returnAllExecutables
}

func (f *TestExecutableFinder) InstallExecutable() (string, error) {
	if f.shouldInstallSucceed {
		return f.installationPath, nil
	}
	return "", errors.New("installation failed")
}

// TestExecutableSelector is a real implementation for testing
type TestExecutableSelector struct {
	selectionIndex int
}

func NewTestExecutableSelector(index int) *TestExecutableSelector {
	return &TestExecutableSelector{
		selectionIndex: index,
	}
}

func (s *TestExecutableSelector) SelectExecutable(executables []string) (string, error) {
	if len(executables) == 0 {
		return "", errors.New("no executables to select from")
	}

	index := s.selectionIndex
	if index < 0 || index >= len(executables) {
		index = 0 // Default to first if out of range
	}

	return executables[index], nil
}

// TestExecutableSetup tests the logic for finding or installing the whisper executable
func TestExecutableSetup(t *testing.T) {
	// Test finding the executable when provided in the config
	tempDir, err := os.MkdirTemp("", "whisper-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a dummy executable
	dummyExePath := filepath.Join(tempDir, "whisper-test")
	if runtime.GOOS == "windows" {
		dummyExePath += ".exe"
	}

	// Create an empty file and make it executable
	f, err := os.Create(dummyExePath)
	if err != nil {
		t.Fatalf("Failed to create dummy exe: %v", err)
	}
	f.Close()
	os.Chmod(dummyExePath, 0755)

	// Create a selector that chooses the first executable
	selector := NewTestExecutableSelector(0)

	// Test with the path provided
	config := DefaultConfig()
	config.ExecutablePath = dummyExePath

	execPath, err := ensureExecutablePath(config, selector)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if execPath != dummyExePath {
		t.Errorf("Expected path %s, got %s", dummyExePath, execPath)
	}

	// Test the system executable finding
	// This test is conditional - it will pass if whisper is installed, and be skipped otherwise
	if _, err := exec.LookPath("whisper"); err == nil {
		// System has whisper installed, test finding it
		config = DefaultConfig() // Use a clean config
		execPath, err = ensureExecutablePath(config, selector)
		if err != nil {
			t.Fatalf("Expected no error when whisper is in PATH, got: %v", err)
		}
		if execPath == "" {
			t.Errorf("Expected to find whisper in PATH, got empty path")
		}
	} else {
		t.Log("Skipping system executable test as whisper is not in PATH")
	}

	// Test auto-installation simulation
	mockExePath := filepath.Join(tempDir, "mock-whisper")

	// Only test on platforms where auto-install is supported
	if isWhisperInstallSupported() {
		// Create a test finder that simulates no executables found but successful installation
		testFinder := NewTestExecutableFinder("", []string{}, mockExePath, true)

		// Create a config with the test finder
		config = DefaultConfig()
		config.Finder = testFinder

		// Test with the config that has the test finder
		execPath, err = ensureExecutablePath(config, selector)
		if err != nil {
			t.Fatalf("Expected no error with test installer, got: %v", err)
		}
		if execPath != mockExePath {
			t.Errorf("Expected path %s, got %s", mockExePath, execPath)
		}
	} else {
		t.Log("Skipping auto-install test as platform is not supported")
	}
}

// Test streaming with small chunks
func TestStreamingTranscription(t *testing.T) {
	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "whisper_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test executable finder
	dummyExePath := filepath.Join(tempDir, "whisper-test")
	dummyModelPath := filepath.Join(tempDir, "model-test.bin")

	// Create dummy files
	if err := os.WriteFile(dummyExePath, []byte("#!/bin/sh\necho 'test transcription result'"), 0755); err != nil {
		t.Fatalf("Failed to create dummy executable: %v", err)
	}
	if err := os.WriteFile(dummyModelPath, []byte("dummy model data"), 0644); err != nil {
		t.Fatalf("Failed to create dummy model: %v", err)
	}

	// Create a config with our test finder
	testFinder := NewTestExecutableFinder(dummyExePath, []string{dummyExePath}, dummyExePath, true)

	config := DefaultConfig()
	config.ExecutablePath = dummyExePath
	config.ModelPath = dummyModelPath
	config.Finder = testFinder

	// Create transcriber with our dependencies
	transcriber, err := NewTranscriber(config)
	if err != nil {
		t.Fatalf("Failed to create transcriber: %v", err)
	}
	defer transcriber.Close()

	// Generate 1 second of audio
	audio := generateTestAudio(1.0, 440.0) // 440Hz tone for 1 second

	// Process the audio in smaller chunks (250ms each)
	chunkSize := 16000 / 4 // 0.25 seconds of audio at 16kHz
	chunks := len(audio) / chunkSize

	results := make([]string, 0)

	for i := 0; i < chunks; i++ {
		start := i * chunkSize
		end := (i + 1) * chunkSize
		if end > len(audio) {
			end = len(audio)
		}

		chunk := audio[start:end]
		result, err := transcriber.ProcessAudioChunk(chunk)
		if err != nil {
			t.Logf("Error processing chunk %d: %v", i, err)
		}

		if result != "" {
			results = append(results, result)
			t.Logf("Chunk %d result: %s", i, result)
		}
	}

	// Since we're using a test setup with a script that always returns a result,
	// we should expect at least one result if not using a placeholder implementation
	if len(results) == 0 {
		_, isPlaceholder := transcriber.(*placeholderTranscriber)
		if !isPlaceholder {
			t.Log("No transcription results received - this may be normal for tone-only audio")
		}
	}
}

// Test concurrent processing
func TestConcurrentProcessing(t *testing.T) {
	// Skip this test in automated environments without the binaries
	_, err := embed.GetWhisperExecutable()
	if err != nil {
		t.Skip("Skipping test: whisper executable not available")
	}

	config := DefaultConfig()
	transcriber, err := NewTranscriber(config)
	if err != nil {
		t.Fatalf("Failed to create transcriber: %v", err)
	}
	defer transcriber.Close()

	// Generate test audio
	audio := generateTestAudio(2.0, 440.0)

	// Process audio concurrently
	const numGoroutines = 5
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			result, err := transcriber.ProcessAudioChunk(audio)
			if err != nil {
				t.Logf("Goroutine %d error: %v", id, err)
			} else {
				t.Logf("Goroutine %d result: %s", id, result)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines with timeout
	timeout := time.After(30 * time.Second)
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
			// Goroutine completed successfully
		case <-timeout:
			t.Fatalf("Test timed out waiting for concurrent processing")
			return
		}
	}
}

// TestStreamingImplementation tests the true streaming implementation
func TestStreamingImplementation(t *testing.T) {
	// Set up test environment
	tempDir, err := os.MkdirTemp("", "whisper_stream_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test whisper executable that supports streaming
	testExePath := createTestStreamingExecutable(t, tempDir)
	testModelPath := filepath.Join(tempDir, "model-test.bin")

	// Create a dummy model file
	if err := os.WriteFile(testModelPath, []byte("dummy model data"), 0644); err != nil {
		t.Fatalf("Failed to create test model file: %v", err)
	}

	// Create a test config with our test executable
	config := DefaultConfig()
	config.ExecutablePath = testExePath
	config.ModelPath = testModelPath
	config.Debug = true // Enable debug for testing

	// Create a transcriber
	transcriber, err := NewExecutableTranscriber(config)
	if err != nil {
		t.Fatalf("Failed to create transcriber: %v", err)
	}
	defer transcriber.Close()

	// Set up a callback to collect results
	resultChan := make(chan string, 10)
	resultMu := sync.Mutex{}
	results := []string{}

	execTranscriber, ok := transcriber.(*ExecutableTranscriber)
	if !ok {
		t.Fatalf("Expected ExecutableTranscriber, got %T", transcriber)
	}

	execTranscriber.SetStreamingCallback(func(text string) {
		resultMu.Lock()
		defer resultMu.Unlock()
		results = append(results, text)
		resultChan <- text
	})

	// Set recording state to active before processing audio
	execTranscriber.SetRecordingState(true)

	// Test 1: Generate some test audio and send it
	audio := generateTestTone(1.0, 440.0) // 1 second of 440Hz tone

	// Process in chunks to simulate streaming
	chunkSize := 4000 // 250ms chunks at 16kHz
	chunks := len(audio) / chunkSize

	// Process the audio in chunks
	for i := 0; i < chunks; i++ {
		start := i * chunkSize
		end := (i + 1) * chunkSize
		if end > len(audio) {
			end = len(audio)
		}

		chunk := audio[start:end]
		_, err := transcriber.ProcessAudioChunk(chunk)
		if err != nil {
			t.Errorf("Error processing chunk %d: %v", i, err)
		}

		// Small delay to simulate real-time processing
		time.Sleep(50 * time.Millisecond)
	}

	// Wait for results with a timeout
	timeout := time.After(5 * time.Second)
	expectedResults := 1

	// Wait for at least one result or timeout
	resultCount := 0
	for resultCount < expectedResults {
		select {
		case <-resultChan:
			resultCount++
		case <-timeout:
			t.Logf("Timed out waiting for results, got %d so far", resultCount)
			break
		}
	}

	// Stop recording and ensure clean shutdown
	execTranscriber.SetRecordingState(false)
	time.Sleep(100 * time.Millisecond) // Small delay to allow shutdown

	// Verify we got expected results
	resultMu.Lock()
	defer resultMu.Unlock()

	if len(results) < 1 {
		t.Errorf("Expected at least 1 result, got %d", len(results))
	}

	t.Logf("Received %d transcription results from streaming", len(results))
	for i, res := range results {
		t.Logf("Result %d: %s", i, res)
	}
}

// createTestStreamingExecutable creates a test executable that simulates whisper streaming mode
func createTestStreamingExecutable(t *testing.T, tempDir string) string {
	// Create a simple bash script that simulates the whisper executable
	// It will read from stdin and write to stdout
	exePath := filepath.Join(tempDir, "test-whisper-stream")

	scriptContent := `#!/bin/bash
# Test executable that simulates whisper.cpp in streaming mode
# It echoes any arguments to stderr for verification
# It then reads from stdin and outputs simulated results to stdout

# Echo arguments for verification
echo "Arguments: $@" >&2

# Check if --help is passed
if [[ "$*" == *"--help"* ]]; then
  echo "whisper.cpp test streaming executable"
  echo "This is a test implementation for whisper.cpp streaming"
  exit 0
fi

# Simulate whisper.cpp processing
echo "whisper_init: loading model..." >&2
echo "whisper_model_load: loading model..." >&2
echo "whisper_model_load: model loaded successfully" >&2

# Read PCM data from stdin and simulate processing
# We don't actually process the audio, just simulate results
BUFFER_SIZE=0
PROCESSED=0

# Read binary data continuously
while true; do
  # Read a chunk of data (4096 bytes = 2048 int16 samples)
  if DATA=$(dd bs=4096 count=1 iflag=nonblock status=none 2>/dev/null); then
    # If data was read, increase the buffer size
    # Each 2 bytes is one audio sample
    BYTES_READ=${#DATA}
    BUFFER_SIZE=$((BUFFER_SIZE + BYTES_READ/2))

    # Simulate processing every 4000 samples (0.25s at 16kHz)
    if [ $BUFFER_SIZE -ge 4000 ]; then
      PROCESSED=$((PROCESSED + 1))
      echo "[%.2f -> %.2f] Test transcription result $PROCESSED"
      echo "whisper_print_timings: %10s %10s %10s %10s %10s %10s" >&2
      # Reset buffer size after "processing"
      BUFFER_SIZE=0
      # Simulate the time it takes to process audio
      sleep 0.05
    fi
  else
    # No data available right now, sleep a bit before trying again
    sleep 0.01

    # Check if stdin is closed (this is a bit of a hack for bash scripts)
    if ! read -t 0; then
      # If we can't read from stdin anymore, it might be closed
      # Output any remaining audio and exit
      if [ $BUFFER_SIZE -gt 0 ]; then
        PROCESSED=$((PROCESSED + 1))
        echo "[%.2f -> %.2f] Final transcription result $PROCESSED"
      fi
      echo "whisper_print_timings: done" >&2
      break
    fi
  fi
done
`

	// Write the script to a file
	err := os.WriteFile(exePath, []byte(scriptContent), 0755)
	if err != nil {
		t.Fatalf("Failed to write test executable: %v", err)
	}

	return exePath
}

// TestStreamingWithRealAudio tests the streaming implementation with synthetic but realistic audio
func TestStreamingWithRealAudio(t *testing.T) {
	// Set up test environment
	tempDir, err := os.MkdirTemp("", "whisper_stream_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test whisper executable that supports streaming
	testExePath := createTestStreamingExecutable(t, tempDir)
	testModelPath := filepath.Join(tempDir, "model-test.bin")

	// Create a dummy model file
	if err := os.WriteFile(testModelPath, []byte("dummy model data"), 0644); err != nil {
		t.Fatalf("Failed to create test model file: %v", err)
	}

	// Create a test config with our test executable
	config := DefaultConfig()
	config.ExecutablePath = testExePath
	config.ModelPath = testModelPath
	config.Debug = true // Enable debug for testing

	// Create the transcriber that uses our test executable
	testTranscriber := &TestStreamingTranscriber{
		config:         config,
		executablePath: testExePath,
		modelPath:      testModelPath,
		results:        []string{},
		resultChan:     make(chan string, 10),
	}

	// Set recording state to active before processing audio
	testTranscriber.SetRecordingState(true)

	// Generate audio with speech patterns (alternating amplitude)
	audio := generateSpeechLikeAudio(3.0) // 3 seconds of speech-like audio

	// Process audio in chunks to simulate real-time capture
	chunkSize := 1600 // 100ms at 16kHz
	totalChunks := len(audio) / chunkSize

	// Process the audio
	for i := 0; i < totalChunks; i++ {
		start := i * chunkSize
		end := (i + 1) * chunkSize
		if end > len(audio) {
			end = len(audio)
		}

		chunk := audio[start:end]
		_, err := testTranscriber.ProcessAudioChunk(chunk)
		if err != nil {
			t.Errorf("Error processing chunk %d: %v", i, err)
		}

		// Small delay to simulate real-time
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for results with a timeout
	timeout := time.After(5 * time.Second)
	select {
	case <-testTranscriber.resultChan:
		// Got at least one result
	case <-timeout:
		// No results after timeout
		if len(testTranscriber.results) == 0 {
			t.Log("No results received within timeout")
		}
	}

	// Stop recording
	testTranscriber.SetRecordingState(false)
	time.Sleep(100 * time.Millisecond) // Small delay to allow cleanup

	// Verify results
	if len(testTranscriber.results) == 0 {
		t.Error("Expected at least one transcription result")
	} else {
		t.Logf("Received %d transcription results", len(testTranscriber.results))
		for i, result := range testTranscriber.results {
			t.Logf("Result %d: %s", i, result)
		}
	}

	// Close the transcriber
	testTranscriber.Close()
}

// TestStreamingTranscriberInterface defines a test implementation for streaming transcription
type TestStreamingTranscriber struct {
	config          Config
	executablePath  string
	modelPath       string
	callback        func(string)
	cmd             *exec.Cmd
	cmdInput        io.WriteCloser
	results         []string
	resultChan      chan string
	mu              sync.Mutex
	recordingActive bool
}

// ProcessAudioChunk implements the Transcriber interface
func (t *TestStreamingTranscriber) ProcessAudioChunk(audioData []float32) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Check if recording is active
	if !t.recordingActive {
		return "", nil
	}

	// Start command if it's not already running
	if t.cmd == nil {
		if err := t.startCommand(); err != nil {
			return "", fmt.Errorf("failed to start command: %w", err)
		}
	}

	// Send audio to the process
	if err := t.sendAudio(audioData); err != nil {
		return "", fmt.Errorf("failed to send audio: %w", err)
	}

	return "", nil
}

// startCommand starts the streaming process
func (t *TestStreamingTranscriber) startCommand() error {
	// Create streaming command
	args := []string{
		"-m", t.modelPath,
		"--stream", // Enable streaming mode
		"-t", "4",  // Use 4 threads
	}

	cmd := exec.Command(t.executablePath, args...)

	// Create pipes
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
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Store state
	t.cmd = cmd
	t.cmdInput = stdin

	// Handle output in a goroutine
	go func() {
		defer stdout.Close()

		buf := make([]byte, 1024)
		var output bytes.Buffer

		for {
			n, err := stdout.Read(buf)
			if err != nil {
				if err != io.EOF {
					fmt.Fprintf(os.Stderr, "Error reading stdout: %v\n", err)
				}
				break
			}

			output.Write(buf[:n])

			// Check for complete lines
			outputStr := output.String()
			lines := strings.Split(outputStr, "\n")

			if len(lines) > 1 {
				// Process all complete lines
				for i := 0; i < len(lines)-1; i++ {
					line := strings.TrimSpace(lines[i])
					if line == "" {
						continue
					}

					// Clean up line (remove timestamps, etc.)
					line = cleanOutputForTest(line)

					if line != "" {
						t.mu.Lock()
						t.results = append(t.results, line)
						t.resultChan <- line
						if t.callback != nil {
							t.callback(line)
						}
						t.mu.Unlock()
					}
				}

				// Keep any partial line for next iteration
				output.Reset()
				output.WriteString(lines[len(lines)-1])
			}
		}
	}()

	// Handle stderr for debugging
	go io.Copy(os.Stderr, stderr)

	return nil
}

// sendAudio sends audio data to the streaming process
func (t *TestStreamingTranscriber) sendAudio(audioData []float32) error {
	if t.cmdInput == nil {
		return fmt.Errorf("no stdin pipe available")
	}

	// Convert float32 to int16 PCM
	pcmData := make([]int16, len(audioData))
	for i, sample := range audioData {
		// Clamp to [-1.0, 1.0]
		if sample > 1.0 {
			sample = 1.0
		} else if sample < -1.0 {
			sample = -1.0
		}

		// Convert to int16
		pcmData[i] = int16(sample * 32767.0)
	}

	// Write to stdin
	return binary.Write(t.cmdInput, binary.LittleEndian, pcmData)
}

// Close implements the Transcriber interface
func (t *TestStreamingTranscriber) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Close stdin if open
	if t.cmdInput != nil {
		t.cmdInput.Close()
		t.cmdInput = nil
	}

	// Kill process if running
	if t.cmd != nil && t.cmd.Process != nil {
		t.cmd.Process.Kill()
		t.cmd = nil
	}

	close(t.resultChan)
	return nil
}

// Update the Config interface
func (t *TestStreamingTranscriber) UpdateConfig(config Config) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.config = config
	return nil
}

// GetModelInfo implements the ConfigurableTranscriber interface
func (t *TestStreamingTranscriber) GetModelInfo() (ModelSize, string) {
	return t.config.ModelSize, t.modelPath
}

// SetStreamingCallback implements the ConfigurableTranscriber interface
func (t *TestStreamingTranscriber) SetStreamingCallback(callback func(text string)) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.callback = callback
}

// SetRecordingState implements the ConfigurableTranscriber interface
func (t *TestStreamingTranscriber) SetRecordingState(isRecording bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Only take action if the state is changing
	if t.recordingActive != isRecording {
		t.recordingActive = isRecording

		// If recording is stopping, clean up resources
		if !isRecording && t.cmd != nil {
			// Close stdin if open
			if t.cmdInput != nil {
				t.cmdInput.Close()
				t.cmdInput = nil
			}

			// Kill process if running
			if t.cmd.Process != nil {
				t.cmd.Process.Kill()
			}

			t.cmd = nil
		}
	}
}

// Helper functions for tests

// cleanOutputForTest cleans up the output from the test executable
func cleanOutputForTest(line string) string {
	// Remove timestamp markers
	timestampRegex := regexp.MustCompile(`\[\d{2}:\d{2}\.\d{3}\s*-->\s*\d{2}:\d{2}\.\d{3}\]`)
	line = timestampRegex.ReplaceAllString(line, "")

	return strings.TrimSpace(line)
}

// generateTestTone generates a test audio signal with a specific frequency
func generateTestTone(durationSecs float64, frequency float64) []float32 {
	// Sample rate is 16kHz
	sampleRate := 16000.0
	numSamples := int(durationSecs * sampleRate)

	audio := make([]float32, numSamples)
	for i := 0; i < numSamples; i++ {
		t := float64(i) / sampleRate
		audio[i] = float32(0.5 * math.Sin(2.0*math.Pi*frequency*t))
	}

	return audio
}

// generateSpeechLikeAudio creates audio that mimics speech patterns
func generateSpeechLikeAudio(durationSecs float64) []float32 {
	// Sample rate is 16kHz
	sampleRate := 16000.0
	numSamples := int(durationSecs * sampleRate)

	audio := make([]float32, numSamples)

	// Create a more speech-like pattern with varying frequencies and amplitudes
	for i := 0; i < numSamples; i++ {
		t := float64(i) / sampleRate

		// Base frequency (simulating fundamental frequency of voice)
		baseFreq := 110.0

		// Add some harmonics
		harmonic1 := 0.5 * math.Sin(2.0*math.Pi*baseFreq*t)
		harmonic2 := 0.3 * math.Sin(2.0*math.Pi*baseFreq*2*t)
		harmonic3 := 0.2 * math.Sin(2.0*math.Pi*baseFreq*3*t)

		// Amplitude modulation to simulate syllables (4 syllables per second)
		syllableFreq := 4.0
		modulation := 0.5 + 0.5*math.Sin(2.0*math.Pi*syllableFreq*t)

		// Combine harmonics with modulation
		signal := modulation * (harmonic1 + harmonic2 + harmonic3)

		// Add some noise for realism
		noise := 0.05 * (2.0*rand.Float64() - 1.0)

		// Normalize to [-1,1] range
		audio[i] = float32(0.7*signal + noise)
	}

	return audio
}
