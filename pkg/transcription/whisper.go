// Package transcription provides speech-to-text functionality
package transcription

import (
	"os"
	"path/filepath"
)

// ModelSize options for Whisper
type ModelSize string

const (
	ModelTiny   ModelSize = "tiny"
	ModelBase   ModelSize = "base"
	ModelSmall  ModelSize = "small"
	ModelMedium ModelSize = "medium"
	ModelLarge  ModelSize = "large"
)

// GetLocalModelPath returns the path to a model file either in the system location
// or in the user's home directory
func GetLocalModelPath(modelSize ModelSize) string {
	if modelSize == "" {
		modelSize = ModelTiny
	}

	// Look in standard locations
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// Standard file naming pattern: ggml-{model}.bin
	modelFileName := "ggml-" + string(modelSize) + ".en.bin"

	// Locations to check (in order of preference)
	locations := []string{
		// Current directory
		filepath.Join(".", modelFileName),
		// Models directory in current directory
		filepath.Join(".", "models", modelFileName),
		// User's home directory models
		filepath.Join(homeDir, ".local", "share", "ramble", "models", modelFileName),
		// Global models directory
		filepath.Join("/usr", "local", "share", "ramble", "models", modelFileName),
	}

	// Check each location
	for _, location := range locations {
		if _, err := os.Stat(location); err == nil {
			return location
		}
	}

	return ""
}
