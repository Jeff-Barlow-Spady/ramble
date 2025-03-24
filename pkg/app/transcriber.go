// Package app contains the core application logic
package app

import (
	"github.com/jeff-barlow-spady/ramble/pkg/logger"
	"github.com/jeff-barlow-spady/ramble/pkg/transcription"
	"github.com/jeff-barlow-spady/ramble/pkg/transcription/embed"
	"github.com/jeff-barlow-spady/ramble/pkg/ui"
)

// TranscriberOptions contains options for creating a transcriber
type TranscriberOptions struct {
	Debug                  bool
	ModelSize              transcription.ModelSize
	ModelPath              string
	Language               string
	PreferSystemExecutable bool
}

// DefaultTranscriberOptions returns the default options for creating a transcriber
func DefaultTranscriberOptions() TranscriberOptions {
	return TranscriberOptions{
		Debug:                  false,
		ModelSize:              transcription.ModelTiny,
		Language:               "",
		PreferSystemExecutable: true,
	}
}

// ExecutableSelectorAdapter adapts UI selectors to transcription.ExecutableSelector
type ExecutableSelectorAdapter struct {
	UISelector ui.ExecutableSelectorUI
}

// SelectExecutable implements the transcription.ExecutableSelector interface
func (a *ExecutableSelectorAdapter) SelectExecutable(executables []string) (string, error) {
	return a.UISelector.SelectExecutable(executables)
}

// CreateTranscriber creates a transcriber with the given options and UI selector
func CreateTranscriber(options TranscriberOptions, selector ui.ExecutableSelectorUI) (transcription.Transcriber, error) {
	// Create the config from options
	config := transcription.Config{
		ModelSize: options.ModelSize,
		ModelPath: options.ModelPath,
		Language:  options.Language,
		Debug:     options.Debug,
	}

	// If debugging, add additional logs
	if config.Debug {
		logger.Info(logger.CategoryTranscription, "Creating transcriber with debug mode enabled")
	}

	// Try to use embedded model if available
	modelPath, err := embed.ExtractModel(string(config.ModelSize))
	if err == nil {
		logger.Info(logger.CategoryTranscription, "Using embedded model: %s", config.ModelSize)
		config.ModelPath = modelPath
	}

	// Create the transcriber with Go bindings
	return transcription.NewTranscriber(config)
}
