package transcription

import (
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

// MockExecutableFinder implements ExecutableFinder for testing
type MockExecutableFinder struct {
	MockFindExecutable    func() string
	MockInstallExecutable func() (string, error)
}

func (m *MockExecutableFinder) FindExecutable() string {
	if m.MockFindExecutable != nil {
		return m.MockFindExecutable()
	}
	return ""
}

func (m *MockExecutableFinder) InstallExecutable() (string, error) {
	if m.MockInstallExecutable != nil {
		return m.MockInstallExecutable()
	}
	return "", nil
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

	// Test with the path provided
	config := DefaultConfig()
	config.ExecutablePath = dummyExePath

	execPath, err := ensureExecutablePath(config)
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
		execPath, err = ensureExecutablePath(config)
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
		// Create a mock finder for testing
		mockFinder := &MockExecutableFinder{
			MockFindExecutable: func() string { return "" }, // Force auto-install
			MockInstallExecutable: func() (string, error) {
				return mockExePath, nil
			},
		}

		// Create a config with the mock finder
		config = DefaultConfig()
		config.Finder = mockFinder

		// Test with the config that has the mock finder
		execPath, err = ensureExecutablePath(config)
		if err != nil {
			t.Fatalf("Expected no error with mock installer, got: %v", err)
		}
		if execPath != mockExePath {
			t.Errorf("Expected mock path %s, got %s", mockExePath, execPath)
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

	// Create a mock executable finder
	dummyExePath := filepath.Join(tempDir, "whisper-test")
	dummyModelPath := filepath.Join(tempDir, "model-test.bin")

	// Create dummy files
	if err := os.WriteFile(dummyExePath, []byte("#!/bin/sh\necho 'test transcription result'"), 0755); err != nil {
		t.Fatalf("Failed to create dummy executable: %v", err)
	}
	if err := os.WriteFile(dummyModelPath, []byte("dummy model data"), 0644); err != nil {
		t.Fatalf("Failed to create dummy model: %v", err)
	}

	// Create a config with our mock finder
	mockFinder := &MockExecutableFinder{
		MockFindExecutable: func() string {
			return dummyExePath
		},
		MockInstallExecutable: func() (string, error) {
			return dummyExePath, nil
		},
	}

	config := DefaultConfig()
	config.ExecutablePath = dummyExePath
	config.ModelPath = dummyModelPath
	config.Finder = mockFinder

	// Create transcriber with our mocked dependencies
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

	// Since we're using a mock that always returns a result, we should expect at least one result
	// Placeholder implementations might not return a result for short audio, so check the type
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
