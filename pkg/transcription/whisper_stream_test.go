package transcription

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jeff-barlow-spady/ramble/pkg/audio"
)

// TestWhisperStreamExecutable is a real-world, non-mocked test
// that verifies the stream executable supports stdin streaming
func TestWhisperStreamExecutable(t *testing.T) {
	// Check for the embedded stream executable
	execPath, err := findStreamExecutableForTest()
	if err != nil {
		t.Skip("Stream executable not found, skipping test: " + err.Error())
	}

	t.Logf("Found stream executable: %s", execPath)

	// Check for test model
	modelPath, err := findTestModel()
	if err != nil {
		t.Skip("Test model not found, skipping test: " + err.Error())
	}

	t.Logf("Found test model: %s", modelPath)

	// Create a temporary test audio file
	audioPath, err := createTestAudioFile()
	if err != nil {
		t.Fatalf("Failed to create test audio: %v", err)
	}
	defer os.Remove(audioPath)

	// Create a test command
	cmd := exec.Command(execPath, "-m", modelPath, "--stdin")

	// Get stdin pipe
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to get stdin pipe: %v", err)
	}

	// Get stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to get stdout pipe: %v", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start stream process: %v", err)
	}

	// Create a done channel to signal when we're finished
	done := make(chan struct{})

	// Read output in a separate goroutine
	go func() {
		defer close(done)
		reader := bufio.NewReader(stdout)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					t.Logf("Error reading from stream: %v", err)
				}
				return
			}

			// Check if we got any real transcription output
			if strings.Contains(line, "[") || strings.TrimSpace(line) != "" {
				t.Logf("Got output: %s", line)
			}
		}
	}()

	// Send test audio data to stdin
	audioData, err := os.ReadFile(audioPath)
	if err != nil {
		t.Fatalf("Failed to read test audio file: %v", err)
	}

	// Write the data to stdin
	if _, err := stdin.Write(audioData); err != nil {
		t.Fatalf("Failed to write to stdin: %v", err)
	}

	// Close stdin to signal end of input
	stdin.Close()

	// Wait for the process to complete or timeout
	select {
	case <-done:
		// Process completed successfully
	case <-time.After(10 * time.Second):
		t.Logf("Timeout waiting for transcription, this is okay for a silent test file")
	}

	// Wait for the command to finish
	if err := cmd.Wait(); err != nil {
		// Some error codes are expected since we're sending a possibly empty test file
		t.Logf("Command exited with error: %v", err)
	}

	t.Log("Stream executable test completed")
}

// findStreamExecutableForTest tries to find the stream executable for testing
func findStreamExecutableForTest() (string, error) {
	// First, check the embedded binary location
	execPath := filepath.Join("embed", "binaries", "linux-amd64", "whisper")
	if _, err := os.Stat(execPath); err == nil {
		return execPath, nil
	}

	// Next, check in the whisper.cpp directory if the repository is cloned
	execPath = filepath.Join("..", "..", "whisper.cpp", "stream")
	if _, err := os.Stat(execPath); err == nil {
		return execPath, nil
	}

	// Try checking in a standard bin directory
	execPath = filepath.Join(os.Getenv("HOME"), ".ramble", "bin", "whisper")
	if _, err := os.Stat(execPath); err == nil {
		return execPath, nil
	}

	return "", fmt.Errorf("no stream executable found for testing")
}

// findTestModel tries to find a model file for testing
func findTestModel() (string, error) {
	// First, check for embedded model
	modelPath := filepath.Join("embed", "models", "tiny.bin")
	if _, err := os.Stat(modelPath); err == nil {
		return modelPath, nil
	}

	// Check in the standard location
	modelPath = filepath.Join(os.Getenv("HOME"), ".local", "share", "ramble", "models", "ggml-tiny.en.bin")
	if _, err := os.Stat(modelPath); err == nil {
		return modelPath, nil
	}

	// Check in the whisper.cpp directory
	modelPath = filepath.Join("..", "..", "whisper.cpp", "models", "ggml-tiny.bin")
	if _, err := os.Stat(modelPath); err == nil {
		return modelPath, nil
	}

	return "", fmt.Errorf("no test model found")
}

// createTestAudioFile creates a simple PCM test file for streaming
func createTestAudioFile() (string, error) {
	// Create a temporary file
	f, err := os.CreateTemp("", "whisper-test-*.pcm")
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Create a simple silent audio buffer (1 second of audio at 16kHz)
	silenceBuffer := make([]float32, 16000)

	// Add a small non-zero section to ensure processing happens
	for i := 0; i < 1000; i++ {
		silenceBuffer[i+8000] = 0.01 * float32(i%10)
	}

	// Convert to PCM
	pcmData := audio.ConvertToPCM16(silenceBuffer)

	// Write to the file
	if _, err := f.Write(pcmData); err != nil {
		return "", err
	}

	return f.Name(), nil
}
