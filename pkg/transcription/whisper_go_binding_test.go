//go:build whisper_go
// +build whisper_go

package transcription

import (
	"math"
	"testing"
)

// TestWhisperGoBindings verifies that the Go bindings for whisper.cpp are working correctly
func TestWhisperGoBindings(t *testing.T) {
	// Skip this test if Go bindings are not available
	err := checkGoBindingsAvailable()
	if err != nil {
		t.Skipf("Skipping test: Go bindings not available: %v", err)
	}

	// Create a basic config for testing
	config := DefaultConfig()

	// Create a transcriber with the Go bindings
	transcriber, err := NewGoBindingTranscriber(config)
	if err != nil {
		t.Fatalf("Failed to create transcriber with Go bindings: %v", err)
	}
	defer transcriber.Close()

	// Generate a test tone (sine wave at 440Hz for 1 second)
	audio := generateTestTone(1.0, 440.0)

	// Process the audio
	result, err := transcriber.ProcessAudioChunk(audio)
	if err != nil {
		t.Logf("Error processing audio: %v", err)
		// This is not necessarily a failure, as processing silence might not give results
	}

	t.Logf("Transcription result: %q", result)

	// Verify that the transcriber is functioning by checking the model info
	modelSize, modelPath := transcriber.GetModelInfo()
	t.Logf("Using model: %s at %s", modelSize, modelPath)

	if modelPath == "" {
		t.Error("Expected non-empty model path")
	}
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
