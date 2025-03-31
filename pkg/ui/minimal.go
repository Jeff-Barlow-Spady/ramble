package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/getlantern/systray"
)

type MinimalUI struct {
	app        fyne.App
	window     fyne.Window
	transcript *widget.Label
	recording  bool
}

func NewMinimalUI() *MinimalUI {
	a := app.New()
	w := a.NewWindow("Ramble")
	w.Resize(fyne.NewSize(400, 300))

	return &MinimalUI{
		app:        a,
		window:     w,
		transcript: widget.NewLabel(""),
	}
}

func (m *MinimalUI) Run(textChan <-chan string) {
	// Systray
	systray.Run(func() {
		systray.SetIcon(rambleIcon)
		systray.SetTitle("Ramble")
		systray.SetTooltip("Voice Transcription")

		show := systray.AddMenuItem("Show", "Show window")
		hide := systray.AddMenuItem("Hide", "Hide window")
		systray.AddSeparator()
		quit := systray.AddMenuItem("Quit", "Exit application")

		go func() {
			for {
				select {
				case <-show.ClickedCh:
					m.window.Show()
				case <-hide.ClickedCh:
					m.window.Hide()
				case <-quit.ClickedCh:
					systray.Quit()
					m.app.Quit()
				}
			}
		}()
	}, nil)

	// Main content
	var recordBtn *widget.Button
	recordBtn = widget.NewButton("Start Recording", func() {
		m.recording = !m.recording
		if m.recording {
			recordBtn.SetText("Stop Recording")
		} else {
			recordBtn.SetText("Start Recording")
		}
	})

	content := container.NewVBox(
		recordBtn,
		container.NewVScroll(m.transcript),
	)

	m.window.SetContent(content)

	// Update transcript
	go func() {
		for text := range textChan {
			m.transcript.SetText(text)
			m.window.Canvas().Refresh(m.transcript)
		}
	}()

	m.window.ShowAndRun()
}

var rambleIcon = []byte{ /* 1KB PNG icon bytes */ }
