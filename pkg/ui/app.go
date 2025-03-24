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

	// Apply our custom theme immediately for better visibility of disabled elements
	fyneApp.Settings().SetTheme(NewRambleTheme(true)) // Default to dark theme

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

	// Ensure the waveform animation is started immediately
	waveform.StartListening()
	waveform.SetAmplitude(0.1) // Set initial amplitude for visibility

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
	// Log that we're trying to append text
	fmt.Printf("UI: AppendTranscript called with text: '%s'\n", text)

	if text == "" {
		fmt.Println("UI: AppendTranscript - empty text, ignoring")
		return
	}

	current := a.transcriptBox.Text
	trimmedText := strings.TrimSpace(text)

	// Only append if we have actual text
	if trimmedText == "" {
		fmt.Println("UI: AppendTranscript - text is only whitespace, ignoring")
		return
	}

	// Check for "Recording stopped" message
	isRecordingStopped := strings.Contains(trimmedText, "(Recording stopped)")

	// Check if the current text already has a recording stopped message
	hasRecordingStopped := strings.Contains(current, "(Recording stopped)")

	// Check if the current text is just the initial placeholder
	initialText := "Ready for transcription. Press Record to start."

	// Handle the different cases
	if current == initialText {
		// Replace the initial text with the first transcription
		fmt.Printf("UI: AppendTranscript - replacing initial text with: '%s'\n", trimmedText)
		a.transcriptBox.SetText(trimmedText)
	} else if isRecordingStopped && hasRecordingStopped {
		// If we already have a recording stopped message, we should not append another one
		fmt.Println("UI: AppendTranscript - already has recording stopped message, not appending")
		// Instead, we could clear the transcript or prepare for new recording
		// But for now, we'll just keep the current transcript
	} else if current == "(Recording stopped)" && isRecordingStopped {
		// Don't duplicate the message
		fmt.Println("UI: AppendTranscript - avoiding duplicate recording stopped message")
	} else {
		// Handle spacing intelligently
		if current == "" {
			fmt.Printf("UI: AppendTranscript - setting first text: '%s'\n", trimmedText)
			a.transcriptBox.SetText(trimmedText)
		} else {
			// Check if text ends with a sentence-ending character to determine formatting
			endsWithSentence := false
			if len(current) > 0 {
				lastChar := current[len(current)-1]
				endsWithSentence = lastChar == '.' || lastChar == '?' || lastChar == '!' || lastChar == '\n'
			}

			// Check if the new text appears to start a new paragraph (capitalized first letter)
			startsNewParagraph := false
			if len(trimmedText) > 0 {
				firstChar := trimmedText[0]
				startsNewParagraph = (firstChar >= 'A' && firstChar <= 'Z') && endsWithSentence
			}

			// Apply appropriate formatting
			if startsNewParagraph {
				// Add a newline for new paragraphs
				newText := current + "\n\n" + trimmedText
				fmt.Printf("UI: AppendTranscript - appending as new paragraph: '%s'\n", newText)
				a.transcriptBox.SetText(newText)
			} else if endsWithSentence {
				// Add a space after sentences
				newText := current + " " + trimmedText
				fmt.Printf("UI: AppendTranscript - appending after sentence: '%s'\n", newText)
				a.transcriptBox.SetText(newText)
			} else {
				// Check if we need to add a space between the current text and the new text
				lastChar := current[len(current)-1]
				needsSpace := lastChar != ' ' && lastChar != '\n'

				if needsSpace {
					newText := current + " " + trimmedText
					fmt.Printf("UI: AppendTranscript - appending with space: '%s'\n", newText)
					a.transcriptBox.SetText(newText)
				} else {
					newText := current + trimmedText
					fmt.Printf("UI: AppendTranscript - appending without space: '%s'\n", newText)
					a.transcriptBox.SetText(newText)
				}
			}
		}
	}

	// Scroll to the end of the text
	a.transcriptBox.CursorRow = len(a.transcriptBox.Text)
	fmt.Println("UI: AppendTranscript - done updating text")
}

// UpdateAudioLevel updates the waveform with current audio level
func (a *App) UpdateAudioLevel(level float32) {
	if a.state == StateListening || a.state == StateTranscribing {
		a.waveform.SetAmplitude(level)
	} else {
		// In non-recording states, keep the waveform visible with minimal animation
		// This ensures the UI component remains responsive and visible to the user
		a.waveform.SetAmplitude(0.05) // Small idle animation
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
	// Set text back to initial prompt
	a.transcriptBox.SetText("Ready for transcription. Press Record to start.")

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
