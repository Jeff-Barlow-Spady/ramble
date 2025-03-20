// Package transcription provides speech-to-text functionality
package transcription

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jeff-barlow-spady/ramble/pkg/logger"
)

// getModelPath determines the correct model file path based on configuration
func getModelPath(modelPathOverride string, modelSize ModelSize) string {
	// If specific path is provided, use it
	if modelPathOverride != "" {
		return modelPathOverride
	}

	// Try to find model in standard locations
	homeDir, err := os.UserHomeDir()
	if err != nil {
		logger.Warning(logger.CategoryTranscription, "Failed to get user home directory: %v", err)
		return "" // Return empty to trigger download
	}

	// Model file name format varies slightly between implementations
	possibleNames := []string{
		fmt.Sprintf("ggml-%s.en.bin", modelSize), // with .en suffix (as downloaded)
		fmt.Sprintf("ggml-%s.bin", modelSize),    // without .en suffix (as some tools expect)
		fmt.Sprintf("ggml-%s-q5_0.bin", modelSize),
		fmt.Sprintf("ggml-%s-q4_0.bin", modelSize),
	}

	// Check standard model locations
	modelLocations := []string{
		// Current directory
		".",
		// Models in application directory
		"models",
		// User's home directory
		filepath.Join(homeDir, ".local", "share", "ramble", "models"),
		filepath.Join(homeDir, ".ramble", "models"), // Also check the old path
		filepath.Join(homeDir, ".cache", "whisper"),
		// System-wide locations
		"/usr/local/share/whisper/models",
		"/usr/share/whisper/models",
		// Windows locations
		filepath.Join("C:\\", "Program Files", "whisper.cpp", "models"),
	}

	// Check all locations with all possible name formats
	for _, location := range modelLocations {
		for _, name := range possibleNames {
			path := filepath.Join(location, name)
			if _, err := os.Stat(path); err == nil {
				logger.Info(logger.CategoryTranscription, "Found existing model at: %s", path)
				return path
			}
		}
	}

	// Didn't find existing model, return path where we will download it
	downloadPath := filepath.Join(homeDir, ".local", "share", "ramble", "models", fmt.Sprintf("ggml-%s.en.bin", modelSize))
	return downloadPath
}
