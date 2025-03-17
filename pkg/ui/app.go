// Package ui provides the user interface for the transcription app
package ui

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/jeff-barlow-spady/ramble/internal/clipboard"
	"github.com/jeff-barlow-spady/ramble/pkg/resources"
)

// AppState tracks the current state of the application
type AppState int

const (
	StateIdle AppState = iota
	StateListening
	StateTranscribing
)

// App manages the Fyne application and UI components
type App struct {
	fyneApp            fyne.App
	mainWindow         fyne.Window
	transcriptBox      *widget.Entry
	statusLabel        *canvas.Text // Changed to canvas.Text for better styling
	copyButton         *widget.Button
	clearButton        *widget.Button
	listenButton       *widget.Button // Add reference to the listen button
	waveform           *WaveformVisualizer
	systray            *SystemTray
	state              AppState
	isTestMode         bool
	currentPreferences Preferences
	keyHandlerEnabled  bool // Track if key handler is enabled

	// Callbacks for UI events
	onStartListening     func()
	onStopListening      func()
	onClearTranscript    func()
	onQuit               func()
	onPreferencesChanged func(Preferences)
}

// RunOnMain runs a function on the main thread
func RunOnMain(f func()) {
	f()
}

// New creates a new UI application
func New() *App {
	return NewWithOptions(false)
}

// NewWithOptions creates a new UI application with customizable options
func NewWithOptions(testMode bool) *App {
	fyneApp := app.New()
	mainWindow := fyneApp.NewWindow("Ramble - Speech-to-Text")

	// Set application icon
	appIcon := resources.LoadAppIcon()
	if appIcon != nil {
		fyneApp.SetIcon(appIcon)
		mainWindow.SetIcon(appIcon)
	}

	// Create system tray
	systray := NewSystemTray(testMode)

	// Set default preferences
	prefs := DefaultPreferences()
	prefs.TestMode = testMode

	app := &App{
		fyneApp:            fyneApp,
		mainWindow:         mainWindow,
		systray:            systray,
		state:              StateIdle,
		isTestMode:         testMode,
		currentPreferences: prefs,
	}

	// Set up window close event to minimize instead of quit
	mainWindow.SetCloseIntercept(func() {
		app.mainWindow.Hide()
	})

	// Set up system tray callbacks
	systray.SetCallbacks(
		app.toggleListening,
		app.showPreferencesDialog,
		app.showAboutDialog,
		app.doQuit,
	)

	app.setupUI()

	// Start system tray after UI is set up
	systray.Start()

	return app
}

// toggleListening switches the listening state on/off
func (a *App) toggleListening() {
	// Make sure the window has focus for key events
	a.mainWindow.RequestFocus()

	// Simply trigger the listen button's action
	if a.listenButton != nil {
		a.listenButton.OnTapped()
	}
}

// showAboutDialog shows the about dialog
func (a *App) showAboutDialog() {
	a.mainWindow.Show()
	dialog.ShowInformation("About Ramble",
		"Ramble Speech-to-Text\nVersion 0.1.0\n\nA cross-platform speech-to-text application.",
		a.mainWindow)
}

// showPreferencesDialog shows the preferences dialog
func (a *App) showPreferencesDialog() {
	a.mainWindow.Show()
	a.ShowPreferences(func(prefs Preferences) {
		a.currentPreferences = prefs
		if a.onPreferencesChanged != nil {
			a.onPreferencesChanged(prefs)
		}
	})
}

// doQuit properly exits the application
func (a *App) doQuit() {
	if a.onQuit != nil {
		a.onQuit()
	}
	a.systray.Stop()
	a.fyneApp.Quit()
}

// setupUI initializes all UI components with improved styling
func (a *App) setupUI() {
	// Create transcript box (multi-line text)
	a.transcriptBox = widget.NewMultiLineEntry()
	a.transcriptBox.Wrapping = fyne.TextWrapWord
	a.transcriptBox.SetPlaceHolder("Transcribed text will appear here")

	// Create status label with styled text
	a.statusLabel = canvas.NewText("Ready", color.NRGBA{R: 180, G: 180, B: 180, A: 255})
	a.statusLabel.TextStyle.Monospace = true
	a.statusLabel.TextSize = 16

	// Create waveform visualizer with a bright cyan color
	waveformColor := color.RGBA{R: 0x61, G: 0xE3, B: 0xFA, A: 0xFF} // Bright cyan from popup.go
	a.waveform = NewWaveformVisualizer(waveformColor)

	// Create action buttons
	recordButtonIcon := theme.MediaRecordIcon()
	a.listenButton = widget.NewButtonWithIcon("Record", recordButtonIcon, func() {
		if a.state == StateIdle {
			a.setState(StateListening)
			if a.onStartListening != nil {
				a.onStartListening()
			}
			a.waveform.StartListening()
			a.listenButton.SetText("Stop")
			a.listenButton.SetIcon(theme.MediaStopIcon())
		} else {
			a.setState(StateIdle)
			if a.onStopListening != nil {
				a.onStopListening()
			}
			a.waveform.StopListening()
			a.listenButton.SetText("Record")
			a.listenButton.SetIcon(theme.MediaRecordIcon())
		}
	})
	a.listenButton.Importance = widget.HighImportance // Make the button stand out

	a.copyButton = widget.NewButton("Copy to Clipboard", func() {
		text := a.transcriptBox.Text
		if text != "" {
			err := clipboard.SetText(text)
			if err != nil {
				a.showError("Clipboard Error", fmt.Sprintf("Failed to copy text: %v", err))
			} else {
				a.setStatus("Text copied to clipboard")
			}
		}
	})

	a.clearButton = widget.NewButton("Clear", func() {
		a.transcriptBox.SetText("")
		if a.onClearTranscript != nil {
			a.onClearTranscript()
		}
	})

	// Add preferences button
	prefsButton := widget.NewButtonWithIcon("", theme.SettingsIcon(), func() {
		a.showPreferencesDialog()
	})
	prefsButton.Importance = widget.LowImportance

	// Create headers and title styling from popup.go
	title := widget.NewLabelWithStyle("Ramble", fyne.TextAlignCenter, fyne.TextStyle{Bold: true, Monospace: true})
	subtitle := widget.NewLabelWithStyle("Speech-to-Text Service", fyne.TextAlignCenter, fyne.TextStyle{Monospace: true})
	version := widget.NewLabelWithStyle("v0.1.0", fyne.TextAlignCenter, fyne.TextStyle{Monospace: true})

	header := container.NewVBox(
		title,
		subtitle,
		version,
	)

	// Create audio section with terminal-like styling
	audioTitle := widget.NewLabelWithStyle("Audio Level", fyne.TextAlignLeading, fyne.TextStyle{Bold: true, Monospace: true})

	// Create waveform container with a darkened background
	waveformBg := canvas.NewRectangle(color.NRGBA{R: 30, G: 30, B: 46, A: 255})
	waveformContainer := container.NewStack(
		waveformBg,
		container.NewPadded(a.waveform),
	)

	// Set minimum size for waveform
	waveformContainer.Resize(fyne.NewSize(600, 100))

	// Create audio container with title
	audioContainer := container.NewBorder(
		audioTitle,
		nil,
		nil,
		nil,
		container.NewPadded(waveformContainer),
	)

	// Create transcription section with terminal-like styling
	transcriptTitle := widget.NewLabelWithStyle("Transcription", fyne.TextAlignLeading, fyne.TextStyle{Bold: true, Monospace: true})

	// Style the transcript box with a dark background
	transcriptBg := canvas.NewRectangle(color.NRGBA{R: 30, G: 30, B: 46, A: 255})
	scrollText := container.NewScroll(a.transcriptBox)
	scrollText.SetMinSize(fyne.NewSize(0, 150))

	styledTranscript := container.NewStack(
		transcriptBg,
		container.NewPadded(scrollText),
	)

	transcriptionContainer := container.NewBorder(
		transcriptTitle,
		nil,
		nil,
		nil,
		container.NewPadded(styledTranscript),
	)

	// Controls section
	controlsContainer := container.NewHBox(
		a.listenButton,
		a.copyButton,
		a.clearButton,
		layout.NewSpacer(),
		widget.NewLabelWithStyle("Keyboard: 'R' to toggle recording", fyne.TextAlignTrailing, fyne.TextStyle{Monospace: true}),
		prefsButton,
	)

	// Create main content with a dark background
	mainBg := canvas.NewRectangle(color.NRGBA{R: 20, G: 20, B: 30, A: 255})

	// Create a container for the status indicator
	statusContainer := container.NewHBox(
		a.statusLabel,
	)

	// Layout all components with terminal-like styling
	content := container.NewStack(
		mainBg,
		container.NewPadded(
			container.NewBorder(
				container.NewVBox(
					container.NewPadded(header),
					audioContainer,
				),
				container.NewVBox(
					container.NewPadded(controlsContainer),
					container.NewPadded(statusContainer),
				),
				nil,
				nil,
				transcriptionContainer,
			),
		),
	)

	// Handle key presses globally with direct event handler
	a.keyHandlerEnabled = true
	a.mainWindow.Canvas().SetOnTypedKey(func(event *fyne.KeyEvent) {
		// Simpler approach without delays that could cause issues
		if event.Name == "R" || event.Name == "r" {
			// Only handle keypress if we're not already processing one
			if a.keyHandlerEnabled {
				a.keyHandlerEnabled = false
				// Use the same logic as the button click
				if a.state == StateIdle {
					a.setState(StateListening)
					if a.onStartListening != nil {
						a.onStartListening()
					}
					a.waveform.StartListening()
					a.listenButton.SetText("Stop")
					a.listenButton.SetIcon(theme.MediaStopIcon())
				} else {
					a.setState(StateIdle)
					if a.onStopListening != nil {
						a.onStopListening()
					}
					a.waveform.StopListening()
					a.listenButton.SetText("Record")
					a.listenButton.SetIcon(theme.MediaRecordIcon())
				}
				// Re-enable immediately after toggling
				a.keyHandlerEnabled = true
			}
		}
	})

	// Ensure the window maintains focus for key events
	a.mainWindow.SetOnClosed(func() {
		if a.onQuit != nil {
			a.onQuit()
		}
	})

	a.mainWindow.SetContent(content)
	a.mainWindow.Resize(fyne.NewSize(650, 600)) // Slightly larger for the improved UI
}

// Run starts the UI event loop
func (a *App) Run() {
	a.mainWindow.ShowAndRun()
}

// UpdateTranscript updates the transcript text
func (a *App) UpdateTranscript(text string) {
	a.transcriptBox.SetText(text)
}

// AppendTranscript adds text to the current transcript
func (a *App) AppendTranscript(text string) {
	current := a.transcriptBox.Text
	if current != "" {
		current += " "
	}
	a.transcriptBox.SetText(current + text)
}

// UpdateAudioLevel updates the waveform with current audio level
func (a *App) UpdateAudioLevel(level float32) {
	if a.state == StateListening || a.state == StateTranscribing {
		a.waveform.SetAmplitude(level)
	}
}

// setState updates the application state and UI elements
func (a *App) setState(state AppState) {
	a.state = state

	// Update system tray state
	a.systray.UpdateRecordingState(state == StateListening || state == StateTranscribing)

	switch state {
	case StateIdle:
		a.statusLabel.Text = "Ready"
		a.statusLabel.Color = color.NRGBA{R: 180, G: 180, B: 180, A: 255}
		a.statusLabel.Refresh()
		a.mainWindow.SetTitle("Ramble")
	case StateListening:
		a.statusLabel.Text = "● RECORDING"
		a.statusLabel.Color = color.RGBA{R: 255, G: 0, B: 0, A: 255}
		a.statusLabel.Refresh()
		a.mainWindow.SetTitle("Ramble - Recording...")
	case StateTranscribing:
		a.statusLabel.Text = "Transcribing..."
		a.statusLabel.Color = color.RGBA{R: 255, G: 165, B: 0, A: 255}
		a.statusLabel.Refresh()
	}
}

// setStatus updates the status label
func (a *App) setStatus(status string) {
	a.statusLabel.Text = status
	a.statusLabel.Refresh()
}

// showError displays an error dialog
func (a *App) showError(title, message string) {
	dialog.ShowError(fmt.Errorf("%s", message), a.mainWindow)
}

// ShowErrorDialog displays an error dialog with title and message
func (a *App) ShowErrorDialog(title, message string) {
	a.showError(title, message)
}

// SetCallbacks sets the callback functions for UI events
func (a *App) SetCallbacks(onStart, onStop, onClear func()) {
	a.onStartListening = onStart
	a.onStopListening = onStop
	a.onClearTranscript = onClear
}

// SetQuitCallback sets the callback function for quitting the application
func (a *App) SetQuitCallback(onQuit func()) {
	a.onQuit = onQuit
}

// SetPreferencesCallback sets the callback function for preferences changes
func (a *App) SetPreferencesCallback(onPrefsChanged func(Preferences)) {
	a.onPreferencesChanged = onPrefsChanged
}

// ShowTemporaryStatus shows a status message that disappears after a delay
func (a *App) ShowTemporaryStatus(status string, duration time.Duration) {
	prevText := a.statusLabel.Text
	prevColor := a.statusLabel.Color

	a.statusLabel.Text = status
	a.statusLabel.Refresh()

	go func() {
		time.Sleep(duration)
		if a.statusLabel.Text == status {
			a.statusLabel.Text = prevText
			a.statusLabel.Color = prevColor
			a.statusLabel.Refresh()
		}
	}()
}

// GetPreferences returns the current preferences
func (a *App) GetPreferences() Preferences {
	return a.currentPreferences
}
