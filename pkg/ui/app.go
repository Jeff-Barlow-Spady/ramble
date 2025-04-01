package ui

import (
	"fmt"
	"image/color"
	"os"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/jeff-barlow-spady/ramble/pkg/clipboard"
	"github.com/jeff-barlow-spady/ramble/pkg/logger"
	"github.com/jeff-barlow-spady/ramble/pkg/resources"
)

// AppState represents the current state of the application
type AppState int

const (
	StateIdle AppState = iota
	StateListening
	StateTranscribing
	StateError
)

// App manages the Fyne application and UI components
type App struct {
	fyneApp            fyne.App
	mainWindow         fyne.Window
	transcriptBox      *widget.Entry
	statusLabel        *canvas.Text
	listenButton       *widget.Button
	waveform           *WaveformVisualizer
	systray            *SystemTray
	appTitle           *canvas.Text
	state              AppState
	isTestMode         bool
	currentPreferences Preferences
	keyHandlerEnabled  bool

	// Hover window for compact UI
	hoverWindow *HoverWindow
	isHoverMode bool

	// Start hidden in system tray
	startHidden bool

	// Callbacks for UI events
	onStartListening     func()
	onStopListening      func()
	onClearTranscript    func()
	onQuit               func()
	onPreferencesChanged func(Preferences)
}

// New creates a new UI application
func New() *App {
	return NewWithOptions(false)
}

// NewWithOptions creates a new UI application with customizable options
func NewWithOptions(testMode bool) *App {
	fyneApp := app.New()

	// Apply our custom theme immediately for better visibility
	fyneApp.Settings().SetTheme(NewRambleTheme(true)) // Default to dark theme

	mainWindow := fyneApp.NewWindow("Ramble")
	mainWindow.Resize(fyne.NewSize(700, 500))

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
	prefs.DarkTheme = true // Default to dark theme

	app := &App{
		fyneApp:            fyneApp,
		mainWindow:         mainWindow,
		systray:            systray,
		state:              StateIdle,
		isTestMode:         testMode,
		currentPreferences: prefs,
		keyHandlerEnabled:  true,
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

// setupUI initializes all UI components with improved styling
func (a *App) setupUI() {
	// Create a simpler, more visible banner with plain text that will show reliably
	bannerText := "RAMBLE"
	asciiBanner := canvas.NewText(bannerText, color.NRGBA{R: 150, G: 180, B: 255, A: 255})
	asciiBanner.TextSize = 36 // Much larger size for better visibility
	asciiBanner.TextStyle = fyne.TextStyle{Bold: true}
	asciiBanner.Alignment = fyne.TextAlignCenter

	// Create subtitle directly under the banner
	subtitle := canvas.NewText("Speech-to-Text Transcription", color.NRGBA{R: 200, G: 200, B: 200, A: 255})
	subtitle.TextSize = 16
	subtitle.TextStyle = fyne.TextStyle{Italic: true}
	subtitle.Alignment = fyne.TextAlignCenter

	// Store the banner as the app title
	a.appTitle = asciiBanner

	// Create waveform visualizer with proper color
	a.waveform = NewWaveformVisualizer(color.NRGBA{R: 100, G: 140, B: 240, A: 255})
	a.waveform.StartListening()
	a.waveform.SetAmplitude(0.1) // Set initial amplitude for visibility

	// Create the transcript box with improved readability
	a.transcriptBox = widget.NewMultiLineEntry()
	a.transcriptBox.Disable() // Make read-only but still selectable
	a.transcriptBox.SetPlaceHolder("Your transcription will appear here...")
	a.transcriptBox.Wrapping = fyne.TextWrapWord
	a.transcriptBox.SetMinRowsVisible(12)
	a.transcriptBox.TextStyle = fyne.TextStyle{Monospace: true} // Monospace for better readability

	// Create a frame around the waveform with centered content that fills the width
	waveformContainer := container.New(layout.NewMaxLayout(),
		canvas.NewRectangle(color.NRGBA{R: 30, G: 36, B: 66, A: 255}),
		a.waveform, // Place waveform directly without additional centering container
	)

	// Create a fixed size container for the waveform to ensure adequate height
	// This will wrap our container to ensure minimum height but allow horizontal expansion
	heightSetter := canvas.NewRectangle(color.Transparent)
	heightSetter.SetMinSize(fyne.NewSize(0, 100))

	waveformWithHeight := container.New(layout.NewMaxLayout(),
		heightSetter,
		waveformContainer,
	)

	// Create a padded container for the waveform that expands horizontally
	waveformPadded := container.NewPadded(waveformWithHeight)

	// Create a full-width container
	waveformSection := container.New(layout.NewMaxLayout(), waveformPadded)

	// Create the buttons
	a.listenButton = widget.NewButtonWithIcon("Start Recording", theme.MediaRecordIcon(), a.toggleListening)
	a.listenButton.Importance = widget.HighImportance

	copyButton := widget.NewButtonWithIcon("Copy to Clipboard", theme.ContentCopyIcon(), a.copyTranscript)
	clearButton := widget.NewButtonWithIcon("Clear", theme.ContentClearIcon(), a.clearTranscript)

	// Create status label with styling
	a.statusLabel = canvas.NewText("Ready", color.NRGBA{R: 100, G: 200, B: 100, A: 255})
	a.statusLabel.TextSize = 16 // Larger text for better visibility
	statusContainer := container.NewHBox(
		canvas.NewCircle(color.NRGBA{R: 100, G: 200, B: 100, A: 255}),
		a.statusLabel,
	)

	// Create banner container with proper spacing
	bannerContainer := container.NewVBox(
		layout.NewSpacer(),
		container.NewCenter(asciiBanner),
		container.NewCenter(subtitle),
		layout.NewSpacer(),
	)

	// Add fixed padding around the banner
	paddedBanner := container.NewPadded(bannerContainer)

	// Arrange header with banner at top
	header := container.NewVBox(
		paddedBanner,
		container.NewHBox(
			layout.NewSpacer(),
			widget.NewButtonWithIcon("", theme.SettingsIcon(), a.showPreferencesDialog),
		),
		widget.NewSeparator(),
	)

	// Arrange buttons in a horizontal container with better spacing
	buttons := container.NewHBox(
		container.NewPadded(a.listenButton),
		layout.NewSpacer(),
		container.NewHBox(
			copyButton,
			clearButton,
		),
	)

	// Create main content with border layout
	content := container.NewBorder(
		// Top - banner and controls
		header,
		// Bottom - waveform and status
		container.NewVBox(
			buttons,
			waveformSection,
			container.NewHBox(layout.NewSpacer(), statusContainer, layout.NewSpacer()),
		),
		// Left - none
		nil,
		// Right - none
		nil,
		// Center - transcript with scroll and padding
		container.NewPadded(container.NewScroll(a.transcriptBox)),
	)

	// Set the window content
	a.mainWindow.SetContent(content)

	// Create the hover window
	a.hoverWindow = NewHoverWindow(a.fyneApp)
	a.hoverWindow.SetCallbacks(
		a.toggleListening,
		a.copyTranscript,
		func() {
			a.hoverWindow.Hide()
			a.isHoverMode = false
		},
	)

	// Set up hotkeys
	a.setupHotkeys()
}

// setupHotkeys sets up keyboard shortcuts
func (a *App) setupHotkeys() {
	// Register the Ctrl+Shift+S shortcut for hover window
	hoverShortcut := &desktop.CustomShortcut{
		KeyName:  fyne.KeyS,
		Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift,
	}

	a.mainWindow.Canvas().AddShortcut(hoverShortcut, func(shortcut fyne.Shortcut) {
		if !a.isTestMode && a.keyHandlerEnabled {
			logger.Info(logger.CategoryUI, "Hover window shortcut triggered")
			a.toggleHoverWindow()
		}
	})

	// Register key handler for space key to toggle recording
	a.mainWindow.Canvas().SetOnTypedKey(func(ke *fyne.KeyEvent) {
		// Skip keyboard handling if disabled or in test mode
		if !a.keyHandlerEnabled || a.isTestMode {
			return
		}

		// Check if we're focused on the transcript box (or any Entry widget)
		focused := a.mainWindow.Canvas().Focused()
		isEntryFocused := false
		if focused != nil {
			_, isEntryFocused = focused.(*widget.Entry)
		}

		// Only allow Space key when NOT typing in an entry widget
		if ke.Name == fyne.KeySpace && !isEntryFocused {
			a.keyHandlerEnabled = false // Temporarily disable to prevent rapid toggling

			// Toggle recording
			a.toggleListening()

			// Re-enable after a delay to prevent accidental double-triggers
			go func() {
				time.Sleep(300 * time.Millisecond)
				a.keyHandlerEnabled = true
			}()
		}
	})
}

// Run starts the UI event loop
func (a *App) Run() {
	// Apply theme based on preferences
	if a.currentPreferences.DarkTheme {
		a.fyneApp.Settings().SetTheme(NewRambleTheme(true))
	} else {
		a.fyneApp.Settings().SetTheme(NewRambleTheme(false))
	}

	// Start hidden or visible based on preferences
	if a.currentPreferences.StartMinimized || a.startHidden {
		a.fyneApp.Run()
	} else {
		a.mainWindow.ShowAndRun()
	}
}

// toggleListening switches between listening and idle states
func (a *App) toggleListening() {
	// Focus the window for key events
	a.mainWindow.RequestFocus()

	if a.state == StateListening || a.state == StateTranscribing {
		// Stop listening
		a.SetState(StateIdle)
		if a.onStopListening != nil {
			a.onStopListening()
		}

		// Re-enable keyboard shortcuts
		a.keyHandlerEnabled = true

		// Update hover window if active
		if a.hoverWindow != nil {
			a.hoverWindow.SetRecordingState(false)
		}
	} else {
		// Start listening
		a.SetState(StateListening)
		if a.onStartListening != nil {
			a.onStartListening()
		}

		// Disable keyboard shortcuts while recording
		a.keyHandlerEnabled = false

		// Update hover window if active
		if a.hoverWindow != nil {
			a.hoverWindow.SetRecordingState(true)
		}
	}
}

// SetState updates the application state and UI elements
func (a *App) SetState(state AppState) {
	a.state = state
	a.systray.UpdateRecordingState(state == StateListening || state == StateTranscribing)

	// Update hover window if active
	if a.isHoverMode && a.hoverWindow != nil {
		a.hoverWindow.SetRecordingState(state == StateListening || state == StateTranscribing)
	}

	// Update UI based on state
	switch state {
	case StateIdle:
		a.statusLabel.Text = "Ready"
		a.statusLabel.Color = color.NRGBA{R: 100, G: 200, B: 100, A: 255}
		a.statusLabel.Refresh()
		a.mainWindow.SetTitle("Ramble")
		a.listenButton.SetText("Start Recording")
		a.listenButton.SetIcon(theme.MediaRecordIcon())
		a.appTitle.Color = color.NRGBA{R: 100, G: 140, B: 240, A: 255} // Reset to normal blue
		a.appTitle.Refresh()
	case StateListening:
		a.statusLabel.Text = "● RECORDING"
		a.statusLabel.Color = color.RGBA{R: 255, G: 50, B: 50, A: 255}
		a.statusLabel.Refresh()
		a.mainWindow.SetTitle("Ramble - Recording...")
		a.listenButton.SetText("Stop Recording")
		a.listenButton.SetIcon(theme.MediaStopIcon())
		a.appTitle.Color = color.NRGBA{R: 200, G: 50, B: 50, A: 255} // Red to indicate recording
		a.appTitle.Refresh()
	case StateTranscribing:
		a.statusLabel.Text = "Transcribing..."
		a.statusLabel.Color = color.RGBA{R: 255, G: 165, B: 0, A: 255}
		a.statusLabel.Refresh()
		a.listenButton.SetText("Stop Recording")
		a.listenButton.SetIcon(theme.MediaStopIcon())
	case StateError:
		a.statusLabel.Text = "Error"
		a.statusLabel.Color = color.RGBA{R: 255, G: 0, B: 0, A: 255}
		a.statusLabel.Refresh()
		a.mainWindow.SetTitle("Ramble - Error")
		a.listenButton.SetText("Start Recording")
		a.listenButton.SetIcon(theme.MediaRecordIcon())
	}
}

// UpdateTranscript updates the transcript text
func (a *App) UpdateTranscript(text string) {
	a.transcriptBox.SetText(text)
}

// AppendTranscript adds text to the transcript
func (a *App) AppendTranscript(text string) {
	// Handle intelligently like in simple.go
	trimmedText := strings.TrimSpace(text)
	if trimmedText == "" {
		return
	}

	current := a.transcriptBox.Text
	if current == "" || current == "Your transcription will appear here..." {
		a.transcriptBox.SetText(trimmedText)
	} else {
		// Check if we need to add punctuation
		lastChar := current[len(current)-1]
		needsPunctuation := lastChar != '.' && lastChar != '?' &&
			lastChar != '!' && lastChar != '\n' && lastChar != ' '

		if needsPunctuation {
			current += ". "
		} else if lastChar != ' ' && lastChar != '\n' {
			current += " "
		}

		a.transcriptBox.SetText(current + trimmedText)
	}

	// Auto-scroll to bottom when new text is added
	a.transcriptBox.CursorRow = len(strings.Split(a.transcriptBox.Text, "\n")) - 1

	// Update hover window if active
	if a.isHoverMode && a.hoverWindow != nil {
		a.hoverWindow.AppendTranscript(trimmedText)
	}
}

// UpdateAudioLevel updates the audio level in the waveform
func (a *App) UpdateAudioLevel(level float32) {
	// Set a minimum amplitude for visual feedback
	displayLevel := level
	if displayLevel < 0.05 {
		displayLevel = 0.05
	}

	// Update main window waveform
	if a.waveform != nil {
		a.waveform.SetAmplitude(displayLevel)
	}

	// Update hover window waveform if active
	if a.isHoverMode && a.hoverWindow != nil {
		a.hoverWindow.UpdateAudioLevel(displayLevel)
	}
}

// ShowTemporaryStatus shows a status message that disappears after a delay
func (a *App) ShowTemporaryStatus(message string, duration time.Duration) {
	prevText := a.statusLabel.Text
	prevColor := a.statusLabel.Color

	// Use a distinct color for clipboard-related messages
	if strings.Contains(message, "clipboard") || strings.Contains(message, "copied") {
		a.statusLabel.Color = color.NRGBA{R: 50, G: 200, B: 50, A: 255} // Green for copy actions
	} else {
		a.statusLabel.Color = color.NRGBA{R: 220, G: 220, B: 0, A: 255} // Yellow for other messages
	}

	a.statusLabel.Text = message
	a.statusLabel.Refresh()

	go func() {
		time.Sleep(duration)
		// Only reset if this status message hasn't been replaced
		if a.statusLabel.Text == message {
			a.statusLabel.Text = prevText
			a.statusLabel.Color = prevColor
			a.statusLabel.Refresh()
		}
	}()
}

// showMainWindow shows the main application window
func (a *App) showMainWindow() {
	a.mainWindow.Show()
	a.mainWindow.RequestFocus()
}

// showAboutDialog shows the about dialog
func (a *App) showAboutDialog() {
	a.mainWindow.Show()
	dialog.ShowInformation("About Ramble",
		"Ramble Speech-to-Text\nVersion 0.1.0\n\nTranscribe speech to text quickly and easily.",
		a.mainWindow)
}

// doQuit properly exits the application
func (a *App) doQuit() {
	// Call any user-defined quit handlers
	if a.onQuit != nil {
		a.onQuit()
	}

	// Stop system tray
	if a.systray != nil {
		a.systray.Stop()
	}

	// Quit the app
	if a.fyneApp != nil {
		a.fyneApp.Quit()
	}

	// Force exit if needed
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
}

// copyTranscript copies the transcript text to clipboard
func (a *App) copyTranscript() {
	text := a.transcriptBox.Text
	if text == "" || text == "Your transcription will appear here..." {
		a.ShowTemporaryStatus("Nothing to copy!", 2*time.Second)
		return
	}

	err := clipboard.SetText(text)
	if err != nil {
		logger.Error(logger.CategoryUI, "Failed to copy text to clipboard: %v", err)
		dialog.ShowError(fmt.Errorf("Failed to copy text: %v", err), a.mainWindow)
	} else {
		a.ShowTemporaryStatus("Copied to clipboard", 2*time.Second)
	}
}

// clearTranscript clears the transcript text
func (a *App) clearTranscript() {
	a.transcriptBox.SetText("")
	a.ShowTemporaryStatus("Transcript cleared", 2*time.Second)

	if a.onClearTranscript != nil {
		a.onClearTranscript()
	}
}

// showPreferencesDialog shows the preferences dialog
func (a *App) showPreferencesDialog() {
	a.mainWindow.Show()
	ShowPreferences(a.fyneApp, a.mainWindow, a.currentPreferences, func(prefs Preferences) {
		a.currentPreferences = prefs

		// Apply theme change immediately
		if prefs.DarkTheme {
			a.fyneApp.Settings().SetTheme(NewRambleTheme(true))
		} else {
			a.fyneApp.Settings().SetTheme(NewRambleTheme(false))
		}

		// Notify callback if set
		if a.onPreferencesChanged != nil {
			a.onPreferencesChanged(prefs)
		}
	})
}

// toggleHoverWindow toggles the hover window UI mode
func (a *App) toggleHoverWindow() {
	a.isHoverMode = !a.isHoverMode

	if a.isHoverMode {
		// Set the hover window's transcript to match the main window
		if a.transcriptBox.Text != "Your transcription will appear here..." {
			a.hoverWindow.UpdateTranscript(a.transcriptBox.Text)
		}

		// Set the recording state to match
		isRecording := (a.state == StateListening || a.state == StateTranscribing)
		a.hoverWindow.SetRecordingState(isRecording)

		// Show hover window and hide main window
		a.hoverWindow.Show()
		a.mainWindow.Hide()

		// Log and show status
		logger.Info(logger.CategoryUI, "Hover window activated")
		a.hoverWindow.ShowTemporaryStatus("Compact mode active", 1500*time.Millisecond)
	} else {
		// Hide hover window and show main window
		if a.hoverWindow != nil {
			a.hoverWindow.Hide()
		}
		a.mainWindow.Show()
		a.mainWindow.RequestFocus()
		logger.Info(logger.CategoryUI, "Hover window deactivated")
	}
}

// SetStartHidden sets whether the app should start hidden in the system tray
func (a *App) SetStartHidden(hidden bool) {
	a.startHidden = hidden
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

// GetPreferences returns the current preferences
func (a *App) GetPreferences() Preferences {
	return a.currentPreferences
}

// ShowErrorDialog displays an error dialog with title and message
func (a *App) ShowErrorDialog(title, message string) {
	dialog.ShowError(fmt.Errorf("%s", message), a.mainWindow)
}
