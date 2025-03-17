// Package transcription provides speech-to-text functionality
package transcription

import (
	"fmt"
	"os"
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
}

// DefaultConfig returns a reasonable default configuration
func DefaultConfig() Config {
	return Config{
		ModelSize:      ModelSmall, // Small is a good balance of accuracy and performance
		ModelPath:      "",         // Use default location
		Language:       "",         // Auto-detect language
		Debug:          false,
		ExecutablePath: "", // Auto-detect executable
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
		config.ModelPath = getDefaultModelPath()
	}

	// Ensure model directory exists
	if err := os.MkdirAll(config.ModelPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create model directory: %w", err)
	}

	// First check if we can use the executable transcriber
	if config.ExecutablePath == "" {
		// Try to find executable
		if execPath, err := findWhisperExecutable(); err == nil {
			// We found a whisper executable, use it
			execType := detectExecutableType(execPath)
			logger.Info(logger.CategoryTranscription,
				"Found whisper executable: %s (type: %s)", execPath, getExecutableTypeName(execType))

			config.ExecutablePath = execPath
		}
	}

	// If we have an executable path, use it
	if config.ExecutablePath != "" {
		return NewExecutableTranscriber(config)
	}

	// No executable found, check if platform is supported for auto-install
	if !isWhisperInstallSupported() {
		// If we can't use or find a whisper executable and can't install one,
		// return a placeholder transcriber
		logger.Warning(logger.CategoryTranscription,
			"No Whisper executable found and automatic installation not supported on %s",
			runtime.GOOS)
		return &placeholderTranscriber{config: config}, nil
	}

	// Try to install whisper-cpp
	execPath, err := installWhisperExecutable()
	if err != nil {
		logger.Warning(logger.CategoryTranscription,
			"Failed to install Whisper executable: %v", err)
		return &placeholderTranscriber{config: config}, nil
	}

	// Set the executable path in the config
	config.ExecutablePath = execPath

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
// This is a simple placeholder - a real implementation would download
// precompiled binaries for the current platform
func installWhisperExecutable() (string, error) {
	// For now, just return an error
	// In a real implementation, this would download and extract
	// precompiled binaries or build from source
	return "", fmt.Errorf("automatic installation not implemented yet")
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
