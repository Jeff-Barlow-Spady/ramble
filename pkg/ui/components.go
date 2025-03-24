// Package ui provides the user interface for the transcription app
package ui

import (
	"image/color"

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
	// Create a multiline entry that is properly configured for transcription display
	transcript := widget.NewMultiLineEntry()

	// Set proper wrapping
	transcript.Wrapping = fyne.TextWrapWord

	// Use clear placeholder text with high contrast
	transcript.SetPlaceHolder("Transcribed text will appear here (waiting for speech)...")

	// Use styling for better visibility - bold is set in the TextStyle struct
	transcript.TextStyle = fyne.TextStyle{
		Bold: true, // Make text bold for better visibility
	}

	// Make the text larger for better readability
	// Note: The actual text size comes from the theme, but we're using a custom theme
	// with larger text size in theme.go (1.2x normal size)

	// Set initial text to make sure it's working
	transcript.SetText("Ready for transcription. Press Record to start.")

	// Set read-only to false to allow text selection and copying
	// This improves usability while still preventing user editing
	transcript.DisableableWidget.Disable()
	transcript.Wrapping = fyne.TextWrapWord

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
	banner.Wrapping = fyne.TextWrapOff

	// Create a container for the banner with a light background
	bannerRect := canvas.NewRectangle(color.NRGBA{R: 40, G: 40, B: 40, A: 255})
	bannerContainer := container.NewStack(
		bannerRect,
		container.NewPadded(banner),
	)

	// Fix the size to ensure proper alignment
	bannerContainer.Resize(fyne.NewSize(900, 150))

	// Create a scroll container for the transcript
	transcriptScroll := container.NewVScroll(transcriptBox)

	// Make the transcript area with a reasonable height and better visibility
	// Use a colored border to make it more prominent
	transcriptBorder := canvas.NewRectangle(theme.PrimaryColor())
	transcriptBackground := canvas.NewRectangle(color.NRGBA{R: 30, G: 30, B: 35, A: 255})

	transcriptContainer := container.NewStack(
		transcriptBorder,
		container.NewPadded(transcriptBackground),
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

		// Transcript at the bottom - make it larger by giving it more weight
		container.NewPadded(transcriptContainer),
	)

	// Set height constraints to make the transcript larger - more space for text
	transcriptContainer.Resize(fyne.NewSize(900, 365))

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
