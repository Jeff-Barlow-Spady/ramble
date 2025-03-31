package embed

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestExtractModel(t *testing.T) {
	// Skip test if embed doesn't have the assets during testing
	// In a real environment, this wouldn't be skipped
	_, err := Assets.ReadFile("models/small.bin")
	if err != nil {
		t.Skip("Test skipped: embed assets not available")
	}

	// Try to extract the model
	modelPath, err := ExtractModel("small")

	// Check if we got the "model is too small" error, which is expected with placeholder models
	if err != nil && err.Error() == "embedded model is too small (586836 bytes), needs to download full model" {
		t.Skip("Test skipped: placeholder model detected. Full model needed for this test.")
	}

	if err != nil {
		t.Fatalf("ExtractModel failed: %v", err)
	}

	// Check that the model file exists
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		t.Fatalf("Extracted model doesn't exist: %s", modelPath)
	}

	// Clean up
	os.RemoveAll(filepath.Dir(modelPath))
}

func TestGetWhisperExecutable(t *testing.T) {
	// Skip test if embed doesn't have the assets during testing
	platform := runtime.GOOS + "-" + runtime.GOARCH
	binPath := filepath.Join("binaries", platform, "whisper")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}

	_, err := Assets.ReadFile(binPath)
	if err != nil {
		t.Skip("Test skipped: embed assets not available")
	}

	execPath, err := GetWhisperExecutable()
	if err != nil {
		t.Fatalf("GetWhisperExecutable failed: %v", err)
	}

	// Check that the executable exists and is executable
	info, err := os.Stat(execPath)
	if os.IsNotExist(err) {
		t.Fatalf("Extracted executable doesn't exist: %s", execPath)
	}

	// On Unix systems, check if the file is executable
	if runtime.GOOS != "windows" {
		if info.Mode()&0111 == 0 {
			t.Fatalf("Extracted file is not executable: %s", execPath)
		}
	}

	// Clean up
	os.RemoveAll(filepath.Dir(execPath))
}

func TestHasEmbeddedAssets(t *testing.T) {
	// This will likely return false during tests, which is fine
	result := HasEmbeddedAssets()

	// Just verify the function runs without panic
	t.Logf("HasEmbeddedAssets() returned: %v", result)

	// If it returns true, we should be able to access the assets
	if result {
		platform := runtime.GOOS + "-" + runtime.GOARCH
		binaryPath := filepath.Join("binaries", platform, "whisper")
		if runtime.GOOS == "windows" {
			binaryPath += ".exe"
		}

		_, err := Assets.ReadFile(binaryPath)
		if err != nil {
			t.Errorf("HasEmbeddedAssets() returned true but assets not readable: %v", err)
		}
	}
}
