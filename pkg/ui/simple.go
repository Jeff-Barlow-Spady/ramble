// Package ui provides a minimal user interface
package ui

import (
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Simple provides a minimal UI for transcription
// This is a legacy implementation and has been replaced by App.
// It is kept for backward compatibility only.
type Simple struct {
	// Core app components
	app     fyne.App
	window  fyne.Window
	content *fyne.Container
	systray *SystemTray

	// UI elements
	transcript  *widget.Entry
	recordBtn   *widget.Button
	audioLevel  *canvas.Rectangle
	statusLabel *widget.Label
	waveform    *WaveformVisualizer
	appTitle    *canvas.Text

	// State
	isRecording        bool
	startHidden        bool
	currentPreferences Preferences

	// Callbacks
	onRecord             func()
	onStop               func()
	onQuit               func()
	onPreferencesChanged func(Preferences)

	// Thread safety
	mu sync.Mutex
}

// NewSimple creates a new minimal UI
func NewSimple() *Simple {
	return NewSimpleWithOptions(false)
}

// NewSimpleWithOptions creates a new minimal UI with options
func NewSimpleWithOptions(testMode bool) *Simple {
	// Create Fyne app
	a := app.New()
	a.Settings().SetTheme(NewRambleTheme(false)) // Use the Ramble theme (light mode)

	// Create main window
	w := a.NewWindow("Ramble")
	w.Resize(fyne.NewSize(600, 500)) // Slightly larger default size

	// Create system tray
	systray := NewSystemTray(testMode)

	// Initialize preferences with defaults
	prefs := DefaultPreferences()
	prefs.TestMode = testMode

	// Create UI instance
	ui := &Simple{
		app:                a,
		window:             w,
		systray:            systray,
		isRecording:        false,
		currentPreferences: prefs,
	}

	// Set up window close event to minimize instead of quit
	w.SetCloseIntercept(func() {
		ui.window.Hide()
	})

	// Initialize UI components
	ui.setupUI()

	// Set up system tray callbacks
	systray.SetCallbacks(
		ui.toggleRecording,
		ui.showPreferencesDialog,
		func() {
			dialog.ShowInformation("About Ramble", "Ramble Speech-to-Text\nVersion 0.1.0\n\nTranscribe speech to text quickly and easily.", w)
		},
		ui.doQuit,
		ui.showMainWindow,
	)

	// Start system tray after UI is set up
	systray.Start()

	return ui
}

// setupUI creates all UI components with personality
func (s *Simple) setupUI() {
	// Create a simplified layout for legacy support
	s.transcript = widget.NewMultiLineEntry()
	s.transcript.Disable()
	s.transcript.SetPlaceHolder("Your transcription will appear here...")

	s.waveform = NewWaveformVisualizer(theme.PrimaryColor())
	s.recordBtn = widget.NewButton("Start Recording", s.toggleRecording)
	s.statusLabel = widget.NewLabel("Ready")

	buttons := container.NewHBox(
		s.recordBtn,
		widget.NewButton("Copy", s.copyToClipboard),
		widget.NewButton("Clear", s.clearTranscript),
	)

	// Simple layout without problematic nested containers
	s.content = container.NewBorder(
		nil,
		container.NewVBox(buttons),
		nil,
		nil,
		s.transcript,
	)

	s.window.SetContent(s.content)
}

// toggleRecording handles start/stop recording
func (s *Simple) toggleRecording() {
	s.isRecording = !s.isRecording
	if s.isRecording {
		if s.onRecord != nil {
			s.onRecord()
		}
	} else {
		if s.onStop != nil {
			s.onStop()
		}
	}
}

// SetCallbacks sets the callbacks for recording control
func (s *Simple) SetCallbacks(onRecord, onStop func()) {
	s.onRecord = onRecord
	s.onStop = onStop
}

// SetQuitCallback sets the callback for application quit
func (s *Simple) SetQuitCallback(onQuit func()) {
	s.onQuit = onQuit
}

// SetPreferencesCallback sets the callback for preferences changes
func (s *Simple) SetPreferencesCallback(onPrefsChanged func(Preferences)) {
	s.onPreferencesChanged = onPrefsChanged
}

// UpdateTranscript updates the transcript text
func (s *Simple) UpdateTranscript(text string) {
	s.transcript.SetText(text)
}

// AppendTranscript adds text to the transcript
func (s *Simple) AppendTranscript(text string) {
	trimmedText := strings.TrimSpace(text)
	if trimmedText == "" {
		return
	}

	current := s.transcript.Text
	if current == "" || current == "Your transcription will appear here..." {
		s.transcript.SetText(trimmedText)
	} else {
		s.transcript.SetText(current + " " + trimmedText)
	}
}

// UpdateAudioLevel updates the waveform visualizer
func (s *Simple) UpdateAudioLevel(level float32) {
	s.waveform.SetAmplitude(level)
}

// ShowStatus displays a status message
func (s *Simple) ShowStatus(message string) {
	s.statusLabel.SetText(message)
}

// ShowTemporaryStatus shows a status message that disappears after a duration
func (s *Simple) ShowTemporaryStatus(message string, duration time.Duration) {
	s.ShowStatus(message)
	go func() {
		time.Sleep(duration)
		s.ShowStatus("Ready")
	}()
}

// Run starts the application
func (s *Simple) Run() {
	s.window.ShowAndRun()
}

// SetStartHidden sets whether the app should start hidden in the system tray
func (s *Simple) SetStartHidden(hidden bool) {
	s.startHidden = hidden
}

// IsRecording returns the current recording state
func (s *Simple) IsRecording() bool {
	return s.isRecording
}

// Window returns the main window
func (s *Simple) Window() fyne.Window {
	return s.window
}

// doQuit properly exits the application
func (s *Simple) doQuit() {
	if s.onQuit != nil {
		s.onQuit()
	}
	if s.systray != nil {
		s.systray.Stop()
	}
	if s.app != nil {
		s.app.Quit()
	}
}

// showMainWindow shows the main application window
func (s *Simple) showMainWindow() {
	s.window.Show()
}

// copyToClipboard copies the transcript to the clipboard
func (s *Simple) copyToClipboard() {
	text := s.transcript.Text
	s.window.Clipboard().SetContent(text)
}

// clearTranscript clears the transcript
func (s *Simple) clearTranscript() {
	s.transcript.SetText("")
}

// showPreferencesDialog shows the preferences dialog
func (s *Simple) showPreferencesDialog() {
	s.window.Show()
	ShowPreferences(s.app, s.window, s.currentPreferences, func(prefs Preferences) {
		s.currentPreferences = prefs
		if prefs.DarkTheme {
			s.app.Settings().SetTheme(NewRambleTheme(true))
		} else {
			s.app.Settings().SetTheme(NewRambleTheme(false))
		}
		if s.onPreferencesChanged != nil {
			s.onPreferencesChanged(prefs)
		}
	})
}

// GetPreferences returns the current preferences
func (s *Simple) GetPreferences() Preferences {
	return s.currentPreferences
}

// ShowPreferences creates and displays a standalone preferences dialog
// This is a helper function to create a preferences dialog without needing an App instance
func ShowPreferences(fyneApp fyne.App, parent fyne.Window, currentPrefs Preferences, onSave func(Preferences)) {
	// If preferences window already exists, bring it to front
	if preferencesWindow != nil {
		preferencesWindow.Show()
		preferencesWindow.RequestFocus()
		return
	}

	// Create a new window for preferences
	w := fyneApp.NewWindow("Ramble Preferences")
	w.Resize(fyne.NewSize(500, 450))

	// Store reference to the window
	preferencesWindow = w

	// Set close handler to clear the reference
	w.SetCloseIntercept(func() {
		preferencesWindow = nil
		w.Close()
	})

	// Create dialog
	dialog := &PreferencesDialog{
		window: w,
		onSave: onSave,
		prefs:  currentPrefs, // Use current preferences as starting point
	}

	// Set up the UI
	dialog.setupUI()

	// Show the window
	w.Show()
}
