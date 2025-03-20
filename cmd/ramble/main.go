// Ramble is a cross-platform speech-to-text application
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jeff-barlow-spady/ramble/pkg/audio"
	"github.com/jeff-barlow-spady/ramble/pkg/hotkey"
	"github.com/jeff-barlow-spady/ramble/pkg/logger"
	"github.com/jeff-barlow-spady/ramble/pkg/transcription"
	"github.com/jeff-barlow-spady/ramble/pkg/ui"
)

// Package-level variable for debounce
var lastStartTime time.Time

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
	transConfig.Debug = debugMode             // Enable debug logs if in debug mode
	transConfig.PreferSystemExecutable = true // Prefer system-installed executable

	// No need to change model size in debug mode - use the default (small)
	// or previously selected model

	// Create the transcriber
	transcriber, err := transcription.NewTranscriber(transConfig)
	if err != nil {
		logger.Error(logger.CategoryTranscription, "Failed to initialize transcriber: %v", err)
		// Continue with a nil transcriber - we'll handle this gracefully
	}

	// Update transcriber configuration
	config := transcription.Config{
		ModelSize: transConfig.ModelSize,
		ModelPath: transConfig.ModelPath,
		Language:  "", // Auto-detect language
		Debug:     debugMode,
	}

	// Update transcriber if possible
	if updater, ok := transcriber.(transcription.ConfigurableTranscriber); ok {
		err := updater.UpdateConfig(config)
		if err != nil {
			logger.Warning(logger.CategoryTranscription,
				"Failed to update transcriber configuration: %v", err)
		}
	}

	app := &Application{
		ui:             uiApp,
		hotkeyDetector: hkDetector,
		audioRecorder:  audioRecorder,
		transcriber:    transcriber,
		isDebugMode:    debugMode,
		exitChan:       make(chan struct{}),
	}

	// Set up streaming callback if the transcriber supports it
	if confTranscriber, ok := transcriber.(transcription.ConfigurableTranscriber); ok {
		logger.Info(logger.CategoryTranscription, "Setting up improved streaming callback")

		// Keep track of processing state
		var processingMutex sync.Mutex
		isProcessing := false
		processingStartTime := time.Time{}
		var lastText string

		// Get model info
		modelSize, _ := confTranscriber.GetModelInfo()
		logger.Info(logger.CategoryTranscription, "Using model size: %s", modelSize)

		confTranscriber.SetStreamingCallback(func(text string) {
			// Handle special status messages
			if text == "Processing audio..." {
				processingMutex.Lock()
				isProcessing = true
				processingStartTime = time.Now()
				processingMutex.Unlock()

				// Show a more prominent processing indicator
				ui.RunOnMain(func() {
					uiApp.ShowTemporaryStatus("Processing speech...", 2*time.Second)
				})
				return
			}

			// Handle completion messages
			if text == "No speech detected" || text == "Transcription timeout - try again" || text == "Processing complete" {
				processingMutex.Lock()
				isProcessing = false
				processMsec := time.Since(processingStartTime).Milliseconds()
				processingMutex.Unlock()

				// Log processing time
				logger.Info(logger.CategoryTranscription, "Processing completed in %d ms with result: %s", processMsec, text)

				// Show a more informative message if no speech was detected
				ui.RunOnMain(func() {
					if text == "No speech detected" {
						// Show user-friendly message for no speech
						uiApp.ShowTemporaryStatus("No speech detected", 1*time.Second)
					} else if text == "Processing complete" {
						// For streaming, just show a subtle indicator that a chunk processed
						if lastText != "" {
							uiApp.ShowTemporaryStatus("✓", 500*time.Millisecond)
						}
					} else {
						// Clear any processing status
						uiApp.ShowTemporaryStatus("", 0)
					}
				})
				return
			}

			// For normal transcription text, update the UI immediately
			if text != "" && !strings.HasPrefix(text, "[") && !strings.HasPrefix(text, "(") {
				logger.Info(logger.CategoryTranscription, "Got streaming text: %s", text)

				// Save the last text
				lastText = text

				// Update the UI with the transcription in the main thread
				ui.RunOnMain(func() {
					uiApp.AppendTranscript(text)
					// Show a brief success indicator
					uiApp.ShowTemporaryStatus("✓", 500*time.Millisecond)
				})

				// Indicate processing is done for now
				processingMutex.Lock()
				isProcessing = false
				processingMutex.Unlock()
			}
		})

		// Start a background goroutine to provide periodic updates during processing
		go func() {
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()

			for {
				<-ticker.C

				processingMutex.Lock()
				if isProcessing {
					elapsedTime := time.Since(processingStartTime)
					// Only show processing time if it's taking more than 1.5 seconds
					if elapsedTime.Seconds() > 1.5 {
						// Update the UI with the processing time
						ui.RunOnMain(func() {
							uiApp.ShowTemporaryStatus(fmt.Sprintf("Processing speech... (%.1f sec)", elapsedTime.Seconds()), 500*time.Millisecond)
						})
					}
				}
				processingMutex.Unlock()

				// Check if we should exit this goroutine
				select {
				case <-app.exitChan:
					return
				default:
					// Continue
				}
			}
		}()
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
	// Use mutex to prevent race conditions
	a.mu.Lock()

	// Debounce protection - don't start again if we've started recently
	if !lastStartTime.IsZero() && time.Since(lastStartTime) < 3*time.Second {
		logger.Warning(logger.CategoryAudio, "Ignoring start request - too soon since last start (%v)", time.Since(lastStartTime))
		a.mu.Unlock()
		return
	}

	// Update timestamp for debounce
	lastStartTime = time.Now()

	// Check if we're already listening
	if a.isListening {
		logger.Warning(logger.CategoryAudio, "Already listening, ignoring start request")
		a.mu.Unlock()
		return
	}

	// Mark as listening at the start of function to prevent race conditions
	a.isListening = true
	a.mu.Unlock()

	// Show a clear status message so users know recording has started
	a.ui.ShowTemporaryStatus("STREAMING ACTIVE - speak naturally", 3*time.Second)

	// Clear any previous text in the transcription area
	a.ui.UpdateTranscript("")

	// Log that we're manually starting to listen
	logger.Info(logger.CategoryAudio, "STREAMING MODE: User initiated recording")

	// Display a startup message in the transcription area to provide immediate feedback
	a.ui.AppendTranscript("Streaming active... (speak naturally, transcription appears continuously)\n\n")

	// Start audio recorder if needed
	if a.audioRecorder == nil {
		var err error
		a.audioRecorder, err = audio.NewRecorder(audio.DefaultConfig())
		if err != nil {
			logger.Error(logger.CategoryAudio, "Failed to create audio recorder: %v", err)
			a.isListening = false
			a.ui.ShowTemporaryStatus("Error: Failed to start recording", 3*time.Second)
			return
		}
	}

	// Set up a safety catch to automatically stop if something goes wrong
	safetyCleanup := func() {
		// If we're still listening after 60 seconds, force stop
		// Extended from 30s to 60s for longer streaming sessions
		time.AfterFunc(60*time.Second, func() {
			a.mu.Lock()
			isStillListening := a.isListening
			a.mu.Unlock()

			if isStillListening {
				logger.Warning(logger.CategoryAudio, "SAFETY CHECK: Still listening after 60 seconds, force stopping")
				a.stopListening()
				if a.ui != nil {
					a.ui.ShowTemporaryStatus("Streaming timeout - press Record to start again", 3*time.Second)
				}
			}
		})
	}

	// Define an audio callback to process the audio data
	audioCallback := func(audioData []float32) {
		a.mu.Lock()
		// Only process if we're still listening
		if !a.isListening {
			a.mu.Unlock()
			return
		}
		a.mu.Unlock()

		// Calculate audio level
		level := calculateRMSLevel(audioData)
		a.ui.UpdateAudioLevel(level)

		// Process audio data for transcription
		// Note: This will not block since we've modified the ProcessAudioChunk method
		// to handle all processing asynchronously in streaming mode
		if a.transcriber != nil && level > 0.005 { // Even lower threshold for better streaming sensitivity
			_, err := a.transcriber.ProcessAudioChunk(audioData)
			if err != nil {
				logger.Error(logger.CategoryTranscription, "Error processing audio: %v", err)
			}
		}
	}

	// Start recording with the callback
	err := a.audioRecorder.Start(audioCallback)
	if err != nil {
		logger.Error(logger.CategoryAudio, "Failed to start audio recording: %v", err)
		a.isListening = false
		a.ui.ShowTemporaryStatus("Error: Failed to start recording", 3*time.Second)
		return
	}

	// Activate safety cleanup
	safetyCleanup()
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

// stopListening stops audio capture and transcription
func (a *Application) stopListening() {
	a.mu.Lock()
	// Only do something if we're actually listening
	wasListening := a.isListening
	a.isListening = false
	// Reset lastStartTime to allow immediate re-recording
	lastStartTime = time.Time{}
	a.mu.Unlock()

	// Skip if we weren't listening
	if !wasListening {
		logger.Warning(logger.CategoryAudio, "stopListening called when not listening")
		return
	}

	// Log that we're about to stop
	logger.Info(logger.CategoryAudio, "Stopping audio recording")

	// Show a message that we're stopping - helps provide immediate feedback
	a.ui.ShowTemporaryStatus("Stopping recording...", 1*time.Second)

	// Stop the audio recorder
	if a.audioRecorder != nil {
		err := a.audioRecorder.Stop()
		if err != nil {
			logger.Error(logger.CategoryAudio, "Failed to stop audio recording: %v", err)
		}
	}

	// Reset UI state with clear feedback
	if a.ui != nil {
		// Make sure the UI reflects that we've stopped listening
		a.ui.ShowTemporaryStatus("Recording stopped", 2*time.Second)

		// Append a note to the transcript to indicate recording has stopped
		// This helps users understand the state of the application
		a.ui.AppendTranscript("\n\n(Recording stopped)")
	}
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
