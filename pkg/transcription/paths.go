// Package transcription provides speech-to-text functionality
package transcription

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/jeff-barlow-spady/ramble/pkg/logger"
)

// getModelPath resolves the path to model files
// It first checks the provided path, then falls back to standard locations
func getModelPath(configPath string, modelSize ModelSize) string {
	// If a path is provided and exists, use it
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}
	}

	// Get default path and check if model exists
	defaultPath := getDefaultModelPath()

	// Get model filename
	modelFilename, ok := WhisperModelFilenames[modelSize]
	if !ok {
		logger.Warning(logger.CategoryTranscription, "Unknown model size: %s, using tiny", modelSize)
		modelFilename = WhisperModelFilenames[ModelTiny]
	}

	// Check if model exists in default path
	modelPath := filepath.Join(defaultPath, modelFilename)
	if _, err := os.Stat(modelPath); err == nil {
		return defaultPath
	}

	// Create default path if it doesn't exist
	if err := os.MkdirAll(defaultPath, 0755); err == nil {
		return defaultPath
	}

	// Fallback to current directory
	return "."
}

// getDefaultModelPath returns the default path for model files
func getDefaultModelPath() string {
	// Try to use a standard location based on the OS
	var baseDirs []string

	homeDir, err := os.UserHomeDir()
	if err == nil {
		switch runtime.GOOS {
		case "windows":
			baseDirs = append(baseDirs, filepath.Join(homeDir, "AppData", "Local", "Ramble", "models"))
		case "darwin":
			baseDirs = append(baseDirs, filepath.Join(homeDir, "Library", "Application Support", "Ramble", "models"))
		default: // Linux, BSD, etc.
			baseDirs = append(baseDirs, filepath.Join(homeDir, ".local", "share", "ramble", "models"))
			baseDirs = append(baseDirs, filepath.Join(homeDir, ".ramble", "models"))
		}
	}

	// Add common system-wide locations
	switch runtime.GOOS {
	case "windows":
		baseDirs = append(baseDirs, filepath.Join("C:", "Program Files", "Ramble", "models"))
	case "darwin":
		baseDirs = append(baseDirs, "/Applications/Ramble.app/Contents/Resources/models")
	default: // Linux, BSD, etc.
		baseDirs = append(baseDirs, "/usr/local/share/ramble/models")
		baseDirs = append(baseDirs, "/usr/share/ramble/models")
	}

	// Check if any of these directories exist and contain models
	for _, dir := range baseDirs {
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
	}

	// None of the standard locations exist, use the first one as default
	if len(baseDirs) > 0 {
		return baseDirs[0]
	}

	// Last resort: use the current directory
	return "models"
}

// findWhisperExecutable searches for the whisper executable in standard locations
func findWhisperExecutable() (string, error) {
	// First check PATH for whisper or whisper.cpp
	exeNames := []string{"whisper", "whisper.cpp", "whisper-cpp", "main"}

	// Add OS-specific executable extensions
	if runtime.GOOS == "windows" {
		for i, name := range exeNames {
			exeNames[i] = name + ".exe"
		}
	}

	// Check for the executable in PATH
	for _, name := range exeNames {
		path, err := exec.LookPath(name)
		if err == nil {
			logger.Info(logger.CategoryTranscription, "Found whisper executable in PATH: %s", path)
			return path, nil
		}
	}

	// Check standard install locations
	searchDirs := []string{"."}

	homeDir, err := os.UserHomeDir()
	if err == nil {
		// Add user-specific directories
		searchDirs = append(searchDirs, filepath.Join(homeDir, "bin"))

		switch runtime.GOOS {
		case "windows":
			searchDirs = append(searchDirs, filepath.Join(homeDir, "AppData", "Local", "Ramble", "bin"))
		case "darwin":
			searchDirs = append(searchDirs, filepath.Join(homeDir, "Library", "Application Support", "Ramble", "bin"))
		default: // Linux, BSD, etc.
			searchDirs = append(searchDirs, filepath.Join(homeDir, ".local", "bin"))
			searchDirs = append(searchDirs, filepath.Join(homeDir, ".ramble", "bin"))
		}
	}

	// Add system directories
	switch runtime.GOOS {
	case "windows":
		searchDirs = append(searchDirs, filepath.Join("C:", "Program Files", "Ramble", "bin"))
	case "darwin":
		searchDirs = append(searchDirs, "/Applications/Ramble.app/Contents/Resources/bin")
		searchDirs = append(searchDirs, "/usr/local/bin")
	default: // Linux, BSD, etc.
		searchDirs = append(searchDirs, "/usr/local/bin")
		searchDirs = append(searchDirs, "/usr/bin")
		searchDirs = append(searchDirs, "/usr/local/share/ramble/bin")
		searchDirs = append(searchDirs, "/usr/share/ramble/bin")
	}

	// Search for the executable in all potential locations
	for _, dir := range searchDirs {
		for _, name := range exeNames {
			path := filepath.Join(dir, name)
			if _, err := os.Stat(path); err == nil {
				// Check if the file is executable
				if isExecutable(path) {
					logger.Info(logger.CategoryTranscription, "Found whisper executable: %s", path)
					return path, nil
				}
			}
		}
	}

	logger.Error(logger.CategoryTranscription, "Whisper executable not found")
	return "", fmt.Errorf("whisper executable not found in standard locations")
}

// isExecutable checks if a file has execute permissions
func isExecutable(path string) bool {
	// On Windows, all files are executable
	if runtime.GOOS == "windows" {
		return true
	}

	// On Unix systems, check execute permissions
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return (info.Mode().Perm() & 0111) != 0 // Check if any execute bit is set
}

// ensureExecutablePath ensures the executable path is valid
func ensureExecutablePath(config Config) (string, error) {
	if config.ExecutablePath != "" {
		// Check if the specified executable exists and is executable
		if _, err := os.Stat(config.ExecutablePath); err == nil {
			if isExecutable(config.ExecutablePath) {
				return config.ExecutablePath, nil
			}
			return "", fmt.Errorf("specified executable is not executable: %s", config.ExecutablePath)
		}
		return "", fmt.Errorf("specified executable not found: %s", config.ExecutablePath)
	}

	// Try to find the executable
	return findWhisperExecutable()
}
