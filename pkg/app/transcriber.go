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
	// Define a simple adapter to connect UI selectors to the transcription package
	execSelector := &ExecutableSelectorAdapter{selector}

	// Create the config from options
	config := transcription.Config{
		ModelSize:              options.ModelSize,
		ModelPath:              options.ModelPath,
		Language:               options.Language,
		Debug:                  options.Debug,
		PreferSystemExecutable: options.PreferSystemExecutable,
		Finder:                 transcription.DefaultConfig().Finder,
	}

	// If debugging, add additional logs
	if config.Debug {
		logger.Info(logger.CategoryTranscription, "Creating transcriber with debug mode enabled")
	}

	// Try to use embedded executable if available
	var execPath string
	var err error

	// First, try to use embedded executable from the embed package
	execPath, err = embed.GetWhisperExecutable()
	if err == nil {
		logger.Info(logger.CategoryTranscription, "Using embedded whisper executable")
		config.ExecutablePath = execPath
	} else {
		// If embedded executable not available, use system version with UI selector
		// Find all executables using the finder
		execs := config.Finder.FindAllExecutables()

		// If we have an explicit path, use it
		if config.ExecutablePath != "" {
			execPath = config.ExecutablePath
		} else if len(execs) > 1 {
			// If we have multiple executables, let the user select
			selected, err := execSelector.SelectExecutable(execs)
			if err == nil {
				config.ExecutablePath = selected
			} else {
				// If selection failed, use the first one
				config.ExecutablePath = execs[0]
			}
		} else if len(execs) == 1 {
			// If we have just one executable, use it
			config.ExecutablePath = execs[0]
		} else {
			// Try to install one if possible
			execPath, err = config.Finder.InstallExecutable()
			if err == nil {
				config.ExecutablePath = execPath
			}
		}
	}

	// Try to use embedded model if available
	modelPath, err := embed.ExtractModel(string(config.ModelSize))
	if err == nil {
		logger.Info(logger.CategoryTranscription, "Using embedded model: %s", config.ModelSize)
		config.ModelPath = modelPath
	}

	// Now create the transcriber with the configured executable
	return transcription.NewTranscriber(config)
}
