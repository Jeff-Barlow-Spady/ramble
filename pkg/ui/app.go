// Package ui provides the user interface for the transcription app
package ui

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/dialog"
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

	// Set window size to match the screenshot
	mainWindow.Resize(fyne.NewSize(650, 600))

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
		app.showMainWindow,
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

// showMainWindow simply shows the main window
func (a *App) showMainWindow() {
	a.mainWindow.Show()
}

// setupUI initializes all UI components with improved styling
func (a *App) setupUI() {
	// Create waveform visualizer
	waveform := NewWaveformVisualizer(theme.PrimaryColor())

	// Create UI components
	components := CreateUI(
		a.toggleListenState,
		a.clearTranscript,
		a.copyTranscript,
		waveform,
	)

	// Store references to components
	a.transcriptBox = components.TranscriptBox
	a.listenButton = components.ListenButton
	a.clearButton = components.ClearButton
	a.copyButton = components.CopyButton
	a.statusLabel = components.StatusLabel
	a.waveform = waveform

	// Set the window content
	a.mainWindow.SetContent(components.MainContent)

	// Set up key press handling
	a.keyHandlerEnabled = true
	a.mainWindow.Canvas().SetOnTypedKey(func(ke *fyne.KeyEvent) {
		// Handle key presses
		if !a.keyHandlerEnabled {
			return
		}

		// Toggle recording state with 'r' key
		if ke.Name == fyne.KeyR {
			a.keyHandlerEnabled = false // Temporarily disable to prevent rapid toggling

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
	})

	// Ensure the window maintains focus for key events
	a.mainWindow.SetOnClosed(func() {
		if a.onQuit != nil {
			a.onQuit()
		}
	})
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
	if text == "" {
		return
	}

	current := a.transcriptBox.Text
	trimmedText := strings.TrimSpace(text)

	// Only append if we have actual text
	if trimmedText == "" {
		return
	}

	// Handle spacing intelligently
	if current == "" {
		a.transcriptBox.SetText(trimmedText)
	} else {
		// Check if we need to add a space between the current text and the new text
		lastChar := current[len(current)-1]
		needsSpace := lastChar != ' ' && lastChar != '\n'

		if needsSpace {
			a.transcriptBox.SetText(current + " " + trimmedText)
		} else {
			a.transcriptBox.SetText(current + trimmedText)
		}
	}

	// Scroll to the end of the text
	a.transcriptBox.CursorRow = len(a.transcriptBox.Text)
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

// copyTranscript copies the transcript text to clipboard
func (a *App) copyTranscript() {
	text := a.transcriptBox.Text
	if text != "" {
		err := clipboard.SetText(text)
		if err != nil {
			a.showError("Clipboard Error", fmt.Sprintf("Failed to copy text: %v", err))
		} else {
			a.setStatus("Text copied to clipboard")
		}
	}
}

// clearTranscript clears the transcript text
func (a *App) clearTranscript() {
	a.transcriptBox.SetText("")
	if a.onClearTranscript != nil {
		a.onClearTranscript()
	}
}

// toggleListenState toggles between listening and idle states
func (a *App) toggleListenState() {
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
}
