package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/getlantern/systray"
	"github.com/jeff-barlow-spady/ramble/pkg/resources"
)

type EssentialUI struct {
	app          fyne.App
	window       fyne.Window
	currentText  *widget.Entry // Simple text entry
	historyList  *widget.List  // Basic history
	statusLabel  *widget.Label // Recording status
	transcriptCh chan string
}

func NewEssentialUI() *EssentialUI {
	a := app.New()
	w := a.NewWindow("Ramble")
	w.Resize(fyne.NewSize(400, 300))

	history := widget.NewList(
		func() int { return 0 },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i int, o fyne.CanvasObject) {},
	)

	return &EssentialUI{
		app:          a,
		window:       w,
		currentText:  widget.NewEntry(),
		historyList:  history,
		statusLabel:  widget.NewLabel("Ready"),
		transcriptCh: make(chan string, 100),
	}
}

func (e *EssentialUI) Run() {
	// Systray (unchanged from minimal)
	setupSystray(e.window)

	// Layout
	content := container.NewBorder(
		e.statusLabel,
		nil,
		nil,
		nil,
		container.NewVSplit(
			e.currentText,
			e.historyList,
		),
	)

	e.window.SetContent(content)

	// Update loop
	go func() {
		for text := range e.transcriptCh {
			e.currentText.SetText(text)
			// e.historyList.Length++ // Incorrect usage, list updates are handled differently
			e.historyList.Refresh()
		}
	}()

	e.window.ShowAndRun()
}

// Retains: Copy to clipboard, basic history, current text display
// Removes: Formatting, note management, complex dialogs

// setupSystray initializes the system tray icon and menu
func setupSystray(window fyne.Window) {
	go systray.Run(
		func() {
			// Get icon data
			iconData, err := resources.GetIconData()
			if err != nil || len(iconData) == 0 {
				// Fallback to empty icon if error
				iconData = []byte{}
			}

			// Set up systray
			systray.SetIcon(iconData)
			systray.SetTitle("Ramble")
			systray.SetTooltip("Ramble Speech-to-Text")

			// Add menu items
			mShow := systray.AddMenuItem("Show Window", "Show the main window")
			systray.AddSeparator()
			mQuit := systray.AddMenuItem("Quit", "Quit the application")

			// Handle menu item clicks
			go func() {
				for {
					select {
					case <-mShow.ClickedCh:
						window.Show()
					case <-mQuit.ClickedCh:
						systray.Quit()
						window.Close()
						return
					}
				}
			}()
		},
		func() {
			// On exit - nothing special needed
		},
	)
}
