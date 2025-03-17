// Ramble is a cross-platform speech-to-text application
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jeff-barlow-spady/ramble/pkg/audio"
	"github.com/jeff-barlow-spady/ramble/pkg/hotkey"
	"github.com/jeff-barlow-spady/ramble/pkg/transcription"
	"github.com/jeff-barlow-spady/ramble/pkg/ui"
)

// Application manages the overall application state and components
type Application struct {
	hotkeyDetector *hotkey.Detector
	audioRecorder  *audio.Recorder
	transcriber    *transcription.Transcriber
	ui             *ui.App

	isTestMode  bool
	isListening bool
	mu          sync.Mutex
	exitChan    chan struct{}
}

// NewApplication creates a new application instance
func NewApplication(testMode bool) (*Application, error) {
	// Initialize UI with test mode
	uiApp := ui.NewWithOptions(testMode)

	// Initialize hotkey detector with default config (Ctrl+Shift+S)
	hkConfig := hotkey.DefaultConfig()
	hkDetector := hotkey.NewDetector(hkConfig)

	// Initialize audio recorder with default config and debug mode
	audioConfig := audio.DefaultConfig()
	audioConfig.Debug = testMode // Enable debug logs if in test mode

	// Create the audio recorder
	var audioRecorder *audio.Recorder
	var err error

	// Create a proper audio recorder with debug mode
	audioRecorder, err = audio.NewRecorder(audioConfig)
	if err != nil {
		log.Printf("Warning: Audio initialization issue: %v", err)
		log.Println("You may need to configure your audio system or check permissions")
		// We'll continue with a nil recorder and handle it gracefully
	}

	// Initialize transcriber with default config
	transConfig := transcription.DefaultConfig()
	transcriber, err := transcription.NewTranscriber(transConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize transcriber: %w", err)
	}

	app := &Application{
		hotkeyDetector: hkDetector,
		audioRecorder:  audioRecorder,
		transcriber:    transcriber,
		ui:             uiApp,
		isTestMode:     testMode,
		isListening:    false,
		exitChan:       make(chan struct{}),
	}

	// Set up UI callbacks
	uiApp.SetCallbacks(
		app.startListening, // On start listening
		app.stopListening,  // On stop listening
		nil,                // On clear transcript
	)

	// Set up preferences callback
	uiApp.SetPreferencesCallback(app.handlePreferencesChanged)

	// Set up quit callback
	uiApp.SetQuitCallback(func() {
		app.Cleanup()
		close(app.exitChan)
	})

	return app, nil
}

// handlePreferencesChanged applies preference changes
func (a *Application) handlePreferencesChanged(prefs ui.Preferences) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Update test mode
	a.isTestMode = prefs.TestMode

	// Update audio settings
	if a.audioRecorder != nil {
		// Create a new config with updated settings
		audioConfig := audio.Config{
			SampleRate:      prefs.SampleRate,
			Channels:        prefs.Channels,
			FramesPerBuffer: prefs.FramesPerBuffer,
			Debug:           a.isTestMode, // Enable debug mode if in test mode
		}

		// If we're currently listening, we need to stop and restart
		wasListening := a.isListening
		if wasListening {
			a.stopListeningInternal()
		}

		// Recreate the recorder with new settings
		a.audioRecorder.Terminate()

		// Try to create a real recorder with the new config
		var err error
		a.audioRecorder, err = audio.NewRecorder(audioConfig)
		if err != nil {
			log.Printf("Error updating audio recorder: %v", err)
			log.Println("Audio functionality may be limited")
		}

		// Restart if we were listening
		if wasListening {
			a.startListeningInternal()
		}
	}

	// Update hotkey if changed
	currentConfig := a.hotkeyDetector.GetConfig()
	if !equalStringSlices(currentConfig.Modifiers, prefs.HotkeyModifiers) ||
		currentConfig.Key != prefs.HotkeyKey {

		// Stop the current detector
		a.hotkeyDetector.Stop()

		// Create a new config
		newConfig := hotkey.Config{
			Modifiers: prefs.HotkeyModifiers,
			Key:       prefs.HotkeyKey,
		}

		// Create and start a new detector
		a.hotkeyDetector = hotkey.NewDetector(newConfig)
		go func() {
			err := a.hotkeyDetector.Start(a.toggleListening)
			if err != nil {
				log.Printf("Hotkey detector error: %v", err)
			}
		}()
	}

	log.Println("Preferences updated")
}

// equalStringSlices compares two string slices
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

// Run starts the application
func (a *Application) Run() error {
	// Start hotkey detector in a goroutine
	go func() {
		err := a.hotkeyDetector.Start(a.toggleListening)
		if err != nil {
			log.Printf("Hotkey detector error: %v", err)
		}
	}()

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal in a separate goroutine
	go func() {
		select {
		case <-sigCh:
			a.Cleanup()
			os.Exit(0)
		case <-a.exitChan:
			// Normal exit through UI
			os.Exit(0)
		}
	}()

	// Start UI (this blocks until window is closed)
	a.ui.Run()

	return nil
}

// toggleListening switches between listening and idle states
func (a *Application) toggleListening() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isListening {
		a.stopListeningInternal()
	} else {
		a.startListeningInternal()
	}
}

// startListening begins audio capture and transcription
func (a *Application) startListening() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.startListeningInternal()
}

// calculateRMSLevel calculates the RMS audio level from a buffer of float32 samples
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

// startListeningInternal is the internal implementation of startListening
// Caller must hold the mutex
func (a *Application) startListeningInternal() {
	if a.isListening {
		return
	}

	// If we don't have an audio recorder, show an error and abort
	if a.audioRecorder == nil {
		errMsg := "Audio recorder not initialized, cannot start recording"
		log.Println(errMsg)
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
		text, err := a.transcriber.ProcessAudioChunk(audioData)
		if err != nil {
			log.Printf("Transcription error: %v", err)
			return
		}

		if text != "" {
			// Update UI with transcribed text
			a.ui.AppendTranscript(text)
		}
	})

	if err != nil {
		log.Printf("Failed to start audio recording: %v", err)
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

// stopListening ends audio capture and transcription
func (a *Application) stopListening() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.stopListeningInternal()
}

// stopListeningInternal is the internal implementation of stopListening
// Caller must hold the mutex
func (a *Application) stopListeningInternal() {
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
		log.Printf("Failed to stop audio recording: %v", err)
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
	flag.Parse()

	// Print startup message
	fmt.Println("Starting Ramble - Speech-to-Text Application")

	if *debugMode {
		fmt.Println("Running in debug mode")
	}

	app, err := NewApplication(*debugMode)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	err = app.Run()
	if err != nil {
		log.Fatalf("Application error: %v", err)
	}
}
