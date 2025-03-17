package ui

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// Preferences represents application preferences
type Preferences struct {
	// Audio settings
	SampleRate      float64
	Channels        int
	FramesPerBuffer int

	// Appearance settings
	MinimizeToTray bool
	DarkTheme      bool

	// Hotkey settings
	HotkeyModifiers []string
	HotkeyKey       string

	// Behavior settings
	AutoCopy        bool
	SaveTranscripts bool
	TranscriptPath  string
	StartMinimized  bool
	TestMode        bool

	// Transcription settings
	ModelSize string
}

// DefaultPreferences returns the default preferences
func DefaultPreferences() Preferences {
	return Preferences{
		SampleRate:      16000,
		Channels:        1,
		FramesPerBuffer: 1024,
		MinimizeToTray:  true,
		DarkTheme:       true,
		HotkeyModifiers: []string{"ctrl", "shift"},
		HotkeyKey:       "s",
		AutoCopy:        false,
		SaveTranscripts: false,
		TranscriptPath:  "",
		StartMinimized:  false,
		TestMode:        false,
		ModelSize:       "small",
	}
}

// PreferencesDialog represents the preferences/settings dialog
type PreferencesDialog struct {
	app    *App
	window fyne.Window
	onSave func(Preferences)
	prefs  Preferences
}

// Global variable to track the preferences window
var preferencesWindow fyne.Window

// ShowPreferences shows the preferences dialog
func (a *App) ShowPreferences(onSave func(Preferences)) {
	// If preferences window already exists, bring it to front
	if preferencesWindow != nil {
		preferencesWindow.Show()
		preferencesWindow.RequestFocus()
		return
	}

	// Create a new window for preferences
	w := a.fyneApp.NewWindow("Ramble Preferences")
	w.Resize(fyne.NewSize(500, 450))

	// Store reference to the window
	preferencesWindow = w

	// Set close handler to clear the reference
	w.SetCloseIntercept(func() {
		preferencesWindow = nil
		w.Close()
	})

	// Create dialog
	dialog := &PreferencesDialog{
		app:    a,
		window: w,
		onSave: onSave,
		prefs:  a.currentPreferences, // Use current preferences as starting point
	}

	// Set up the UI
	dialog.setupUI()

	// Show the window
	w.Show()
}

// setupUI creates all the UI elements for the preferences dialog
func (d *PreferencesDialog) setupUI() {
	// Create tabs for different settings categories
	tabs := container.NewAppTabs(
		container.NewTabItem("General", d.createGeneralTab()),
		container.NewTabItem("Audio", d.createAudioTab()),
		container.NewTabItem("Hotkeys", d.createHotkeysTab()),
		container.NewTabItem("Appearance", d.createAppearanceTab()),
		container.NewTabItem("Transcription", d.createTranscriptionTab()),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	// Create buttons
	saveButton := widget.NewButton("Save", func() {
		if d.onSave != nil {
			d.onSave(d.prefs)
		}
		d.window.Close()
		preferencesWindow = nil
	})

	cancelButton := widget.NewButton("Cancel", func() {
		d.window.Close()
		preferencesWindow = nil
	})

	// Create button container
	buttons := container.NewHBox(
		layout.NewSpacer(),
		cancelButton,
		saveButton,
	)

	// Create main content
	content := container.NewBorder(
		nil,                          // Top
		container.NewPadded(buttons), // Bottom
		nil,                          // Left
		nil,                          // Right
		tabs,                         // Center
	)

	// Set content to window
	d.window.SetContent(content)
}

// createGeneralTab creates the general settings tab
func (d *PreferencesDialog) createGeneralTab() fyne.CanvasObject {
	// Auto-copy checkbox
	autoCopyCheck := widget.NewCheck("Automatically copy transcriptions to clipboard", func(checked bool) {
		d.prefs.AutoCopy = checked
	})
	autoCopyCheck.Checked = d.prefs.AutoCopy

	// Save transcripts checkbox
	saveTranscriptsCheck := widget.NewCheck("Save transcriptions to file", func(checked bool) {
		d.prefs.SaveTranscripts = checked
	})
	saveTranscriptsCheck.Checked = d.prefs.SaveTranscripts

	// Transcript path entry
	transcriptPathEntry := widget.NewEntry()
	transcriptPathEntry.SetText(d.prefs.TranscriptPath)
	transcriptPathEntry.OnChanged = func(text string) {
		d.prefs.TranscriptPath = text
	}

	// Choose folder button
	chooseFolderButton := widget.NewButton("Choose Folder", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil {
				log.Println("Error selecting folder:", err)
				return
			}
			if uri == nil {
				return
			}
			path := uri.Path()
			transcriptPathEntry.SetText(path)
			d.prefs.TranscriptPath = path
		}, d.window)
	})

	// Start minimized checkbox
	startMinimizedCheck := widget.NewCheck("Start application minimized", func(checked bool) {
		d.prefs.StartMinimized = checked
	})
	startMinimizedCheck.Checked = d.prefs.StartMinimized

	// Test mode checkbox
	testModeCheck := widget.NewCheck("Test mode (simulated audio)", func(checked bool) {
		d.prefs.TestMode = checked
	})
	testModeCheck.Checked = d.prefs.TestMode

	// Create the layout
	return container.NewVBox(
		widget.NewLabelWithStyle("General Settings", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewPadded(autoCopyCheck),
		container.NewPadded(saveTranscriptsCheck),
		container.NewGridWithColumns(2,
			widget.NewLabel("Transcript folder:"),
			container.NewBorder(nil, nil, nil, chooseFolderButton, transcriptPathEntry),
		),
		container.NewPadded(startMinimizedCheck),
		container.NewPadded(testModeCheck),
	)
}

// createAudioTab creates the audio settings tab
func (d *PreferencesDialog) createAudioTab() fyne.CanvasObject {
	// Sample rate selection
	sampleRateSelect := widget.NewSelect([]string{"8000", "16000", "22050", "44100", "48000"}, func(selected string) {
		var rate float64
		switch selected {
		case "8000":
			rate = 8000
		case "16000":
			rate = 16000
		case "22050":
			rate = 22050
		case "44100":
			rate = 44100
		case "48000":
			rate = 48000
		}
		d.prefs.SampleRate = rate
	})
	sampleRateSelect.SetSelected(intToString(int(d.prefs.SampleRate)))

	// Channels selection
	channelsSelect := widget.NewSelect([]string{"1 (Mono)", "2 (Stereo)"}, func(selected string) {
		if selected == "1 (Mono)" {
			d.prefs.Channels = 1
		} else {
			d.prefs.Channels = 2
		}
	})
	if d.prefs.Channels == 1 {
		channelsSelect.SetSelected("1 (Mono)")
	} else {
		channelsSelect.SetSelected("2 (Stereo)")
	}

	// Buffer size selection
	bufferSizeSelect := widget.NewSelect([]string{"512", "1024", "2048", "4096"}, func(selected string) {
		var size int
		switch selected {
		case "512":
			size = 512
		case "1024":
			size = 1024
		case "2048":
			size = 2048
		case "4096":
			size = 4096
		}
		d.prefs.FramesPerBuffer = size
	})
	bufferSizeSelect.SetSelected(intToString(d.prefs.FramesPerBuffer))

	// Create the layout
	return container.NewVBox(
		widget.NewLabelWithStyle("Audio Settings", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWithColumns(2,
			widget.NewLabel("Sample Rate (Hz):"),
			sampleRateSelect,
		),
		container.NewGridWithColumns(2,
			widget.NewLabel("Channels:"),
			channelsSelect,
		),
		container.NewGridWithColumns(2,
			widget.NewLabel("Buffer Size (frames):"),
			bufferSizeSelect,
		),
	)
}

// createHotkeysTab creates the hotkeys settings tab
func (d *PreferencesDialog) createHotkeysTab() fyne.CanvasObject {
	// Modifiers selection
	ctrlCheck := widget.NewCheck("Ctrl", func(checked bool) {
		updateModifiers(checked, "ctrl", &d.prefs.HotkeyModifiers)
	})
	shiftCheck := widget.NewCheck("Shift", func(checked bool) {
		updateModifiers(checked, "shift", &d.prefs.HotkeyModifiers)
	})
	altCheck := widget.NewCheck("Alt", func(checked bool) {
		updateModifiers(checked, "alt", &d.prefs.HotkeyModifiers)
	})

	// Set initial state
	for _, mod := range d.prefs.HotkeyModifiers {
		switch mod {
		case "ctrl":
			ctrlCheck.Checked = true
		case "shift":
			shiftCheck.Checked = true
		case "alt":
			altCheck.Checked = true
		}
	}

	// Key selection
	keyEntry := widget.NewEntry()
	keyEntry.SetText(d.prefs.HotkeyKey)
	keyEntry.OnChanged = func(text string) {
		if len(text) > 0 {
			d.prefs.HotkeyKey = string([]rune(text)[0])
			keyEntry.SetText(d.prefs.HotkeyKey)
		}
	}

	// Create modifier layout
	modifiersBox := container.NewHBox(
		ctrlCheck,
		shiftCheck,
		altCheck,
	)

	// Warning label
	warningLabel := widget.NewLabelWithStyle(
		"Note: Hotkey changes will take effect after restarting the application.",
		fyne.TextAlignCenter,
		fyne.TextStyle{Italic: true},
	)

	// Create the layout
	return container.NewVBox(
		widget.NewLabelWithStyle("Hotkey Settings", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWithColumns(2,
			widget.NewLabel("Modifiers:"),
			modifiersBox,
		),
		container.NewGridWithColumns(2,
			widget.NewLabel("Key:"),
			keyEntry,
		),
		container.NewPadded(warningLabel),
	)
}

// createAppearanceTab creates the appearance settings tab
func (d *PreferencesDialog) createAppearanceTab() fyne.CanvasObject {
	// Theme selection
	themeCheck := widget.NewCheck("Dark theme", func(checked bool) {
		d.prefs.DarkTheme = checked
	})
	themeCheck.Checked = d.prefs.DarkTheme

	// Minimize to tray checkbox
	minimizeToTrayCheck := widget.NewCheck("Minimize to system tray when closing", func(checked bool) {
		d.prefs.MinimizeToTray = checked
	})
	minimizeToTrayCheck.Checked = d.prefs.MinimizeToTray

	// Create the layout
	return container.NewVBox(
		widget.NewLabelWithStyle("Appearance Settings", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewPadded(themeCheck),
		container.NewPadded(minimizeToTrayCheck),
	)
}

// createTranscriptionTab creates the transcription settings tab
func (d *PreferencesDialog) createTranscriptionTab() fyne.CanvasObject {
	// Model size selection
	modelSizeSelect := widget.NewSelect([]string{"tiny", "base", "small", "medium", "large"}, func(selected string) {
		d.prefs.ModelSize = selected
	})

	// Set the current value
	if d.prefs.ModelSize != "" {
		modelSizeSelect.SetSelected(d.prefs.ModelSize)
	} else {
		modelSizeSelect.SetSelected("small") // Default to small
	}

	// Create the layout
	return container.NewVBox(
		widget.NewLabelWithStyle("Transcription Settings", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWithColumns(2,
			widget.NewLabel("Model Size:"),
			modelSizeSelect,
		),
		widget.NewLabel(""), // Spacer
		widget.NewLabel("Smaller models are faster but less accurate."),
		widget.NewLabel("Larger models are more accurate but use more resources."),
	)
}

// Helper functions

// intToString converts an int to string
func intToString(val int) string {
	switch val {
	case 8000:
		return "8000"
	case 16000:
		return "16000"
	case 22050:
		return "22050"
	case 44100:
		return "44100"
	case 48000:
		return "48000"
	case 512:
		return "512"
	case 1024:
		return "1024"
	case 2048:
		return "2048"
	case 4096:
		return "4096"
	default:
		return ""
	}
}

// updateModifiers updates the modifiers list based on checkbox state
func updateModifiers(checked bool, modifier string, modifiers *[]string) {
	if checked {
		// Add modifier if not already present
		found := false
		for _, mod := range *modifiers {
			if mod == modifier {
				found = true
				break
			}
		}
		if !found {
			*modifiers = append(*modifiers, modifier)
		}
	} else {
		// Remove modifier
		newModifiers := []string{}
		for _, mod := range *modifiers {
			if mod != modifier {
				newModifiers = append(newModifiers, mod)
			}
		}
		*modifiers = newModifiers
	}
}
