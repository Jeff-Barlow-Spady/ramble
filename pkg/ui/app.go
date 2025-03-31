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

	"github.com/jeff-barlow-spady/ramble/internal/clipboard"
	"github.com/jeff-barlow-spady/ramble/pkg/logger"
	"github.com/jeff-barlow-spady/ramble/pkg/resources"
)

// AppState represents the current state of the application
type AppState int

const (
	StateIdle AppState = iota
	StateListening
	StateTranscribing
	StateError // Adding StateError constant
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

	// Hover window for compact UI
	hoverWindow *HoverWindow
	isHoverMode bool // Whether we're in hover mode

	// Start hidden in system tray
	startHidden bool

	// Callbacks for UI events
	onStartListening     func()
	onStopListening      func()
	onClearTranscript    func()
	onQuit               func()
	onPreferencesChanged func(Preferences)
}

// RunOnMain runs a function on the main thread
func RunOnMain(f func()) {
	// For Fyne apps, most operations are already run on the main thread automatically
	// This is a simple implementation that will be enhanced in future versions
	// When not on the main thread, we would ideally use a proper queue mechanism

	// In a production app, we would check if we're on the main thread
	// and use a channel or other mechanism to queue the function if not

	// For now, we directly execute the function
	// In most cases this is safe since Fyne handles UI operations on the main thread
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

	// Set window size to better accommodate the transcript display area
	// Increased width and height for better text readability
	mainWindow.Resize(fyne.NewSize(700, 900))

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
	// First call any user-defined quit handlers
	if a.onQuit != nil {
		a.onQuit()
	}

	// Ensure system tray is stopped before quitting the app
	if a.systray != nil {
		a.systray.Stop()
	}

	// Now quit the app itself
	if a.fyneApp != nil {
		a.fyneApp.Quit()
	}

	// If we're still here after attempting to quit, force exit
	// This ensures the application closes when requested
	go func() {
		// Wait a brief moment for graceful exit to complete
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
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

	// Create a high-contrast transcript box for better readability
	transcriptBox := widget.NewMultiLineEntry()
	transcriptBox.Disable() // Make read-only but still selectable
	transcriptBox.SetPlaceHolder("Ready for transcription. Press Record to start.")
	transcriptBox.Wrapping = fyne.TextWrapWord

	// Set a minimum size to ensure readability
	transcriptBox.SetMinRowsVisible(10)
	transcriptBox.TextStyle = fyne.TextStyle{
		Bold: true, // Make text bold for better visibility
	}

	// Create high-visibility buttons with clear labels
	// Record button - make it bright and obvious
	listenButton := widget.NewButtonWithIcon("Record", theme.MediaRecordIcon(), a.toggleListenState)
	listenButton.Importance = widget.HighImportance

	// Clear button - add "Transcript" to clarify purpose
	clearButton := widget.NewButtonWithIcon("Clear Transcript", theme.ContentClearIcon(), a.clearTranscript)

	// Copy button - add "to Clipboard" to clarify purpose
	copyButton := widget.NewButtonWithIcon("Copy to Clipboard", theme.ContentCopyIcon(), a.copyTranscript)

	// Status label with higher contrast
	statusLabel := canvas.NewText("Ready", color.NRGBA{R: 0, G: 180, B: 0, A: 255})
	statusLabel.TextSize = 16 // Larger text for better visibility

	// Store references to components
	a.transcriptBox = transcriptBox
	a.listenButton = listenButton
	a.clearButton = clearButton
	a.copyButton = copyButton
	a.statusLabel = statusLabel
	a.waveform = waveform

	// Create a clean button bar with more spacing between elements
	buttonBar := container.NewHBox(
		listenButton,
		layout.NewSpacer(),
		clearButton,
		copyButton,
	)

	// Create status bar with waveform
	statusBar := container.NewHBox(
		statusLabel,
		layout.NewSpacer(),
		container.NewWithoutLayout(waveform),
	)
	waveform.Resize(fyne.NewSize(200, 20))
	waveform.Move(fyne.NewPos(300, 0))

	// Main content with clear visual hierarchy
	mainContent := container.NewBorder(
		buttonBar,                          // Top
		statusBar,                          // Bottom
		nil,                                // Left
		nil,                                // Right
		container.NewScroll(transcriptBox), // Center - transcript with scrolling
	)

	// Set the window content
	a.mainWindow.SetContent(mainContent)

	// Create the hover window
	a.hoverWindow = NewHoverWindow(a.fyneApp)
	a.hoverWindow.SetCallbacks(
		a.toggleListenState, // Record/Stop
		a.copyTranscript,    // Copy
		func() { // Close
			a.hoverWindow.Hide()
			a.isHoverMode = false
		},
	)

	// Set up key press handling
	a.keyHandlerEnabled = true

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

	// Also register key handler for when window is not focused - global hotkey
	a.mainWindow.Canvas().SetOnTypedKey(func(ke *fyne.KeyEvent) {
		// Skip all keyboard handling if disabled or in test mode
		if !a.keyHandlerEnabled || a.isTestMode {
			return
		}

		// Check if we're focused on the transcript box (or any Entry widget)
		// Space key should NOT trigger recording when typing in text fields
		focused := a.mainWindow.Canvas().Focused()
		isEntryFocused := false
		if focused != nil {
			_, isEntryFocused = focused.(*widget.Entry)
		}

		// Only allow Space key when NOT typing in an entry widget
		if ke.Name == fyne.KeySpace && !isEntryFocused {
			a.keyHandlerEnabled = false // Temporarily disable to prevent rapid toggling

			// Toggle recording with the listenButton
			a.toggleListenState()

			// Re-enable after a delay to prevent accidental double-triggers
			go func() {
				time.Sleep(300 * time.Millisecond)
				a.keyHandlerEnabled = true
			}()
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
	// If we're starting hidden, don't call ShowAndRun
	if a.startHidden {
		// Just show the system tray
		a.fyneApp.Run()
	} else {
		a.mainWindow.ShowAndRun()
	}
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

	// Check if the current text is just the initial placeholder or empty
	initialText := "Ready for transcription. Press Record to start."
	isInitialOrEmpty := current == initialText || current == "" || current == "(Recording stopped)"

	// Handle the different cases
	if isInitialOrEmpty {
		// If this is just a recording stopped message and we're at initial state,
		// we want to preserve "(Recording stopped)" as a standalone message
		if isRecordingStopped && trimmedText == "(Recording stopped)" {
			fmt.Printf("UI: AppendTranscript - setting first text: '%s'\n", trimmedText)
			a.transcriptBox.SetText(trimmedText)
		} else {
			// Replace the initial text with the first transcription
			fmt.Printf("UI: AppendTranscript - replacing initial text with: '%s'\n", trimmedText)
			a.transcriptBox.SetText(trimmedText)
		}
	} else if isRecordingStopped && hasRecordingStopped {
		// If we already have a recording stopped message, we should not append another one
		fmt.Println("UI: AppendTranscript - already has recording stopped message, not appending")
	} else {
		// Handle spacing intelligently
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

	// Also update the hover window if it's active
	if a.isHoverMode && a.hoverWindow != nil {
		a.hoverWindow.AppendTranscript(trimmedText)
	}

	// Scroll to the end of the text
	a.transcriptBox.CursorRow = len(a.transcriptBox.Text)
	fmt.Println("UI: AppendTranscript - done updating text")
}

// UpdateAudioLevel updates the audio level in the waveform
func (a *App) UpdateAudioLevel(level float32) {
	// Only update UI components that are actively visible
	RunOnMain(func() {
		// Update main window waveform if visible
		if a.waveform != nil && a.mainWindow != nil && a.mainWindow.Content().Visible() {
			// Set a minimum amplitude to keep the waveform visible when idle
			if level > 0.01 {
				a.waveform.SetAmplitude(level)
			} else {
				a.waveform.SetAmplitude(0.05) // Small idle animation
			}
		}

		// Update hover window waveform if active and visible
		if a.isHoverMode && a.hoverWindow != nil {
			if level > 0.01 {
				a.hoverWindow.UpdateAudioLevel(level)
			} else {
				a.hoverWindow.UpdateAudioLevel(0.05)
			}
		}
	})
}

// SetState updates the application state and UI elements
func (a *App) SetState(state AppState) {
	// Store the state change immediately (this is thread-safe)
	a.state = state

	// Update system tray state (this should be thread-safe)
	a.systray.UpdateRecordingState(state == StateListening || state == StateTranscribing)

	// Use RunOnMain to safely update any UI elements on the main thread
	RunOnMain(func() {
		// Update hover window recording state if active
		if a.isHoverMode && a.hoverWindow != nil {
			a.hoverWindow.SetRecordingState(state == StateListening || state == StateTranscribing)
		}

		// Update the UI based on the new state - safely on the main thread
		switch state {
		case StateIdle:
			if a.statusLabel != nil {
				a.statusLabel.Text = "Ready"
				a.statusLabel.Color = color.NRGBA{R: 180, G: 180, B: 180, A: 255}
				a.statusLabel.Refresh()
			}

			// Only update window title if visible to prevent GLFW errors
			if a.mainWindow != nil && a.mainWindow.Content().Visible() {
				a.mainWindow.SetTitle("Ramble")
			}

			if a.listenButton != nil {
				a.listenButton.SetText("Record")
				a.listenButton.SetIcon(theme.MediaRecordIcon())
			}
		case StateListening:
			if a.statusLabel != nil {
				a.statusLabel.Text = "● RECORDING"
				a.statusLabel.Color = color.RGBA{R: 255, G: 0, B: 0, A: 255}
				a.statusLabel.Refresh()
			}

			// Only update window title if visible to prevent GLFW errors
			if a.mainWindow != nil && a.mainWindow.Content().Visible() {
				a.mainWindow.SetTitle("Ramble - Recording...")
			}

			if a.listenButton != nil {
				a.listenButton.SetText("Stop")
				a.listenButton.SetIcon(theme.MediaStopIcon())
			}
		case StateTranscribing:
			if a.statusLabel != nil {
				a.statusLabel.Text = "Transcribing..."
				a.statusLabel.Color = color.RGBA{R: 255, G: 165, B: 0, A: 255}
				a.statusLabel.Refresh()
			}

			if a.listenButton != nil {
				a.listenButton.SetText("Stop")
				a.listenButton.SetIcon(theme.MediaStopIcon())
			}
		case StateError:
			if a.statusLabel != nil {
				a.statusLabel.Text = "Error"
				a.statusLabel.Color = color.RGBA{R: 255, G: 0, B: 0, A: 255}
				a.statusLabel.Refresh()
			}

			// Only update window title if visible to prevent GLFW errors
			if a.mainWindow != nil && a.mainWindow.Content().Visible() {
				a.mainWindow.SetTitle("Ramble - Error")
			}

			if a.listenButton != nil {
				a.listenButton.SetText("Record")
				a.listenButton.SetIcon(theme.MediaRecordIcon())
			}
		}
	})
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

	// Use a distinct color for clipboard-related messages to make them more noticeable
	if strings.Contains(status, "clipboard") || strings.Contains(status, "copied") {
		a.statusLabel.Color = color.NRGBA{R: 50, G: 200, B: 50, A: 255} // Bright green for copy/paste actions
	} else {
		a.statusLabel.Color = color.NRGBA{R: 220, G: 220, B: 0, A: 255} // Yellow for other status messages
	}

	a.statusLabel.Text = status
	a.statusLabel.Refresh()

	go func() {
		time.Sleep(duration)
		// Only reset if this status message hasn't been replaced by another one
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
	if text == "" || text == "Ready for transcription. Press Record to start." {
		a.ShowTemporaryStatus("No text to copy", 2*time.Second)
		return
	}

	// Log that we're attempting to copy
	logger.Info(logger.CategoryUI, "Attempting to copy text to clipboard: %d characters", len(text))

	err := clipboard.SetText(text)
	if err != nil {
		// Log detailed error
		logger.Error(logger.CategoryUI, "Failed to copy text to clipboard: %v", err)

		// Show error to user
		a.showError("Clipboard Error", fmt.Sprintf("Failed to copy text: %v\nPlease try manually selecting and copying the text.", err))

		// Enable the text widget for manual selection
		a.transcriptBox.Enable()
		a.transcriptBox.FocusGained() // Focus on the entry to make it easier to select
		a.ShowTemporaryStatus("Please select and copy the text manually (Ctrl+C)", 5*time.Second)
	} else {
		// Log success
		logger.Info(logger.CategoryUI, "Successfully copied %d characters to clipboard", len(text))

		// Show success message to user
		a.ShowTemporaryStatus("Text copied to clipboard", 2*time.Second)

		// Ensure disabled state is restored if we enabled for manual selection
		a.transcriptBox.Disable()
	}
}

// insertAtCursor copies the transcript text to clipboard and simulates a paste command
// This gets closer to a true "insert at cursor" functionality by both copying text
// and using keyboard automation to trigger the paste operation
func (a *App) insertAtCursor() {
	text := a.transcriptBox.Text
	if text == "" || text == "Ready for transcription. Press Record to start." {
		// Don't copy empty or default text
		a.ShowTemporaryStatus("No transcription text to insert", 2*time.Second)
		return
	}

	// Log that we're attempting to copy
	logger.Info(logger.CategoryUI, "Attempting to copy text for 'Insert at Cursor': %d characters", len(text))

	// Copy the text to clipboard
	err := clipboard.SetText(text)
	if err != nil {
		// Log detailed error
		logger.Error(logger.CategoryUI, "Failed to copy text for insertion: %v", err)

		// Show error to user
		a.showError("Clipboard Error", fmt.Sprintf("Failed to copy text: %v\nPlease try manually selecting and copying the text.", err))

		// Enable the text widget for manual selection
		a.transcriptBox.Enable()
		a.transcriptBox.FocusGained() // Focus on the entry to make it easier to select
		a.ShowTemporaryStatus("Please select and copy the text manually (Ctrl+C)", 5*time.Second)
		return
	}

	// Log success
	logger.Info(logger.CategoryUI, "Successfully copied %d characters for insertion", len(text))

	// Show a clear, more readable message with better timing for ADHD users
	a.ShowTemporaryStatus("✓ Text copied! Click where you want to paste", 3*time.Second)

	// First, ensure the window is visible so the user can see instructions
	a.mainWindow.Show()
	a.mainWindow.RequestFocus()

	// Use a more appropriate timing sequence for users with ADHD
	go func() {
		// Wait just long enough to read the message, but not so long it becomes distracting
		time.Sleep(2500 * time.Millisecond)

		// Give a countdown warning before hiding
		a.ShowTemporaryStatus("Window hiding in 3...", 1000*time.Millisecond)
		time.Sleep(1000 * time.Millisecond)
		a.ShowTemporaryStatus("Window hiding in 2...", 1000*time.Millisecond)
		time.Sleep(1000 * time.Millisecond)
		a.ShowTemporaryStatus("Window hiding in 1...", 1000*time.Millisecond)
		time.Sleep(1000 * time.Millisecond)

		// Hide the window to allow the user to focus on their target application
		a.mainWindow.Hide()

		// Log a notification message
		logger.Info(logger.CategoryUI, "Ready to paste: Click where you want to paste and press Ctrl+V")
	}()
}

// attemptToPaste tries to simulate a keyboard paste operation (Ctrl+V)
// This is a helper function that may not work on all systems due to security restrictions
func attemptToPaste() {
	// Use our platform-specific paste simulation
	err := SimulatePaste()
	if err != nil {
		logger.Error(logger.CategoryUI, "Failed to simulate paste: %v", err)
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
	// Make the toggle behavior more predictable and reliable
	if a.state == StateListening || a.state == StateTranscribing {
		// If we're in any recording state, go to idle
		a.SetState(StateIdle)
		if a.onStopListening != nil {
			a.onStopListening()
		}
		a.waveform.StopListening()

		// Re-enable keyboard shortcuts when not recording
		a.keyHandlerEnabled = true

		// Update hover window if it exists
		if a.hoverWindow != nil {
			a.hoverWindow.SetRecordingState(false)
		}
	} else {
		// Otherwise, start listening
		a.SetState(StateListening)
		if a.onStartListening != nil {
			a.onStartListening()
		}
		a.waveform.StartListening()

		// Completely disable keyboard shortcuts during recording to prevent accidental stopping
		a.keyHandlerEnabled = false

		// Update hover window if it exists
		if a.hoverWindow != nil {
			a.hoverWindow.SetRecordingState(true)
		}
	}
}

// toggleHoverWindow toggles the hover window UI mode
func (a *App) toggleHoverWindow() {
	a.isHoverMode = !a.isHoverMode

	if a.isHoverMode {
		// Make sure the hover window exists and is properly initialized
		if a.hoverWindow == nil {
			a.hoverWindow = NewHoverWindow(a.fyneApp)
			a.hoverWindow.SetCallbacks(
				a.toggleListenState, // Record/Stop
				a.copyTranscript,    // Copy
				func() { // Close
					a.hoverWindow.Hide()
					a.isHoverMode = false
				},
			)
		}

		// Set the hover window's transcript to match the main window
		if a.transcriptBox.Text != "Ready for transcription. Press Record to start." {
			a.hoverWindow.UpdateTranscript(a.transcriptBox.Text)
		}

		// Set the recording state to match the current state
		isRecording := (a.state == StateListening || a.state == StateTranscribing)
		a.hoverWindow.SetRecordingState(isRecording)

		// Set the audio level to match the current level
		if isRecording {
			a.hoverWindow.UpdateAudioLevel(0.5) // Start with medium level
		} else {
			a.hoverWindow.UpdateAudioLevel(0.1) // Low idle level
		}

		// Show the hover window (make sure it's visible)
		a.hoverWindow.Show()

		// Hide the main window if we're showing the hover window
		a.mainWindow.Hide()

		// Log that the hover window was activated
		logger.Info(logger.CategoryUI, "Hover window activated")

		// Show a status message briefly
		a.hoverWindow.ShowTemporaryStatus("Compact mode active", 1500*time.Millisecond)
	} else {
		// Hide the hover window
		if a.hoverWindow != nil {
			a.hoverWindow.Hide()
			logger.Info(logger.CategoryUI, "Hover window deactivated")
		}

		// Show the main window
		a.mainWindow.Show()
		a.mainWindow.RequestFocus()
	}
}

// IsWindowFocused returns true if the main window is currently focused
func (a *App) IsWindowFocused() bool {
	// Check if the main window exists and is focused
	if a.mainWindow == nil {
		return false
	}

	// In Fyne, we can check if any widget is focused as a proxy for window focus
	return a.mainWindow.Canvas().Focused() != nil || a.mainWindow.Content().Visible()
}

// SetStartHidden sets whether the app should start hidden in the system tray
func (a *App) SetStartHidden(hidden bool) {
	a.startHidden = hidden
}

// IsHoverActive returns true if the hover window is currently active
func (a *App) IsHoverActive() bool {
	return a.isHoverMode
}

// ShowHoverWindow shows the hover window and sets isHoverMode to true
func (a *App) ShowHoverWindow() {
	// Only proceed if not already in hover mode
	if a.isHoverMode {
		return
	}

	// Toggle to hover mode
	a.toggleHoverWindow()
}

// MakeWindowMinimized ensures the GLFW context is initialized
// without actually showing the window to the user
func (a *App) MakeWindowMinimized() {
	if a.mainWindow != nil {
		// Show and then hide to ensure GLFW is initialized
		a.mainWindow.Show()
		// Immediately hide again
		a.mainWindow.Hide()
	}
}
