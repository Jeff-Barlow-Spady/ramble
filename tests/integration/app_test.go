//go:build cgo && whisper_go
// +build cgo,whisper_go

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jeff-barlow-spady/ramble/pkg/audio"
	"github.com/jeff-barlow-spady/ramble/pkg/transcription"
)

// Integration tests for the Ramble application
func TestApplicationStartup(t *testing.T) {
	// Skip if no model is available
	modelPath := transcription.GetLocalModelPath(transcription.ModelTiny)
	if modelPath == "" {
		t.Skip("Skipping integration test: No model file found")
	}

	t.Run("ApplicationInit", func(t *testing.T) {
		// Test audio subsystem initialization
		capture, err := audio.New(16000, false)
		if err != nil {
			t.Skipf("Audio initialization failed (normal in CI environments): %v", err)
			return
		}
		defer capture.Close()

		// Verify audio capture was created
		if capture == nil {
			t.Fatal("Expected audio capture to be created, got nil")
		}
	})

	t.Run("TranscriptionFlow", func(t *testing.T) {
		// Load the test audio file
		testDataPath := filepath.Join("testdata", "test_audio.wav")
		if _, err := os.Stat(testDataPath); os.IsNotExist(err) {
			t.Skip("Skipping test audio processing: test audio file not found")
		}

		// Create transcription manager
		manager, err := transcription.NewManager(modelPath)
		if err != nil {
			t.Fatalf("Failed to create transcription manager: %v", err)
		}
		defer manager.Close()

		// Setup test callback
		manager.SetStreamingCallback(func(text string) {
			// In a real test we'd do something with this text
			// but for now just verifying the callback works without errors
		})

		// Start recording
		manager.SetRecordingState(true)

		// In a real test, we would load the audio file and process it through the manager
		// For this test, we'll just simulate receiving audio by sending a small buffer
		// This won't actually produce transcription but tests the flow
		buffer := make([]float32, 16000) // 1 second of audio at 16kHz
		_, err = manager.ProcessAudioChunk(buffer)
		if err != nil {
			t.Errorf("Error processing audio chunk: %v", err)
		}

		// Stop recording
		manager.SetRecordingState(false)

		// No real assertion for text in this test since we're not loading real audio
		// In a full test, we'd load real audio and check for non-empty text
	})

	t.Run("AudioLevelCalculation", func(t *testing.T) {
		// Create some test audio data
		samples := []float32{0.1, 0.5, -0.3, 0.8, -0.2}

		// Calculate the audio level
		level := audio.CalculateLevel(samples)

		// Check that the level is reasonable
		if level < 0 || level > 1.0 {
			t.Errorf("Expected audio level between 0 and 1, got %f", level)
		}
	})
}
