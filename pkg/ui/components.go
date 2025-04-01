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
	TranscriptBox     *widget.Entry
	StreamingPreview  *widget.Entry
	FinalizedSegments *fyne.Container
	ListenButton      *widget.Button
	ClearButton       *widget.Button
	CopyButton        *widget.Button
	InsertButton      *widget.Button
	StatusLabel       *canvas.Text
	MainContent       *fyne.Container
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

// createStreamingPreviewArea creates the streaming preview area for real-time transcription updates
func createStreamingPreviewArea() *widget.Entry {
	// Create a multiline entry that is properly configured for streaming transcription preview
	preview := widget.NewMultiLineEntry()

	// Set proper wrapping
	preview.Wrapping = fyne.TextWrapWord

	// Use clear placeholder text with high contrast
	preview.SetPlaceHolder("Live transcription will appear here...")

	// Use styling for better visibility
	preview.TextStyle = fyne.TextStyle{
		Italic: true, // Indicate this is not final text
	}

	// Make the text slightly smaller than the main transcript
	// The actual text size comes from the theme

	// Set initial text
	preview.SetText("Waiting for speech...")

	// Make read-only but still allow selection
	preview.DisableableWidget.Disable()

	return preview
}

// createFinalizedSegmentsContainer creates a container to hold finalized transcription segments
func createFinalizedSegmentsContainer() *fyne.Container {
	// Create a vertical box container that will hold the finalized transcription "cards"
	segmentsContainer := container.NewVBox()

	// Wrap in a scroll container to handle many segments
	scrollContainer := container.NewVScroll(segmentsContainer)

	// Return the scroll container wrapped in a regular container to match the return type
	return container.NewMax(scrollContainer)
}

// createTranscriptionSegmentCard creates an individual card for a finalized transcription segment
func createTranscriptionSegmentCard(text string, onDelete func(), onSave func()) *fyne.Container {
	// Create the text display with better styling
	textLabel := widget.NewLabel(text)
	textLabel.Wrapping = fyne.TextWrapWord
	textLabel.TextStyle = fyne.TextStyle{
		Bold: true, // Make text bold for better readability
	}

	// Create action buttons with clearer labels and larger size
	deleteButton := widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), onDelete)
	deleteButton.Importance = widget.WarningImportance

	saveButton := widget.NewButtonWithIcon("Copy", theme.ContentCopyIcon(), onSave)
	saveButton.Importance = widget.HighImportance

	// Create a horizontal container for buttons with better spacing
	buttonContainer := container.NewHBox(
		layout.NewSpacer(),
		saveButton,
		container.NewPadded(widget.NewSeparator()),
		deleteButton,
	)

	// Create a card with a border and background that's more visually distinct
	background := canvas.NewRectangle(color.NRGBA{R: 40, G: 50, B: 80, A: 255})

	// Create a border for the card
	border := canvas.NewRectangle(color.NRGBA{R: 60, G: 70, B: 100, A: 255})

	// Create the content with padding
	content := container.NewVBox(
		container.NewPadded(textLabel),
		widget.NewSeparator(), // Add a separator between text and buttons
		buttonContainer,
	)

	paddedContent := container.NewPadded(content)

	// Use a border layout to create a more distinct card effect
	card := container.New(layout.NewMaxLayout(),
		border,
		container.NewPadded(background),
		paddedContent,
	)

	return card
}

// createMainUI creates the main UI layout
func createMainUI(
	transcriptBox *widget.Entry,
	streamingPreview *widget.Entry,
	finalizedSegments *fyne.Container,
	controlPanel *fyne.Container,
	statusBar *fyne.Container,
) *fyne.Container {

	// Create a tabbed container to separate the different views
	tabs := container.NewAppTabs(
		container.NewTabItem("Two-Stage View",
			container.NewBorder(
				nil,
				statusBar,
				nil,
				nil,
				container.NewVSplit(
					streamingPreview,
					finalizedSegments,
				),
			),
		),
		container.NewTabItem("Classic View",
			container.NewBorder(
				nil,
				statusBar,
				nil,
				nil,
				transcriptBox,
			),
		),
	)

	// Create the main layout with control panel at the top
	mainLayout := container.NewBorder(
		controlPanel, // top
		nil,          // bottom (status bar is in the tab content)
		nil,          // left
		nil,          // right
		tabs,         // center content
	)

	return mainLayout
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

	// Create streaming preview area
	streamingPreview := createStreamingPreviewArea()

	// Create finalized segments container
	finalizedSegments := createFinalizedSegmentsContainer()

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
		streamingPreview,
		finalizedSegments,
		controlPanel,
		statusBar,
	)

	return UIComponents{
		TranscriptBox:     transcriptBox,
		StreamingPreview:  streamingPreview,
		FinalizedSegments: finalizedSegments,
		ListenButton:      listenButton,
		ClearButton:       clearButton,
		CopyButton:        copyButton,
		InsertButton:      insertButton,
		StatusLabel:       statusLabel,
		MainContent:       mainContent,
	}
}
