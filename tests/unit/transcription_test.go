//go:build cgo && whisper_go
// +build cgo,whisper_go

package unit

import (
	"testing"

	"github.com/jeff-barlow-spady/ramble/pkg/transcription"
)

func TestTranscriptionManager(t *testing.T) {
	// Skip this test if we don't have a valid model file
	// This is important because we don't want CI to fail if models aren't present
	modelPath := transcription.GetLocalModelPath(transcription.ModelTiny)
	if modelPath == "" {
		t.Skip("Skipping transcription test: No model file found")
	}

	t.Run("NewManager", func(t *testing.T) {
		// Test creating a new manager
		manager, err := transcription.NewManager(modelPath)
		if err != nil {
			t.Fatalf("Failed to create transcription manager: %v", err)
		}
		defer manager.Close()

		// Verify manager is not nil
		if manager == nil {
			t.Fatal("Expected transcription manager to be created, got nil")
		}
	})

	t.Run("SetRecordingState", func(t *testing.T) {
		// Create a manager
		manager, err := transcription.NewManager(modelPath)
		if err != nil {
			t.Fatalf("Failed to create transcription manager: %v", err)
		}
		defer manager.Close()

		// Set recording state
		manager.SetRecordingState(true)
		// Set it back to false
		manager.SetRecordingState(false)
		// No assertions needed - just testing it doesn't crash
	})

	t.Run("ProcessAudio", func(t *testing.T) {
		// Test processing audio data with empty buffer
		manager, err := transcription.NewManager(modelPath)
		if err != nil {
			t.Fatalf("Failed to create transcription manager: %v", err)
		}
		defer manager.Close()

		// Start recording
		manager.SetRecordingState(true)

		// Process empty buffer (shouldn't crash)
		emptyBuffer := []float32{}
		_, err = manager.ProcessAudioChunk(emptyBuffer)
		if err != nil {
			t.Errorf("ProcessAudioChunk failed with empty buffer: %v", err)
		}

		// Create a small dummy buffer (not enough to trigger processing)
		smallBuffer := make([]float32, 1000)
		_, err = manager.ProcessAudioChunk(smallBuffer)
		if err != nil {
			t.Errorf("ProcessAudioChunk failed with small buffer: %v", err)
		}

		// Stop recording
		manager.SetRecordingState(false)
	})

	t.Run("SetStreamingCallback", func(t *testing.T) {
		// Test setting callback function
		manager, err := transcription.NewManager(modelPath)
		if err != nil {
			t.Fatalf("Failed to create transcription manager: %v", err)
		}
		defer manager.Close()

		// Test with nil callback
		manager.SetStreamingCallback(nil)

		// Test with actual callback
		manager.SetStreamingCallback(func(text string) {
			// We don't expect this to be called in the test
		})

		// No assertions needed - just testing it doesn't crash
	})
}
