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
	InsertButton  *widget.Button
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
	onInsert func(),
) (*fyne.Container, *widget.Button, *widget.Button, *widget.Button, *widget.Button) {
	// Create more subtle buttons with icons but less prominent styling
	listenButton := widget.NewButtonWithIcon("Record", theme.MediaRecordIcon(), onListen)
	clearButton := widget.NewButtonWithIcon("Clear", theme.ContentClearIcon(), onClear)
	copyButton := widget.NewButtonWithIcon("Copy", theme.ContentCopyIcon(), onCopy)
	insertButton := widget.NewButtonWithIcon("Insert at Cursor", theme.ContentPasteIcon(), onInsert)

	// Create vertical button stack with subtle separators and more compact spacing
	controlPanel := container.NewVBox(
		listenButton,
		widget.NewSeparator(), // Add separator for visual spacing
		clearButton,
		widget.NewSeparator(), // Add separator for visual spacing
		copyButton,
		widget.NewSeparator(), // Add separator for visual spacing
		insertButton,
		layout.NewSpacer(), // Push buttons to the top
	)

	// Use less padding for a more compact control panel
	paddedPanel := container.NewPadded(controlPanel)

	return paddedPanel, listenButton, clearButton, copyButton, insertButton
}

// createStatusBar creates the status bar with audio visualization
func createStatusBar(waveform *WaveformVisualizer) (*fyne.Container, *canvas.Text) {
	// Create status label with improved style but less prominent
	statusLabel := canvas.NewText("Ready", color.NRGBA{R: 220, G: 220, B: 220, A: 255})
	statusLabel.TextSize = 14
	statusLabel.Alignment = fyne.TextAlignCenter

	// Start the waveform animation immediately with higher amplitude
	waveform.StartListening()
	waveform.SetAmplitude(0.5) // Higher initial amplitude for more visibility

	// Create a container specifically for the waveform with a height that ensures visibility
	waveformBg := canvas.NewRectangle(color.NRGBA{R: 20, G: 40, B: 80, A: 255})

	// Stack the waveform on top of the background
	waveformStack := container.NewStack(waveformBg, waveform)

	// Create a container for the waveform that ensures proper height
	waveformContainer := container.NewStack(waveformStack)

	// Create a proper height for the waveform - significantly taller
	waveformStack.Resize(fyne.NewSize(600, 80))

	// Create a status bar with the waveform at the bottom of the application
	statusBar := container.NewVBox(
		container.NewBorder(
			nil, nil, // No top/bottom in this inner container
			nil,         // No left component
			statusLabel, // Status on right
			nil,         // Nothing in center
		),
		waveformContainer, // Waveform gets its own full row for maximum height
	)

	// Use minimal padding to maximize waveform visibility
	paddedStatusBar := container.NewPadded(statusBar)

	return paddedStatusBar, statusLabel
}

// createMainUI creates the main UI layout
func createMainUI(
	transcriptBox *widget.Entry,
	controlPanel *fyne.Container,
	statusBar *fyne.Container,
) *fyne.Container {
	// Create a deep blue background for the entire application
	mainBg := canvas.NewRectangle(color.NRGBA{R: 8, G: 15, B: 35, A: 255})

	// Compressed ASCII banner designed to fit in sidebar
	bannerText := `
██████╗  █████╗ ███╗   ███╗██████╗ ██╗     ███████╗
██╔══██╗██╔══██╗████╗ ████║██╔══██╗██║     ██╔════╝
██████╔╝███████║██╔████╔██║██████╔╝██║     █████╗
██╔══██╗██╔══██║██║╚██╔╝██║██╔══██╗██║     ██╔══╝
██║  ██║██║  ██║██║ ╚═╝ ██║██████╔╝███████╗███████╗
`

	// Important: Use a widget.Label instead of canvas.Text for proper ASCII art rendering
	banner := widget.NewLabelWithStyle(bannerText, fyne.TextAlignCenter,
		fyne.TextStyle{Monospace: true, Bold: true})

	// Set wrapping off to preserve ASCII art formatting
	banner.Wrapping = fyne.TextWrapOff

	// Add a simple subtitle
	subtitle := canvas.NewText("Speech-to-Text Service", color.White)
	subtitle.TextStyle = fyne.TextStyle{Bold: true}
	subtitle.TextSize = 12
	subtitle.Alignment = fyne.TextAlignCenter

	// Create a container for the banner with padding and distinctive background
	bannerBox := container.NewVBox(
		banner,
		subtitle,
	)

	// Create a fixed size container for the banner that fits properly
	bannerContainer := container.NewStack(
		canvas.NewRectangle(color.NRGBA{R: 15, G: 25, B: 60, A: 255}),
		container.NewPadded(bannerBox),
	)

	// Make the banner container larger to ensure visibility
	bannerContainer.Resize(fyne.NewSize(200, 130))

	// Create a smaller, less prominent sidebar with standard button layout
	paddedPanel := container.NewPadded(controlPanel)

	// Use a border layout to create the sidebar with banner at top and buttons below
	sidebar := container.NewBorder(
		bannerContainer, // Banner at the top
		nil, nil, nil,   // No elements for bottom/left/right
		paddedPanel, // Control panel fills the rest
	)

	// Create a visually distinct sidebar background that's less prominent
	sidebarBg := canvas.NewRectangle(color.NRGBA{R: 10, G: 18, B: 40, A: 255})
	sidebarContainer := container.NewStack(
		sidebarBg,
		sidebar,
	)

	// Make the sidebar narrower to give more space to the transcript
	sidebarContainer.Resize(fyne.NewSize(180, 800))

	// Create a scroll container for the transcript
	transcriptScroll := container.NewVScroll(transcriptBox)

	// Create transcript container with border and better contrast
	transcriptBorder := canvas.NewRectangle(color.NRGBA{R: 45, G: 85, B: 135, A: 255})
	transcriptBackground := canvas.NewRectangle(color.NRGBA{R: 15, G: 35, B: 60, A: 255})

	transcriptContainer := container.NewStack(
		transcriptBorder,
		container.NewPadded(transcriptBackground),
		container.NewPadded(transcriptScroll),
	)

	// Create a container for the transcript area with the status bar at the bottom
	rightSide := container.NewBorder(
		nil,       // No top component
		statusBar, // Status bar at bottom
		nil, nil,  // No left/right components
		transcriptContainer, // Transcript takes remaining space
	)

	// Create the main layout with horizontal split between sidebar and content
	mainLayout := container.NewHSplit(
		sidebarContainer, // Left sidebar (fixed width)
		rightSide,        // Right side (transcript + status)
	)

	// Set the split position to give the sidebar only 15% of the width (smaller than before)
	mainLayout.Offset = 0.15

	// Create the final container with background
	finalContainer := container.NewStack(
		mainBg,
		mainLayout,
	)

	return finalContainer
}

// CreateUI creates all UI components and returns them in a struct
func CreateUI(
	onListen func(),
	onClear func(),
	onCopy func(),
	onInsert func(),
	waveform *WaveformVisualizer,
) UIComponents {
	// Create transcript area using the helper function
	transcriptBox := createTranscriptArea()

	// Create control panel using the helper function
	controlPanel, listenButton, clearButton, copyButton, insertButton := createControlPanel(
		onListen,
		onClear,
		onCopy,
		onInsert,
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
		InsertButton:  insertButton,
		StatusLabel:   statusLabel,
		MainContent:   mainContent,
	}
}
