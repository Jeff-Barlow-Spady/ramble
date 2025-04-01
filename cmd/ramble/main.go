// Package main is the entry point for the Ramble application
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jeff-barlow-spady/ramble/pkg/audio"
	"github.com/jeff-barlow-spady/ramble/pkg/logger"
	"github.com/jeff-barlow-spady/ramble/pkg/transcription"
	"github.com/jeff-barlow-spady/ramble/pkg/ui"
)

// Comment out the embedded model code for now since the directory doesn't exist
// Uncomment and use this when you want to embed models in the binary
// //go:embed models/ggml-small.bin
// var embeddedModel []byte

// App represents the main application
type App struct {
	ui          *ui.App
	transcriber *transcription.WhisperTranscriber
	audio       *audio.Capture
	debug       bool
	mu          sync.Mutex
	fullText    string
}

// New creates a new application instance
func New(debug bool) (*App, error) {
	// Initialize components
	app := &App{
		debug:    debug,
		fullText: "",
	}

	// Setup UI
	app.ui = ui.NewWithOptions(debug)
	app.ui.SetCallbacks(
		app.startRecording,
		app.stopRecording,
		func() {
			// Handle clear transcript
			app.mu.Lock()
			app.fullText = ""
			app.mu.Unlock()
		},
	)

	// Find model path
	modelPath := transcription.GetLocalModelPath(transcription.ModelTiny)
	if modelPath == "" {
		return nil, fmt.Errorf("could not find a valid model file")
	}

	// Setup transcriber using the Manager directly
	transcriber, err := transcription.NewManager(modelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize transcriber: %w", err)
	}
	app.transcriber = transcriber

	// Setup audio capture
	capture, err := audio.New(16000, debug)
	if err != nil {
		app.transcriber.Close()
		return nil, fmt.Errorf("failed to initialize audio: %w", err)
	}
	app.audio = capture

	// Set up transcript callback
	app.transcriber.SetStreamingCallback(func(text string) {
		// Normalize text before displaying
		normalizedText := transcription.NormalizeTranscriptionText(text)
		if normalizedText != "" {
			// Use the new session accumulation method to build session text
			app.ui.AppendSessionText(normalizedText)

			// Store the text for later
			app.appendToFullText(normalizedText)

			// The finalization only happens when recording stops, not on a timer
			// So we don't need to reset a timer here
		}
	})

	return app, nil
}

// Run starts the application
func (a *App) Run() {
	// Run the UI
	a.ui.Run()
}

// startRecording begins audio capture and transcription
func (a *App) startRecording() {
	// Reset the transcript for a new recording
	a.mu.Lock()
	a.fullText = ""
	a.mu.Unlock()

	// Clear the UI for the new recording session
	a.ui.UpdateTranscript("")
	a.ui.UpdateStreamingPreview("")

	// Start the transcriber
	a.transcriber.SetRecordingState(true)
	a.ui.ShowTemporaryStatus("Starting recording...", 2*time.Second)

	// Start audio capture with callback
	err := a.audio.Start(func(samples []float32) {
		// Calculate audio level for visualization
		level := audio.CalculateLevel(samples)
		a.ui.UpdateAudioLevel(level)

		// Process audio through transcriber
		_, err := a.transcriber.ProcessAudioChunk(samples)
		if err != nil {
			logger.Error(logger.CategoryTranscription, "Error processing audio: %v", err)
		}
	})

	if err != nil {
		logger.Error(logger.CategoryAudio, "Failed to start recording: %v", err)
		a.ui.ShowTemporaryStatus(fmt.Sprintf("Error: %v", err), 3*time.Second)
		a.stopRecording()
		return
	}

	a.ui.SetState(ui.StateListening)
}

// stopRecording ends audio capture and transcription
func (a *App) stopRecording() {
	// Stop audio capture
	if a.audio != nil && a.audio.IsActive() {
		if err := a.audio.Stop(); err != nil {
			logger.Error(logger.CategoryAudio, "Error stopping audio: %v", err)
		}
	}

	// Stop transcriber
	if a.transcriber != nil {
		a.transcriber.SetRecordingState(false)
	}

	// Finalize current session
	a.ui.FinalizeTranscriptionSegment()

	a.ui.SetState(ui.StateIdle)
}

// appendToFullText adds text to the complete transcript
func (a *App) appendToFullText(text string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.fullText != "" {
		a.fullText += " "
	}
	a.fullText += text
}

// Close performs cleanup
func (a *App) Close() {
	a.stopRecording()

	if a.transcriber != nil {
		a.transcriber.Close()
	}

	if a.audio != nil {
		a.audio.Close()
	}
}

/*
// Comment out the loadEmbeddedModel function until we actually implement embedded models
func loadEmbeddedModel() ([]byte, error) {
	// Logic to potentially write embeddedModel to a temporary file
	// if the whisper library needs a file path, or use a memory-based
	// loading function if the library supports it.
	// For ggml libraries, often a file path is needed.
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, "ramble-embedded-model.bin")
	err := os.WriteFile(tmpFile, embeddedModel, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write embedded model to temp file: %w", err)
	}
	// Return the path to the temp file
	// Remember to clean up this file on exit
	return []byte(tmpFile), nil // Returning path as bytes for example
}
*/

func main() {
	// Parse command line flags
	debug := flag.Bool("debug", false, "Enable debug output")
	flag.Parse()

	// Configure logger based on debug flag
	if *debug {
		logger.SetLevel(logger.LevelDebug)
	}
	logger.Info(logger.CategoryApp, "Starting Ramble - Speech to Text")

	// Create and run the application
	app, err := New(*debug)
	if err != nil {
		logger.Error(logger.CategoryApp, "Failed to initialize application: %v", err)
		os.Exit(1)
	}

	// Handle termination signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		logger.Info(logger.CategoryApp, "Shutting down...")
		app.Close()
		os.Exit(0)
	}()

	// Run the application
	app.Run()
}
