package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/jeff-barlow-spady/ramble/pkg/logger"
	"github.com/jeff-barlow-spady/ramble/pkg/transcription"
)

func main() {
	// Command line flags
	preferSystem := flag.Bool("system", true, "Prefer system-installed whisper executables")
	autoInstall := flag.Bool("auto-install", false, "Force auto-installation even if system executable is found")
	debug := flag.Bool("debug", false, "Enable debug logging")
	customExec := flag.String("exec", "", "Path to a specific whisper executable")
	modelSize := flag.String("model", "small", "Model size (tiny, base, small, medium, large)")

	flag.Parse()

	// Set up logging
	logger.Initialize()
	if *debug {
		logger.SetLevel(logger.LevelDebug)
	}
	logger.Info(logger.CategoryApp, "Starting transcription test")

	// Parse model size
	var model transcription.ModelSize
	switch *modelSize {
	case "tiny":
		model = transcription.ModelTiny
	case "base":
		model = transcription.ModelBase
	case "small":
		model = transcription.ModelSmall
	case "medium":
		model = transcription.ModelMedium
	case "large":
		model = transcription.ModelLarge
	default:
		logger.Warning(logger.CategoryApp, "Unknown model size: %s, using small", *modelSize)
		model = transcription.ModelSmall
	}

	// Create a config
	config := transcription.DefaultConfig()
	config.Debug = *debug
	config.ModelSize = model
	config.PreferSystemExecutable = *preferSystem

	if !*preferSystem {
		logger.Info(logger.CategoryApp, "Auto-installation mode: will try to download and install whisper.cpp")
	} else {
		logger.Info(logger.CategoryApp, "System preference mode: will prefer system-installed whisper executables")
	}

	if *customExec != "" {
		logger.Info(logger.CategoryApp, "Using custom executable: %s", *customExec)
		config.ExecutablePath = *customExec
	}

	// Force auto-install if requested
	if *autoInstall {
		logger.Info(logger.CategoryApp, "Forcing auto-installation")
		config.PreferSystemExecutable = false
		config.ExecutablePath = "" // Clear any executable path to force auto-detection
	}

	// Create a transcriber
	logger.Info(logger.CategoryApp, "Creating transcriber with model: %s", config.ModelSize)
	transcriber, err := transcription.NewTranscriber(config)
	if err != nil {
		logger.Error(logger.CategoryApp, "Failed to create transcriber: %v", err)
		os.Exit(1)
	}
	defer transcriber.Close()

	// Check if the transcriber is a configurable one
	if configurable, ok := transcriber.(transcription.ConfigurableTranscriber); ok {
		// Get model info
		modelSize, modelPath := configurable.GetModelInfo()
		logger.Info(logger.CategoryApp, "Using model: %s at %s", modelSize, modelPath)
	}

	// Print message about test
	logger.Info(logger.CategoryApp, "Transcription service initialized. Press Ctrl+C to exit.")
	logger.Info(logger.CategoryApp, "Test was successful if no errors occurred.")

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Keep the program running until signal is received
	<-sigCh
	logger.Info(logger.CategoryApp, "Shutting down...")
}
