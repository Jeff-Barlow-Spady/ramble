//go:build whisper_go
// +build whisper_go

package embed

import (
	"os"
	"testing"
)

func TestRegisterAndCleanupTempFiles(t *testing.T) {
	// Create a temporary file
	tmpfile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpfile.Name()
	tmpfile.Close()

	// Register it for cleanup
	RegisterTempFile(tmpPath)

	// Ensure the file exists
	if _, err := os.Stat(tmpPath); os.IsNotExist(err) {
		t.Fatalf("Test file doesn't exist before cleanup: %s", tmpPath)
	}

	// Clean up
	CleanupTempFiles()

	// Ensure the file was removed
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Fatalf("Test file still exists after cleanup: %s", tmpPath)
	}
}

func TestWhisperGoStubs(t *testing.T) {
	// Test HasEmbeddedAssets returns false
	if HasEmbeddedAssets() {
		t.Error("HasEmbeddedAssets() should return false with whisper_go build")
	}

	// Test ExtractModel returns the expected error
	_, err := ExtractModel("small")
	if err != ErrAssetsNotEmbedded {
		t.Errorf("ExtractModel() expected error %v, got %v", ErrAssetsNotEmbedded, err)
	}

	// Test GetWhisperExecutable returns the expected error
	_, err = GetWhisperExecutable()
	if err != ErrAssetsNotEmbedded {
		t.Errorf("GetWhisperExecutable() expected error %v, got %v", ErrAssetsNotEmbedded, err)
	}

	// Test GetEmbeddedExecutableType returns the expected type
	if GetEmbeddedExecutableType() != WhisperCppStream {
		t.Errorf("GetEmbeddedExecutableType() didn't return WhisperCppStream")
	}
}
