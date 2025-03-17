// Ramble is a cross-platform speech-to-text application
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jeff-barlow-spady/ramble/pkg/audio"
	"github.com/jeff-barlow-spady/ramble/pkg/hotkey"
	"github.com/jeff-barlow-spady/ramble/pkg/logger"
	"github.com/jeff-barlow-spady/ramble/pkg/transcription"
	"github.com/jeff-barlow-spady/ramble/pkg/ui"
)

// Application manages the overall application state and components
type Application struct {
	hotkeyDetector *hotkey.Detector
	audioRecorder  *audio.Recorder
	transcriber    transcription.Transcriber
	ui             *ui.App

	isDebugMode bool
	isListening bool
	mu          sync.Mutex
	exitChan    chan struct{}
	cleanupOnce sync.Once // Ensure cleanup only happens once
}

// NewApplication creates a new application instance
func NewApplication(debugMode bool) (*Application, error) {
	// Initialize UI with test mode
	uiApp := ui.NewWithOptions(debugMode)

	// Initialize hotkey detector with default config (Ctrl+Shift+S)
	hkConfig := hotkey.DefaultConfig()
	hkDetector := hotkey.NewDetector(hkConfig)

	// Initialize audio recorder with default config and debug mode
	audioConfig := audio.DefaultConfig()
	audioConfig.Debug = debugMode // Enable debug logs if in debug mode

	// Create the audio recorder
	var audioRecorder *audio.Recorder
	var err error

	// Create a proper audio recorder with debug mode
	audioRecorder, err = audio.NewRecorder(audioConfig)
	if err != nil {
		logger.Warning(logger.CategoryAudio, "Audio initialization issue: %v", err)
		logger.Info(logger.CategoryAudio, "You may need to configure your audio system or check permissions")
		// We'll continue with a nil recorder and handle it gracefully
	}

	// Initialize transcriber with default config
	transConfig := transcription.DefaultConfig()
	transConfig.Debug = debugMode // Enable debug logs if in debug mode

	// If running in debug mode, use a smaller model for faster loading
	if debugMode {
		transConfig.ModelSize = transcription.ModelTiny
	}

	// Create the transcriber
	transcriber, err := transcription.NewTranscriber(transConfig)
	if err != nil {
		logger.Error(logger.CategoryTranscription, "Failed to initialize transcriber: %v", err)
		// Continue with a nil transcriber - we'll handle this gracefully
	}

	app := &Application{
		ui:             uiApp,
		hotkeyDetector: hkDetector,
		audioRecorder:  audioRecorder,
		transcriber:    transcriber,
		isDebugMode:    debugMode,
		exitChan:       make(chan struct{}),
	}

	return app, nil
}

// Run starts the application event loop
func (a *Application) Run() error {
	// Set up UI callbacks
	a.ui.SetCallbacks(
		func() { a.startListening() },
		func() { a.stopListening() },
		func() { a.ui.UpdateTranscript("") },
	)

	// Set up quit callback with safe cleanup
	a.ui.SetQuitCallback(func() {
		a.SafeCleanup()
	})

	// Set up preferences callback
	a.ui.SetPreferencesCallback(func(prefs ui.Preferences) {
		a.handlePreferencesChanged(prefs)
	})

	// Start hotkey detection
	err := a.hotkeyDetector.Start(func() {
		// Toggle listening state
		a.mu.Lock()
		isListening := a.isListening
		a.mu.Unlock()

		if isListening {
			a.stopListening()
		} else {
			a.startListening()
		}
	})
	if err != nil {
		logger.Warning(logger.CategorySystem, "Failed to start hotkey detection: %v", err)
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		// Wait for either a signal or exitChan
		select {
		case <-sigChan:
			logger.Info(logger.CategoryApp, "Received shutdown signal")
		case <-a.exitChan:
			logger.Info(logger.CategoryApp, "Application exit requested")
		}

		// Perform cleanup once
		a.SafeCleanup()
	}()

	// Enter UI main loop (blocks until UI closes)
	a.ui.Run()

	return nil
}

// handlePreferencesChanged handles preference changes
func (a *Application) handlePreferencesChanged(prefs ui.Preferences) {
	if a.transcriber != nil {
		// Convert UI model selection to transcription model size
		var modelSize transcription.ModelSize
		switch prefs.ModelSize {
		case "tiny":
			modelSize = transcription.ModelTiny
		case "base":
			modelSize = transcription.ModelBase
		case "small":
			modelSize = transcription.ModelSmall
		case "medium":
			modelSize = transcription.ModelMedium
		case "large":
			modelSize = transcription.ModelLarge
		default:
			modelSize = transcription.ModelSmall
		}

		// Update transcriber configuration
		config := transcription.Config{
			ModelSize: modelSize,
			ModelPath: prefs.TranscriptPath, // Use transcript path for models if specified
			Language:  "",                   // Auto-detect language
			Debug:     a.isDebugMode,
		}

		// Update transcriber if possible
		if updater, ok := a.transcriber.(transcription.ConfigurableTranscriber); ok {
			err := updater.UpdateConfig(config)
			if err != nil {
				logger.Warning(logger.CategoryTranscription,
					"Failed to update transcriber configuration: %v", err)
			}
		}
	}
}

// SafeCleanup ensures cleanup is only performed once
func (a *Application) SafeCleanup() {
	a.cleanupOnce.Do(func() {
		// Stop listening if needed
		a.stopListening()

		// Clean up resources
		a.Cleanup()

		// Close exit channel
		close(a.exitChan)
	})
}

// startListening begins audio capture and transcription
func (a *Application) startListening() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isListening {
		return
	}

	// If we don't have an audio recorder, show an error and abort
	if a.audioRecorder == nil {
		errMsg := "Audio recorder not initialized, cannot start recording"
		logger.Error(logger.CategoryAudio, "%s", errMsg)
		a.ui.ShowTemporaryStatus(fmt.Sprintf("Error: %s", errMsg), 3*time.Second)

		// Show a more helpful error dialog
		a.ui.ShowErrorDialog("Audio Configuration Error",
			"Could not initialize audio recording.\n\n"+
				"Possible solutions:\n"+
				"1. Check if you have a microphone connected\n"+
				"2. Check audio permissions\n"+
				"3. Try using the provided asound.conf file:\n"+
				"   cp asound.conf ~/.asoundrc\n"+
				"4. Install PulseAudio: sudo apt-get install pulseaudio")
		return
	}

	// Start audio recording
	err := a.audioRecorder.Start(func(audioData []float32) {
		// Calculate audio level for visualization
		level := calculateRMSLevel(audioData)
		a.ui.UpdateAudioLevel(level)

		// Process audio data for transcription
		if a.transcriber != nil {
			text, err := a.transcriber.ProcessAudioChunk(audioData)
			if err != nil {
				logger.Error(logger.CategoryTranscription, "Transcription error: %v", err)
				return
			}

			if text != "" {
				// Update UI with transcribed text
				a.ui.AppendTranscript(text)
			}
		}
	})

	if err != nil {
		logger.Error(logger.CategoryAudio, "Failed to start audio recording: %v", err)
		a.ui.ShowTemporaryStatus(fmt.Sprintf("Error: %v", err), 3*time.Second)

		// Show a more detailed error dialog
		a.ui.ShowErrorDialog("Audio Recording Error",
			fmt.Sprintf("Error starting audio recording: %v\n\n"+
				"Try running the application with debug mode: ./ramble -debug\n"+
				"This will provide more detailed logs that may help diagnose the issue.", err))
		return
	}

	a.isListening = true
	a.ui.ShowTemporaryStatus("Started listening (Ctrl+Shift+S to stop)", 2*time.Second)
}

// calculateRMSLevel calculates audio level for visualization
func calculateRMSLevel(buffer []float32) float32 {
	if len(buffer) == 0 {
		return 0
	}

	// Calculate sum of squares
	var sumOfSquares float64
	for _, sample := range buffer {
		sumOfSquares += float64(sample * sample)
	}

	// Calculate RMS
	meanSquare := sumOfSquares / float64(len(buffer))
	rms := math.Sqrt(meanSquare)

	// Convert to a 0-1 range with some scaling to make it visually appealing
	// The 0.1 scaling factor is arbitrary and may need adjustment based on typical audio levels
	level := float32(rms * 5)
	if level > 1.0 {
		level = 1.0
	}

	return level
}

// stopListening ends audio capture and transcription
func (a *Application) stopListening() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isListening {
		return
	}

	// If we don't have an audio recorder, just update UI state
	if a.audioRecorder == nil {
		a.isListening = false
		return
	}

	// Stop audio recording
	err := a.audioRecorder.Stop()
	if err != nil {
		logger.Error(logger.CategoryAudio, "Failed to stop audio recording: %v", err)
	}

	a.isListening = false
	a.ui.ShowTemporaryStatus("Stopped listening (Ctrl+Shift+S to start)", 2*time.Second)
}

// Cleanup releases resources
func (a *Application) Cleanup() {
	a.stopListening()

	if a.audioRecorder != nil {
		a.audioRecorder.Terminate()
	}

	if a.transcriber != nil {
		a.transcriber.Close()
	}

	a.hotkeyDetector.Stop()
}

func main() {
	// Parse command-line flags
	debugMode := flag.Bool("debug", false, "Run in debug mode")
	useTUI := flag.Bool("t", false, "Use text-based UI instead of GUI")
	flag.Parse()

	// Initialize logger
	logger.Initialize()

	// Configure logger based on debug mode
	if *debugMode {
		logger.SetLevel(logger.LevelDebug)
		logger.Info(logger.CategoryApp, "Debug mode enabled - verbose logging active")
	} else {
		// In normal mode, suppress common ALSA warnings
		logger.SuppressASLAWarnings(true)
	}

	// Print startup message
	logger.Info(logger.CategoryApp, "Starting Ramble - Speech-to-Text Application")

	if *useTUI {
		logger.Info(logger.CategoryUI, "Using terminal-based UI")
		// Terminal UI support will go here
		// For now just show a placeholder message
		fmt.Println("Terminal UI support coming soon!")
		return
	}

	app, err := NewApplication(*debugMode)
	if err != nil {
		logger.Error(logger.CategoryApp, "Failed to initialize application: %v", err)
		os.Exit(1)
	}

	err = app.Run()
	if err != nil {
		logger.Error(logger.CategoryApp, "Application error: %v", err)
		os.Exit(1)
	}
}
