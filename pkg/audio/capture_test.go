package audio

import (
	"testing"
)

// TestCalculateLevel tests the audio level calculation function
func TestCalculateLevel(t *testing.T) {
	// Test cases
	testCases := []struct {
		name     string
		samples  []float32
		expected float32
	}{
		{
			name:     "Empty buffer",
			samples:  []float32{},
			expected: 0,
		},
		{
			name:     "Zero samples",
			samples:  []float32{0, 0, 0, 0},
			expected: 0,
		},
		{
			name:     "Full scale sine wave peak",
			samples:  []float32{1.0, 0, -1.0, 0},
			expected: 0.5, // RMS of a sine wave is 1/sqrt(2) ≈ 0.7071, but our sequence isn't a full sine wave
		},
		{
			name:     "Half scale",
			samples:  []float32{0.5, 0.5, 0.5, 0.5},
			expected: 0.5,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			level := CalculateLevel(tc.samples)

			// Allow for some floating point imprecision
			if tc.expected == 0 && level != 0 {
				t.Errorf("Expected 0, got %f", level)
			} else if tc.expected > 0 && (level < tc.expected*0.95 || level > tc.expected*1.05) {
				t.Errorf("Expected %f, got %f", tc.expected, level)
			}
		})
	}
}

// TestCaptureCreation tests the creation of the capture object
func TestCaptureCreation(t *testing.T) {
	// Skip test if PortAudio is not available
	// This is a compromise - we test the code path but don't require audio hardware
	capture, err := New(16000, true)
	if err != nil {
		t.Skip("Skipping test as audio initialization failed. This is normal if no audio device is available.")
	}

	// Ensure proper cleanup
	defer capture.Close()

	// Verify initialization
	if capture == nil {
		t.Error("Capture object is nil")
	}

	if capture.sampleRate != 16000 {
		t.Errorf("Expected sample rate 16000, got %f", capture.sampleRate)
	}

	if capture.isActive {
		t.Error("Expected isActive to be false initially")
	}
}

// TestIsActive tests the IsActive method
func TestIsActive(t *testing.T) {
	// Create a capture instance without initializing PortAudio
	capture := &Capture{
		isActive: false,
	}

	if capture.IsActive() {
		t.Error("Expected IsActive() to return false initially")
	}

	// Set to active
	capture.isActive = true

	if !capture.IsActive() {
		t.Error("Expected IsActive() to return true after setting isActive")
	}
}

// TestAudioCallback tests the audio callback processing
func TestAudioCallback(t *testing.T) {
	// Create a capture instance without initializing PortAudio
	capture := &Capture{
		isActive: true,
	}

	// Test variables to verify callback execution
	callbackExecuted := false
	var capturedData []float32

	// Set up a callback
	capture.onAudio = func(samples []float32) {
		callbackExecuted = true
		capturedData = samples
	}

	// Test data
	testData := []float32{0.1, 0.2, 0.3, 0.4}

	// Process the test data
	capture.processAudio(testData, nil)

	// Verify callback execution
	if !callbackExecuted {
		t.Error("Callback was not executed")
	}

	// Verify the data was copied properly
	if len(capturedData) != len(testData) {
		t.Errorf("Expected %d samples, got %d", len(testData), len(capturedData))
	}

	for i, sample := range testData {
		if capturedData[i] != sample {
			t.Errorf("Sample %d: expected %f, got %f", i, sample, capturedData[i])
		}
	}
}
