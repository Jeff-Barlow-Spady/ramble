// Package transcription provides speech-to-text functionality
package transcription

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// findWhisperExecutable locates the whisper executable
func findWhisperExecutable() (string, error) {
	// Check PATH first
	if path, err := exec.LookPath("whisper"); err == nil {
		return path, nil
	}

	// Check snap location
	snapPath := "/snap/whisper-cpp/4/bin/main"
	if _, err := os.Stat(snapPath); err == nil {
		return snapPath, nil
	}

	return "", fmt.Errorf("whisper executable not found")
}

// ensureExecutablePath ensures the executable path is valid
func ensureExecutablePath(config Config) (string, error) {
	if config.ExecutablePath != "" {
		if _, err := os.Stat(config.ExecutablePath); err == nil {
			return config.ExecutablePath, nil
		}
		return "", fmt.Errorf("specified executable not found: %s", config.ExecutablePath)
	}
	return findWhisperExecutable()
}

// getModelPath returns the path for model files
func getModelPath(configPath string, modelSize ModelSize) string {
	// If config path is provided and exists, use it
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}
	}

	// Use standard XDG data directory
	homeDir, err := os.UserHomeDir()
	if err == nil {
		path := filepath.Join(homeDir, ".local", "share", "ramble", "models")
		if err := os.MkdirAll(path, 0755); err == nil {
			return path
		}
	}

	// Fallback to current directory
	return "models"
}
