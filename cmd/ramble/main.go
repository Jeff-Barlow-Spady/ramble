// Package ramble is a cross-platform speech-to-text application
package main

import (
	"flag"
	"fmt"
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
	"github.com/jeff-barlow-spady/ramble/pkg/transcription/embed"
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

	// Track last audio level for inactivity detection
	lastAudioLevel float32
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
		lastAudioLevel: 0,
	}

	// Set up streaming callback if the transcriber supports it
	if confTranscriber, ok := transcriber.(transcription.ConfigurableTranscriber); ok {
		logger.Info(logger.CategoryTranscription, "Setting up improved streaming callback")

		// Keep track of processing state
		var processingMutex sync.Mutex
		isProcessing := false
		processingStartTime := time.Time{}

		// Get model info
		modelSize, _ := confTranscriber.GetModelInfo()
		logger.Info(logger.CategoryTranscription, "Using model size: %s", modelSize)

		confTranscriber.SetStreamingCallback(func(text string) {
			logger.Info(logger.CategoryTranscription, "Got streaming text: %s", text)

			// Don't ignore or filter out actual transcription text
			// Only skip special tokens in brackets that aren't actual speech
			if strings.HasPrefix(text, "[") && strings.HasSuffix(text, "]") {
				// Special token - skip tokens like [BLANK_AUDIO] but keep meaningful ones
				if text == "[BLANK_AUDIO]" {
					logger.Info(logger.CategoryTranscription, "Skipping special token: %s", text)
					return
				}
			}

			// Process valid transcription text
			logger.Info(logger.CategoryTranscription, "Updating transcript with: %s", text)
			if app.ui != nil {
				ui.RunOnMain(func() {
					app.ui.AppendTranscript(text)
				})
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
	a.mu.Lock()
	// If already listening, do nothing (debounce)
	if a.isListening {
		a.mu.Unlock()
		logger.Info(logger.CategoryAudio, "Already listening, ignoring start request")
		return
	}

	// Implement a cooldown to prevent rapid start/stop
	// If less than 500ms have passed since the last start, ignore this request
	now := time.Now()
	if !lastStartTime.IsZero() && now.Sub(lastStartTime) < 500*time.Millisecond {
		a.mu.Unlock()
		logger.Info(logger.CategoryAudio, "Ignoring start request due to cooldown period")
		return
	}
	lastStartTime = now

	// Mark as listening before we start processing
	a.isListening = true
	logger.Info(logger.CategoryAudio, "Starting listening")

	// Configure the transcriber for streaming mode if available
	if a.transcriber != nil {
		if confTranscriber, ok := a.transcriber.(transcription.ConfigurableTranscriber); ok {
			// Set recording state to active
			confTranscriber.SetRecordingState(true)
			logger.Info(logger.CategoryTranscription, "Transcriber recording state set to active")
		}
	}

	a.mu.Unlock()

	// Ensure we have a valid audio recorder
	if a.audioRecorder == nil {
		logger.Info(logger.CategoryAudio, "Initializing audio recorder")
		var err error
		a.audioRecorder, err = audio.NewRecorder(audio.DefaultConfig())
		if err != nil {
			logger.Error(logger.CategoryAudio, "Failed to create audio recorder: %v", err)
			a.isListening = false
			a.ui.ShowTemporaryStatus("Error: Could not initialize audio", 3*time.Second)
			return
		}
	}

	// If we get this far but fail later, make sure we clean up
	safetyCleanup := func() {
		// If we panic or exit abruptly, make sure resources are cleaned up
		go func() {
			// Wait up to 5 minutes (should never happen in normal usage)
			time.Sleep(5 * time.Minute)
			// Force cleanup
			a.stopListening()
		}()
	}

	audioCallback := func(audioData []float32) {
		a.mu.Lock()
		// Only process if we're still listening
		if !a.isListening {
			a.mu.Unlock()
			return
		}

		// Calculate audio level
		level := calculateRMSLevel(audioData)

		// Store the level for inactivity detection
		a.lastAudioLevel = level

		a.mu.Unlock()

		// Always update audio level for visualization, even if not processing for transcription
		a.ui.UpdateAudioLevel(level)

		// Process audio data for transcription
		if a.transcriber != nil {
			// Always process audio data regardless of level - let the transcriber decide
			// This ensures we don't miss quiet speech
			logger.Debug(logger.CategoryAudio, "Processing audio chunk with level: %.4f", level)
			// Discard the error since we're handling errors in the transcriber itself
			a.transcriber.ProcessAudioChunk(audioData)
		}
	}

	// Start recording with the callback
	err := a.audioRecorder.Start(audioCallback)
	if err != nil {
		logger.Error(logger.CategoryAudio, "Failed to start audio recording: %v", err)
		a.isListening = false
		a.ui.ShowTemporaryStatus("Error: Failed to start recording", 3*time.Second)

		// Reset recording state if start failed
		if a.transcriber != nil {
			if confTranscriber, ok := a.transcriber.(transcription.ConfigurableTranscriber); ok {
				confTranscriber.SetRecordingState(false)
			}
		}
		return
	}

	// Activate safety cleanup
	safetyCleanup()
}

// calculateRMSLevel calculates audio level for visualization
func calculateRMSLevel(buffer []float32) float32 {
	// Use the base RMS calculation
	level := audio.CalculateRMSLevel(buffer)

	// Scale for UI display purposes - this scaling is UI-specific
	// The scaling factor makes the visualization more sensitive
	level = level * 8 // Increase sensitivity for better visualization
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

	// Reset flagsto prevent start/stop loops
	a.isListening = false

	// Add a cooldown period to prevent accidental rapid stop/start
	lastStartTime = time.Now()

	// Signal the transcriber that recording has stopped
	if a.transcriber != nil {
		if confTranscriber, ok := a.transcriber.(transcription.ConfigurableTranscriber); ok {
			// Set recording state to inactive to ensure proper cleanup
			confTranscriber.SetRecordingState(false)
			logger.Info(logger.CategoryTranscription, "Transcriber recording state set to inactive")
		}
	}

	a.mu.Unlock()

	// Reset the audio level to zero for the waveform
	a.ui.UpdateAudioLevel(0)

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

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	if *useTUI {
		logger.Info(logger.CategoryUI, "Using terminal-based UI")
		runTerminalUI(*debugMode)
		return
	}

	app, err := NewApplication(*debugMode)
	if err != nil {
		logger.Error(logger.CategoryApp, "Failed to initialize application: %v", err)
		os.Exit(1)
	}

	// Set up signal handler to ensure cleanup
	go func() {
		<-sigChan
		logger.Info(logger.CategoryApp, "Received termination signal, cleaning up resources...")
		app.SafeCleanup()
		os.Exit(0)
	}()

	err = app.Run()
	if err != nil {
		logger.Error(logger.CategoryApp, "Application error: %v", err)
		app.SafeCleanup()
		os.Exit(1)
	}
}

// runTerminalUI runs the application with a terminal UI
func runTerminalUI(debugMode bool) {
	// Set up a buffer for logs instead of writing directly to the console
	// This will capture logs and allow us to display them in a controlled manner
	logBuffer := &ui.LogBuffer{}

	// Redirect logger output to our buffer - critical for clean TUI experience
	logger.SetOutput(logBuffer)

	// Create a terminal UI
	terminalUI := ui.NewTerminalUI("Ctrl+Shift+S")

	// Connect the log buffer to the terminal UI so logs can be displayed properly
	logBuffer.SetLogConsumer(terminalUI)

	// Create a terminal-based executable selector for the transcription package
	termSelector := ui.NewTerminalExecutableSelector()

	// Initialize audio recorder with default config
	audioConfig := audio.DefaultConfig()
	audioConfig.Debug = debugMode

	audioRecorder, err := audio.NewRecorder(audioConfig)
	if err != nil {
		logger.Warning(logger.CategoryAudio, "Audio initialization issue: %v", err)
		logger.Info(logger.CategoryAudio, "You may need to configure your audio system or check permissions")
	}

	// Initialize transcriber with terminal selector
	transConfig := transcription.DefaultConfig()
	transConfig.Debug = debugMode

	// Redirect transcription process output to our buffer or /dev/null
	// This prevents whisper output from interfering with our UI
	transConfig.RedirectProcessOutput = true

	// Create transcription service with the terminal selector
	var transcriber transcription.Transcriber

	// Create the transcriber with UI selector
	transcriber, err = createTranscriberWithSelector(transConfig, termSelector)
	if err != nil {
		logger.Error(logger.CategoryTranscription, "Failed to initialize transcriber: %v", err)
		// Continue with a nil transcriber - we'll handle it gracefully
		transcriber = nil
	} else {
		// Log successful initialization
		logger.Info(logger.CategoryTranscription, "Transcriber initialized successfully")
	}

	// Set up streaming callback if the transcriber supports it
	if confTranscriber, ok := transcriber.(transcription.ConfigurableTranscriber); ok {
		// Get model info
		modelSize, modelPath := confTranscriber.GetModelInfo()
		logger.Info(logger.CategoryTranscription, "Using model size: %s at path: %s", modelSize, modelPath)

		confTranscriber.SetStreamingCallback(func(text string) {
			if text == "Processing audio..." {
				terminalUI.AddLog("Processing audio...")
				return
			}

			// Handle completion and other status messages
			if text == "No speech detected" || text == "Transcription timeout - try again" || text == "Processing complete" {
				terminalUI.AddLog(text)
				return
			}

			// For actual transcription text
			if text != "" && !strings.HasPrefix(text, "[") && !strings.HasPrefix(text, "(") {
				// Log that we got transcription text
				logger.Debug(logger.CategoryTranscription, "Transcription text: %s", text)
				terminalUI.UpdateText(text)
			}
		})
	}

	// Start the terminal UI
	terminalUI.Start()

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Setup recording toggle handler
	isRecording := false
	statusChan := terminalUI.GetStatusChannel()

	// Start a goroutine to handle recording state changes and signals
	go func() {
		for {
			select {
			case <-statusChan:
				// Toggle recording state
				if isRecording {
					// Stop recording
					if audioRecorder != nil {
						audioRecorder.Stop()
					}
					isRecording = false
					terminalUI.SetRecordingState(false)
					terminalUI.AddLog("Recording stopped")
				} else {
					// Start recording
					if audioRecorder != nil {
						// Define an audio callback to process the audio data
						audioCallback := func(audioData []float32) {
							// Calculate audio level
							level := calculateRMSLevel(audioData)
							terminalUI.UpdateAudioLevel(level)

							// Process audio data for transcription
							if transcriber != nil {
								// Check if the audio level is sufficient to process (avoid processing silence)
								// Using a more appropriate threshold based on real-world testing
								if level > 0.01 { // Adjusted threshold for better noise rejection
									logger.Debug(logger.CategoryAudio, "Processing audio chunk with level: %.4f", level)
									// Discard the error since we're handling errors in the transcriber itself
									transcriber.ProcessAudioChunk(audioData)
									// We don't need to log errors here as the transcriber already handles error logging
									// This eliminates repeated error messages in the logs
									// The error is already logged in whisper_streaming_transcriber.go
								} else {
									// For very low level audio, we just update the UI but don't process
									// This saves CPU and avoids unnecessary processing of silence
									logger.Debug(logger.CategoryAudio, "Skipping processing for low-level audio: %.4f", level)
								}
							} else if debugMode && transcriber != nil {
								// In debug mode, log when audio is below threshold
								terminalUI.AddLog(fmt.Sprintf("Audio below threshold: %.4f", level))

								// In debug mode, process even when below threshold to help diagnose issues
								// Discard the error since we're handling errors in the transcriber itself
								transcriber.ProcessAudioChunk(audioData)
								// We don't need to log errors here as the transcriber already handles error logging
							}
						}

						// Start audio recording with callback
						err := audioRecorder.Start(audioCallback)
						if err != nil {
							logger.Error(logger.CategoryAudio, "Failed to start recording: %v", err)
							terminalUI.AddLog("Failed to start recording")
							continue
						}
					}
					isRecording = true
					terminalUI.SetRecordingState(true)
					terminalUI.AddLog("Recording started")

					// Clear previous transcription when starting new recording
					terminalUI.UpdateText("")
				}
			case <-sigChan:
				// Cleanup on signal
				if audioRecorder != nil {
					audioRecorder.Stop()
					audioRecorder.Terminate()
				}
				if transcriber != nil {
					transcriber.Close()
				}
				terminalUI.Stop()
				return
			}
		}
	}()

	// Run the terminal UI (blocks until UI exits)
	err = terminalUI.RunBlocking()
	if err != nil {
		// This error happens after UI exits, so we can print to console directly
		fmt.Fprintf(os.Stderr, "Terminal UI error: %v\n", err)
	}

	// Cleanup when UI exits
	if audioRecorder != nil {
		audioRecorder.Terminate()
	}
	if transcriber != nil {
		transcriber.Close()
	}

	// Restore logger output to stderr for any remaining logs after UI closes
	logger.SetOutput(os.Stderr)
}

// createTranscriberWithSelector creates a transcriber using the UI selector for executables
func createTranscriberWithSelector(config transcription.Config, selector ui.ExecutableSelectorUI) (transcription.Transcriber, error) {
	// Define a simple adapter to connect UI selectors to the transcription package
	execSelector := &uiSelectorAdapter{selector}

	// Ensure debug settings for improved diagnostics
	if config.Debug {
		// Add debug logs for the transcription process
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
		// Get all executables using the finder
		if config.Finder == nil {
			config.Finder = transcription.DefaultConfig().Finder
		}

		// Find all available executables
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

// uiSelectorAdapter adapts UI selectors to transcription.ExecutableSelector
type uiSelectorAdapter struct {
	uiSelector ui.ExecutableSelectorUI
}

// SelectExecutable implements the transcription.ExecutableSelector interface
func (a *uiSelectorAdapter) SelectExecutable(executables []string) (string, error) {
	return a.uiSelector.SelectExecutable(executables)
}
