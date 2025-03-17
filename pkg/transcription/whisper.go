// Package transcription provides speech-to-text functionality
package transcription

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/jeff-barlow-spady/ramble/pkg/logger"
)

// Model size options for Whisper
type ModelSize string

const (
	ModelTiny   ModelSize = "tiny"
	ModelBase   ModelSize = "base"
	ModelSmall  ModelSize = "small"
	ModelMedium ModelSize = "medium"
	ModelLarge  ModelSize = "large"
)

// Config holds configuration for the transcriber
type Config struct {
	// Model size to use
	ModelSize ModelSize
	// Path to model files (if empty, uses default location)
	ModelPath string
	// Language code (if empty, auto-detects)
	Language string
	// Whether to enable debug logs
	Debug bool
	// Path to the executable (if empty, auto-detected)
	ExecutablePath string
	// Whether to prefer system-installed whisper executables over auto-installation
	PreferSystemExecutable bool
}

// DefaultConfig returns a reasonable default configuration
func DefaultConfig() Config {
	return Config{
		ModelSize:              ModelSmall, // Small is a good balance of accuracy and performance
		ModelPath:              "",         // Use default location
		Language:               "",         // Auto-detect language
		Debug:                  false,
		ExecutablePath:         "",   // Auto-detect executable
		PreferSystemExecutable: true, // By default, prefer system-installed executables
	}
}

// Transcriber interface defines methods for speech-to-text conversion
type Transcriber interface {
	// ProcessAudioChunk processes a chunk of audio data and returns transcribed text
	ProcessAudioChunk(audioData []float32) (string, error)
	// Close frees resources
	Close() error
}

// ConfigurableTranscriber is an enhanced transcriber that supports updating its configuration
type ConfigurableTranscriber interface {
	Transcriber
	// UpdateConfig updates the transcriber's configuration
	UpdateConfig(config Config) error
	// GetModelInfo returns information about the current model
	GetModelInfo() (ModelSize, string)
}

// NewTranscriber creates a new transcription service
func NewTranscriber(config Config) (Transcriber, error) {
	// Ensure model path is set
	if config.ModelPath == "" {
		config.ModelPath = getModelPath("", config.ModelSize)
	}

	// Ensure model directory exists
	if err := os.MkdirAll(config.ModelPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create model directory: %w", err)
	}

	// If a specific executable path is provided, use it directly
	if config.ExecutablePath != "" {
		if _, err := os.Stat(config.ExecutablePath); err == nil {
			logger.Info(logger.CategoryTranscription, "Using specified whisper executable: %s", config.ExecutablePath)
			return NewExecutableTranscriber(config)
		}
		return nil, fmt.Errorf("specified executable not found: %s", config.ExecutablePath)
	}

	// Try to find executable on the system
	execPath, err := findWhisperExecutable()
	systemExecFound := err == nil

	if systemExecFound {
		// We found a whisper executable
		execType := detectExecutableType(execPath)
		logger.Info(logger.CategoryTranscription,
			"Found system whisper executable: %s (type: %s)", execPath, getExecutableTypeName(execType))

		// If we prefer system executables or auto-install is not supported, use this executable
		if config.PreferSystemExecutable || !isWhisperInstallSupported() {
			config.ExecutablePath = execPath
			return NewExecutableTranscriber(config)
		}

		// Otherwise, we'll still keep it as a fallback option
	}

	// If auto-install is not supported or we explicitly prefer system executables,
	// we should not try to auto-install
	if !isWhisperInstallSupported() || (config.PreferSystemExecutable && !systemExecFound) {
		// If we can't find a system executable and can't/shouldn't auto-install,
		// return a placeholder transcriber
		logger.Warning(logger.CategoryTranscription,
			"No Whisper executable found and automatic installation not available or disabled")
		return &placeholderTranscriber{config: config}, nil
	}

	// Try to install whisper-cpp
	logger.Info(logger.CategoryTranscription, "Attempting to auto-install whisper.cpp...")
	autoExecPath, err := installWhisperExecutable()
	if err != nil {
		logger.Warning(logger.CategoryTranscription,
			"Failed to auto-install Whisper executable: %v", err)

		// If we found a system executable earlier, use it as fallback
		if systemExecFound {
			logger.Info(logger.CategoryTranscription,
				"Using system whisper executable as fallback: %s", execPath)
			config.ExecutablePath = execPath
			return NewExecutableTranscriber(config)
		}

		return &placeholderTranscriber{config: config}, nil
	}

	// Set the executable path in the config
	logger.Info(logger.CategoryTranscription, "Using auto-installed whisper executable: %s", autoExecPath)
	config.ExecutablePath = autoExecPath

	// Check if the model exists and download if needed
	modelPath, err := ensureModel(config)
	if err != nil {
		logger.Warning(logger.CategoryTranscription,
			"Failed to download model: %v", err)
		return &placeholderTranscriber{config: config}, nil
	}

	// Update config with resolved model path
	config.ModelPath = modelPath

	// Create a new executable transcriber
	return NewExecutableTranscriber(config)
}

// ensureModel checks if model exists and downloads it if needed
func ensureModel(config Config) (string, error) {
	modelPath := getModelPath(config.ModelPath, config.ModelSize)

	// Download the model if needed
	modelFile, err := DownloadModel(modelPath, config.ModelSize)
	if err != nil {
		return "", fmt.Errorf("failed to ensure model: %w", err)
	}

	return modelFile, nil
}

// isWhisperInstallSupported returns true if this platform supports auto-install
func isWhisperInstallSupported() bool {
	// Currently we only support auto-install on limited platforms
	// This could be expanded in the future
	return runtime.GOOS == "linux" && runtime.GOARCH == "amd64"
}

// installWhisperExecutable tries to install the whisper executable
func installWhisperExecutable() (string, error) {
	logger.Info(logger.CategoryTranscription, "Attempting to download and install whisper.cpp executable")

	// Create installation directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	installDir := filepath.Join(homeDir, ".local", "share", "ramble", "bin")
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create installation directory: %w", err)
	}

	// Target executable path
	execPath := filepath.Join(installDir, "whisper-cpp")

	// Potential download URLs in order of preference
	// We try specific release URLs first, then fall back to latest
	downloadURLs := []string{
		// Specific version URLs
		"https://github.com/ggerganov/whisper.cpp/releases/download/v1.5.0/whisper-linux-x64",
		"https://github.com/ggerganov/whisper.cpp/releases/download/v1.4.2/whisper-linux-x64",
		// Latest release fallback
		"https://github.com/ggerganov/whisper.cpp/releases/latest/download/whisper-linux-x64",
	}

	// Try each URL until one works
	var dlError error
	for _, downloadURL := range downloadURLs {
		logger.Info(logger.CategoryTranscription, "Attempting to download whisper.cpp from %s", downloadURL)

		err := downloadExecutable(downloadURL, execPath)
		if err == nil {
			// Successful download
			logger.Info(logger.CategoryTranscription, "Successfully installed whisper.cpp to %s", execPath)
			return execPath, nil
		}

		dlError = err
		logger.Warning(logger.CategoryTranscription, "Failed to download from %s: %v", downloadURL, err)
	}

	// If we get here, all downloads failed
	return "", fmt.Errorf("all download attempts failed, last error: %w", dlError)
}

// downloadExecutable downloads an executable from a URL and saves it to the specified path
func downloadExecutable(url string, destPath string) error {
	// Download the executable
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP status error: %s", resp.Status)
	}

	// Create output file
	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY, 0755) // Make it executable
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer out.Close()

	// Copy the data
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		// Clean up on error
		os.Remove(destPath)
		return fmt.Errorf("failed to save executable: %w", err)
	}

	return nil
}

// placeholderTranscriber is a simple implementation that doesn't do real transcription
type placeholderTranscriber struct {
	config Config
	mu     sync.Mutex
}

// ProcessAudioChunk returns a placeholder message
func (t *placeholderTranscriber) ProcessAudioChunk(audioData []float32) (string, error) {
	// Only return a message occasionally to avoid flooding the UI
	if len(audioData) > 16000*3 { // Only for chunks > 3 seconds
		return "Speech transcription placeholder (Whisper not available)", nil
	}
	return "", nil
}

// Close does nothing for the placeholder
func (t *placeholderTranscriber) Close() error {
	return nil
}

// UpdateConfig updates the configuration
func (t *placeholderTranscriber) UpdateConfig(config Config) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.config = config
	return nil
}

// GetModelInfo returns information about the current model
func (t *placeholderTranscriber) GetModelInfo() (ModelSize, string) {
	return t.config.ModelSize, "placeholder"
}

// getExecutableTypeName returns a string representation of the executable type
func getExecutableTypeName(execType ExecutableType) string {
	switch execType {
	case ExecutableTypeWhisperCpp:
		return "whisper.cpp"
	case ExecutableTypeWhisperGael:
		return "whisper-gael (Python)"
	default:
		return "unknown"
	}
}
