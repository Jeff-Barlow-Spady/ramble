package audio

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestCalculateRMSLevel(t *testing.T) {
	testCases := []struct {
		name     string
		input    []float32
		expected float32
	}{
		{
			name:     "empty buffer",
			input:    []float32{},
			expected: 0,
		},
		{
			name:     "single value buffer",
			input:    []float32{0.5},
			expected: 0.5,
		},
		{
			name:     "silence",
			input:    []float32{0, 0, 0, 0},
			expected: 0,
		},
		{
			name:     "constant amplitude",
			input:    []float32{0.5, 0.5, 0.5, 0.5},
			expected: 0.5,
		},
		{
			name:     "varying amplitude",
			input:    []float32{0, 1, 0, -1},
			expected: float32(math.Sqrt(0.5)),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := CalculateRMSLevel(tc.input)
			if math.Abs(float64(result-tc.expected)) > 0.0001 {
				t.Errorf("Expected %f, got %f", tc.expected, result)
			}
		})
	}
}

func TestWavSaveLoad(t *testing.T) {
	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "audio_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test data
	testData := []float32{0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0}

	// Save to WAV
	wavPath := filepath.Join(tempDir, "test.wav")
	err = SaveToWav(testData, wavPath)
	if err != nil {
		t.Fatalf("Failed to save WAV: %v", err)
	}

	// Load the WAV file
	loadedData, err := LoadFromWav(wavPath)
	if err != nil {
		t.Fatalf("Failed to load WAV: %v", err)
	}

	// Check the data length
	if len(loadedData) != len(testData) {
		t.Errorf("Expected length %d, got %d", len(testData), len(loadedData))
	}

	// Check the data values (with some tolerance for potential precision loss)
	for i := 0; i < len(testData) && i < len(loadedData); i++ {
		if math.Abs(float64(loadedData[i]-testData[i])) > 0.01 {
			t.Errorf("At index %d: expected %f, got %f", i, testData[i], loadedData[i])
		}
	}
}

func TestProcessDspFilters(t *testing.T) {
	// Test a basic case
	input := []float32{0.1, -0.2, 0.3, -0.4}
	output := ProcessDspFilters(input)

	// Currently, the function should just return the input
	for i, val := range input {
		if val != output[i] {
			t.Errorf("Expected ProcessDspFilters to return input, got different values at index %d", i)
		}
	}
}

// TestConvertToPCM16 tests the ConvertToPCM16 function for converting audio samples to 16-bit PCM
func TestConvertToPCM16(t *testing.T) {
	tests := []struct {
		name     string
		input    []float32
		expected []byte
	}{
		{
			name:     "empty input",
			input:    []float32{},
			expected: []byte{},
		},
		{
			name:     "zero value",
			input:    []float32{0.0},
			expected: []byte{0, 0}, // 0.0 -> int16(0) -> [0, 0]
		},
		{
			name:     "positive one",
			input:    []float32{1.0},
			expected: []byte{0xFF, 0x7F}, // 1.0 -> int16(32767) -> [0xFF, 0x7F] in little-endian
		},
		{
			name:     "negative one",
			input:    []float32{-1.0},
			expected: []byte{0x00, 0x80}, // -1.0 -> int16(-32768) -> [0x00, 0x80] in little-endian
		},
		{
			name:  "multiple values",
			input: []float32{0.5, -0.5, 0.0},
			expected: []byte{
				0xFF, 0x3F, // 0.5 -> int16(16383) -> [0xFF, 0x3F]
				0x01, 0xC0, // -0.5 -> int16(-16384) -> [0x01, 0xC0]
				0x00, 0x00, // 0.0 -> int16(0) -> [0x00, 0x00]
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToPCM16(tt.input)

			if len(result) != len(tt.expected) {
				t.Fatalf("Expected length %d, got %d", len(tt.expected), len(result))
			}

			for i := 0; i < len(result); i++ {
				if result[i] != tt.expected[i] {
					t.Errorf("Byte at index %d: expected 0x%02X, got 0x%02X",
						i, tt.expected[i], result[i])
				}
			}
		})
	}
}
