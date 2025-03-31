// Package ui provides a minimal user interface
package ui

import (
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/jeff-barlow-spady/ramble/pkg/logger"
)

// Simple provides a minimal UI for transcription
type Simple struct {
	// Core app components
	app     fyne.App
	window  fyne.Window
	content *fyne.Container

	// UI elements
	transcript  *widget.Entry
	recordBtn   *widget.Button
	audioLevel  *canvas.Rectangle
	statusLabel *widget.Label

	// State
	isRecording bool

	// Callbacks
	onRecord func()
	onStop   func()

	// Thread safety
	mu sync.Mutex
}

// NewSimple creates a new minimal UI
func NewSimple() *Simple {
	// Create Fyne app
	a := app.New()
	a.Settings().SetTheme(theme.LightTheme())

	// Create main window
	w := a.NewWindow("Ramble")
	w.Resize(fyne.NewSize(500, 400))

	// Create UI instance
	ui := &Simple{
		app:         a,
		window:      w,
		isRecording: false,
	}

	// Initialize UI components
	ui.setupUI()

	return ui
}

// setupUI creates all UI components
func (s *Simple) setupUI() {
	// Create transcript field with monospace font
	s.transcript = widget.NewMultiLineEntry()
	s.transcript.TextStyle = fyne.TextStyle{Monospace: true}
	s.transcript.Wrapping = fyne.TextWrapWord
	s.transcript.SetMinRowsVisible(10)
	s.transcript.Disable()

	// Create copy and clear buttons
	copyBtn := widget.NewButtonWithIcon("Copy", theme.ContentCopyIcon(), func() {
		s.copyToClipboard()
	})

	clearBtn := widget.NewButtonWithIcon("Clear", theme.ContentClearIcon(), func() {
		s.clearTranscript()
	})

	// Create audio level indicator
	s.audioLevel = canvas.NewRectangle(theme.PrimaryColor())
	s.audioLevel.SetMinSize(fyne.NewSize(200, 20))

	// Create record button with increased size and prominence
	s.recordBtn = widget.NewButton("Start Recording", func() {
		s.toggleRecording()
	})
	s.recordBtn.Importance = widget.HighImportance // Make the button stand out

	// Create status label
	s.statusLabel = widget.NewLabel("Ready")

	// Create controls container
	controls := container.NewHBox(
		s.recordBtn,
		clearBtn,
		copyBtn,
		layout.NewSpacer(),
		s.statusLabel,
	)

	// Create level meter container
	levelMeter := container.NewPadded(s.audioLevel)

	// Create main layout
	s.content = container.NewBorder(
		controls,                          // Top
		levelMeter,                        // Bottom
		nil,                               // Left
		nil,                               // Right
		container.NewScroll(s.transcript), // Center
	)

	// Set window content
	s.window.SetContent(s.content)
}

// toggleRecording handles start/stop recording
func (s *Simple) toggleRecording() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.isRecording = !s.isRecording

	if s.isRecording {
		s.recordBtn.SetText("Stop Recording")
		s.statusLabel.SetText("Recording...")
		if s.onRecord != nil {
			go s.onRecord()
		}
	} else {
		s.recordBtn.SetText("Start Recording")
		s.statusLabel.SetText("Idle")
		if s.onStop != nil {
			go s.onStop()
		}
	}
}

// SetCallbacks sets the callbacks for recording control
func (s *Simple) SetCallbacks(onRecord, onStop func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.onRecord = onRecord
	s.onStop = onStop
}

// UpdateTranscript updates the transcript text
func (s *Simple) UpdateTranscript(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.transcript.SetText(text)
}

// AppendTranscript adds text to the transcript
func (s *Simple) AppendTranscript(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current := s.transcript.Text
	if current != "" {
		current += "\n"
	}
	s.transcript.SetText(current + text)
}

// UpdateAudioLevel updates the audio level meter
func (s *Simple) UpdateAudioLevel(level float32) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Scale up for visibility (level is typically a small value)
	scaledLevel := level * 100
	if scaledLevel > 1.0 {
		scaledLevel = 1.0
	}

	// Calculate width based on level
	width := float32(200) * scaledLevel

	// Update from the main thread via RunOnMain
	go s.window.Canvas().Content().Refresh()
	s.audioLevel.Resize(fyne.NewSize(width, 20))
	s.audioLevel.Refresh()
}

// ShowStatus displays a status message
func (s *Simple) ShowStatus(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

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

// Run starts the UI event loop
func (s *Simple) Run() {
	// Log app start
	logger.Info(logger.CategoryUI, "Starting simple UI")

	// Show and run the window
	s.window.ShowAndRun()
}

// IsRecording returns whether recording is active
func (s *Simple) IsRecording() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.isRecording
}

// Window returns the main application window
func (s *Simple) Window() fyne.Window {
	return s.window
}

// copyToClipboard copies the current transcript text to the clipboard
func (s *Simple) copyToClipboard() {
	s.mu.Lock()
	text := s.transcript.Text
	s.mu.Unlock()

	// Copy to clipboard
	s.window.Clipboard().SetContent(text)
	s.ShowTemporaryStatus("Copied to clipboard", 2*time.Second)
}

// clearTranscript clears the transcript text
func (s *Simple) clearTranscript() {
	s.mu.Lock()
	s.transcript.SetText("")
	s.mu.Unlock()

	s.ShowStatus("Transcript cleared")

	// Inform listeners that the transcript was cleared
	// This can be expanded later to call a callback if needed
}
