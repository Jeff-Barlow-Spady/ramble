// Package ui provides the user interface for the transcription app
package ui

import (
	"image/color"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// HoverWindow represents a compact floating window for transcription
type HoverWindow struct {
	window             fyne.Window
	transcriptBox      *widget.Entry
	waveform           *WaveformVisualizer
	recordButton       *widget.Button
	copyButton         *widget.Button
	closeButton        *widget.Button
	statusLabel        *canvas.Text
	isRecording        bool
	onRecordToggle     func()
	onCopy             func()
	onClose            func()
	defaultX, defaultY int
}

// NewHoverWindow creates a new compact hover window
func NewHoverWindow(app fyne.App) *HoverWindow {
	// Create a new window that will float above others
	window := app.NewWindow("Ramble Transcription")
	window.SetCloseIntercept(func() {
		// Hide instead of close when 'X' is clicked
		window.Hide()
	})

	// Set a more compact size for the hover window
	window.Resize(fyne.NewSize(300, 200))

	// Create a new hover window instance
	hw := &HoverWindow{
		window:      window,
		isRecording: false,
		defaultX:    100,
		defaultY:    100,
	}

	// Initialize UI components
	hw.createUI()

	// Position the window in a sensible default location
	window.CenterOnScreen()

	return hw
}

// createUI initializes the user interface components
func (hw *HoverWindow) createUI() {
	// Create the transcript text area (read-only)
	hw.transcriptBox = widget.NewMultiLineEntry()
	hw.transcriptBox.Disable() // Makes it read-only
	hw.transcriptBox.SetText("Ready for transcription...")
	hw.transcriptBox.Wrapping = fyne.TextWrapWord

	// Make the font size smaller for the compact view
	hw.transcriptBox.TextStyle = fyne.TextStyle{Monospace: true}

	// Create small waveform visualizer
	hw.waveform = NewWaveformVisualizer(theme.PrimaryColor())
	hw.waveform.StartListening()
	hw.waveform.SetAmplitude(0.1) // Small initial amplitude

	// Create the status label - smaller font
	hw.statusLabel = canvas.NewText("Ready", color.NRGBA{R: 150, G: 150, B: 150, A: 255})
	hw.statusLabel.TextSize = 10

	// Create control buttons - more compact with icons only
	hw.recordButton = widget.NewButtonWithIcon("", theme.MediaRecordIcon(), func() {
		if hw.onRecordToggle != nil {
			hw.onRecordToggle()
		}
	})

	hw.copyButton = widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
		if hw.onCopy != nil {
			hw.onCopy()
		}
	})

	hw.closeButton = widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
		hw.Hide()
		if hw.onClose != nil {
			hw.onClose()
		}
	})

	// Make buttons smaller
	buttonSize := fyne.NewSize(24, 24)
	hw.recordButton.Resize(buttonSize)
	hw.copyButton.Resize(buttonSize)
	hw.closeButton.Resize(buttonSize)

	// Create a compact button bar
	buttonBar := container.NewHBox(
		hw.recordButton,
		layout.NewSpacer(),
		hw.copyButton,
		layout.NewSpacer(),
		hw.closeButton,
	)

	// Create a very compact status bar with waveform
	statusBar := container.NewBorder(
		nil, nil, hw.statusLabel,
		container.NewWithoutLayout(hw.waveform),
	)
	hw.waveform.Resize(fyne.NewSize(40, 16))
	hw.waveform.Move(fyne.NewPos(statusBar.Size().Width-50, 0))

	// Create the main layout with minimal padding
	content := container.NewBorder(
		nil, // top
		container.NewVBox(
			buttonBar,
			container.NewPadded(statusBar),
		), // bottom
		nil,                                   // left
		nil,                                   // right
		container.NewPadded(hw.transcriptBox), // center
	)

	hw.window.SetContent(content)
}

// Show displays the hover window
func (hw *HoverWindow) Show() {
	hw.window.Show()
}

// Hide hides the hover window
func (hw *HoverWindow) Hide() {
	hw.window.Hide()
}

// UpdateTranscript updates the transcript text
func (hw *HoverWindow) UpdateTranscript(text string) {
	if text == "" {
		hw.transcriptBox.SetText("Ready for transcription...")
	} else {
		hw.transcriptBox.SetText(text)
	}
}

// AppendTranscript adds text to the current transcript
func (hw *HoverWindow) AppendTranscript(text string) {
	current := hw.transcriptBox.Text

	// Replace the placeholder if it's the initial text
	if current == "Ready for transcription..." {
		hw.transcriptBox.SetText(text)
		return
	}

	// Otherwise append with proper spacing
	newText := current
	if !strings.HasSuffix(current, "\n") && !strings.HasPrefix(text, "\n") {
		newText += " "
	}
	newText += text
	hw.transcriptBox.SetText(newText)
}

// UpdateAudioLevel updates the waveform visualization
func (hw *HoverWindow) UpdateAudioLevel(level float32) {
	hw.waveform.SetAmplitude(level)
}

// SetRecordingState updates the UI to reflect recording state
func (hw *HoverWindow) SetRecordingState(isRecording bool) {
	hw.isRecording = isRecording
	if isRecording {
		hw.recordButton.SetIcon(theme.MediaStopIcon())
		hw.statusLabel.Text = "Recording..."
		hw.statusLabel.Color = color.RGBA{R: 255, G: 0, B: 0, A: 255}
	} else {
		hw.recordButton.SetIcon(theme.MediaRecordIcon())
		hw.statusLabel.Text = "Ready"
		hw.statusLabel.Color = color.NRGBA{R: 150, G: 150, B: 150, A: 255}
	}
	hw.statusLabel.Refresh()
}

// ShowTemporaryStatus shows a temporary status message
func (hw *HoverWindow) ShowTemporaryStatus(status string, duration time.Duration) {
	prevText := hw.statusLabel.Text
	prevColor := hw.statusLabel.Color

	// Use a distinct color for the message
	hw.statusLabel.Text = status
	hw.statusLabel.Color = color.NRGBA{R: 220, G: 220, B: 0, A: 255}
	hw.statusLabel.Refresh()

	// Reset after the specified duration
	go func() {
		time.Sleep(duration)
		// Only reset if the message hasn't been changed
		if hw.statusLabel.Text == status {
			hw.statusLabel.Text = prevText
			hw.statusLabel.Color = prevColor
			hw.statusLabel.Refresh()
		}
	}()
}

// SetCallbacks sets callbacks for the hover window controls
func (hw *HoverWindow) SetCallbacks(onRecord, onCopy, onClose func()) {
	hw.onRecordToggle = onRecord
	hw.onCopy = onCopy
	hw.onClose = onClose
}
