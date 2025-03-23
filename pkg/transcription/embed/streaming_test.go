package embed

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestWhisperStreamingSupport tests that the embedded executable supports stdin streaming
func TestWhisperStreamingSupport(t *testing.T) {
	// Skip test if embed doesn't have the assets during testing
	// In a real environment, this wouldn't be skipped
	platform := runtime.GOOS + "-" + runtime.GOARCH
	binaryPath := filepath.Join("binaries", platform, "whisper")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}

	_, err := Assets.ReadFile(binaryPath)
	if err != nil {
		t.Skip("Test skipped: embed assets not available")
	}

	// Get the embedded whisper executable path
	whisperPath, err := GetWhisperExecutable()
	if err != nil {
		t.Fatalf("Failed to get whisper executable: %v", err)
	}

	// Verify the file exists
	if _, err := os.Stat(whisperPath); os.IsNotExist(err) {
		t.Fatalf("Whisper executable not found at: %s", whisperPath)
	}

	// Test if it supports the --stdin flag
	cmd := exec.Command(whisperPath, "--stdin", "--help")
	output, err := cmd.CombinedOutput()
	outputStr := strings.ToLower(string(output))

	// Check for streaming support
	if err != nil && (strings.Contains(outputStr, "unknown argument") ||
		strings.Contains(outputStr, "unrecognized option") ||
		strings.Contains(outputStr, "invalid option")) {

		t.Fatalf("Embedded whisper executable does not support --stdin flag: %v\n%s", err, outputStr)
	}

	// Check that the help text mentions stdin
	if !strings.Contains(outputStr, "stdin") {
		t.Fatalf("Help text doesn't mention stdin, executable may not support streaming: %s", outputStr)
	}

	// Output the executable type for verification
	execType := GetEmbeddedExecutableType()
	t.Logf("Embedded executable type: %v", execType)

	// Verify it's the WhisperCppStream type
	if execType != WhisperCppStream {
		t.Errorf("Expected executable type to be WhisperCppStream, got: %v", execType)
	}
}
