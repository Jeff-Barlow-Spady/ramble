// Package ui provides the user interface for the transcription app
package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// UIComponents holds all the UI components
type UIComponents struct {
	TranscriptBox *widget.Entry
	ListenButton  *widget.Button
	ClearButton   *widget.Button
	CopyButton    *widget.Button
	StatusLabel   *canvas.Text
	MainContent   *fyne.Container
}

// createTranscriptArea creates the main transcript text area
func createTranscriptArea() *widget.Entry {
	transcript := widget.NewMultiLineEntry()
	transcript.Wrapping = fyne.TextWrapWord
	transcript.SetPlaceHolder("Transcribed text will appear here...")

	// Make it read-only by disabling it in Fyne
	transcript.Disable()

	return transcript
}

// createControlPanel creates the panel with control buttons
func createControlPanel(
	onListen func(),
	onClear func(),
	onCopy func(),
) (*fyne.Container, *widget.Button, *widget.Button, *widget.Button) {
	// Create buttons with icons
	listenButton := widget.NewButtonWithIcon("Record", theme.MediaRecordIcon(), onListen)
	clearButton := widget.NewButtonWithIcon("Clear", theme.ContentClearIcon(), onClear)
	copyButton := widget.NewButtonWithIcon("Copy", theme.ContentCopyIcon(), onCopy)

	// Create control panel with buttons centered
	controlPanel := container.NewHBox(
		layout.NewSpacer(),
		listenButton,
		clearButton,
		copyButton,
		layout.NewSpacer(),
	)

	return controlPanel, listenButton, clearButton, copyButton
}

// createStatusBar creates the status bar with audio visualization
func createStatusBar(waveform *WaveformVisualizer) (*fyne.Container, *canvas.Text) {
	// Create status label with improved style
	statusLabel := canvas.NewText("Ready", theme.ForegroundColor())
	statusLabel.TextSize = 14
	statusLabel.Alignment = fyne.TextAlignCenter

	// Start the waveform animation immediately
	waveform.StartListening()
	waveform.SetAmplitude(0.3) // Give it some initial movement

	// Create a container with the waveform
	waveformContainer := container.NewPadded(waveform)

	// Make the waveform visually distinct with a background
	audioBox := container.NewStack(
		canvas.NewRectangle(theme.BackgroundColor()),
		waveformContainer,
	)

	// Create a fixed size to ensure the waveform has adequate space
	audioBox.Resize(fyne.NewSize(600, 150))

	// Create status bar with waveform, shortcut info, and status text
	statusBar := container.NewVBox(
		container.NewPadded(audioBox),
		widget.NewLabelWithStyle("Press 'R' to start/stop recording",
			fyne.TextAlignCenter, fyne.TextStyle{Italic: true}),
		container.NewHBox(layout.NewSpacer(), statusLabel, layout.NewSpacer()),
	)

	return statusBar, statusLabel
}

// createMainUI creates the main UI layout
func createMainUI(
	transcriptBox *widget.Entry,
	controlPanel *fyne.Container,
	statusBar *fyne.Container,
) *fyne.Container {
	// ASCII banner with improved alignment
	bannerText := `
	██████╗  █████╗ ███╗   ███╗██████╗ ██╗     ███████╗
	██╔══██╗██╔══██╗████╗ ████║██╔══██╗██║     ██╔════╝
	██████╔╝███████║██╔████╔██║██████╔╝██║     █████╗
	██╔══██╗██╔══██║██║╚██╔╝██║██╔══██╗██║     ██╔══╝
	██║  ██║██║  ██║██║ ╚═╝ ██║██████╔╝███████╗███████╗
	╚═╝  ╚═╝╚═╝  ╚═╝╚═╝     ╚═╝╚═════╝ ╚══════╝╚══════╝
						 Speech-to-Text Service
 `

	banner := widget.NewLabelWithStyle(bannerText, fyne.TextAlignCenter, fyne.TextStyle{Monospace: true})

	// Use a specific fixed width to ensure proper alignment
	bannerRect := canvas.NewRectangle(theme.BackgroundColor())

	// Stack the banner on top of the rectangle for proper alignment
	bannerContainer := container.NewStack(
		bannerRect,
		banner,
	)

	// Fix the size to ensure proper alignment
	bannerContainer.Resize(fyne.NewSize(900, 150))

	// Create a scroll container for the transcript
	transcriptScroll := container.NewVScroll(transcriptBox)

	// Make the transcript area with a reasonable height
	transcriptContainer := container.NewStack(
		canvas.NewRectangle(theme.BackgroundColor()),
		container.NewPadded(transcriptScroll),
	)

	// Create the main container with banner at top, waveform next, controls, and transcript at bottom
	mainContent := container.NewVBox(
		// Banner at the very top
		bannerContainer,

		// Waveform below the banner
		container.NewPadded(statusBar),

		// Control panel in the middle
		container.NewPadded(controlPanel),

		// Transcript at the bottom
		container.NewPadded(transcriptContainer),
	)

	// Set height constraints to make the transcript larger
	transcriptContainer.Resize(fyne.NewSize(900, 300))

	return mainContent
}

// CreateUI creates all UI components and returns them in a struct
func CreateUI(
	onListen func(),
	onClear func(),
	onCopy func(),
	waveform *WaveformVisualizer,
) UIComponents {
	// Create transcript area using the helper function
	transcriptBox := createTranscriptArea()

	// Create control panel using the helper function
	controlPanel, listenButton, clearButton, copyButton := createControlPanel(
		onListen,
		onClear,
		onCopy,
	)

	// Create status bar using the helper function
	statusBar, statusLabel := createStatusBar(waveform)

	// Create main UI layout using the helper function
	mainContent := createMainUI(
		transcriptBox,
		controlPanel,
		statusBar,
	)

	return UIComponents{
		TranscriptBox: transcriptBox,
		ListenButton:  listenButton,
		ClearButton:   clearButton,
		CopyButton:    copyButton,
		StatusLabel:   statusLabel,
		MainContent:   mainContent,
	}
}
