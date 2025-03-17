package ui

import (
	"log"

	"fyne.io/systray"
	"github.com/jeff-barlow-spady/ramble/pkg/resources"
)

// SystemTray represents a system tray icon
type SystemTray struct {
	isTestMode   bool
	isRunning    bool
	onAboutClick func()
	onQuitClick  func()
	onStartStop  func()
	onPrefsClick func()
	isRecording  bool

	// Store menu items as fields to ensure they aren't garbage collected
	mStartStop   *systray.MenuItem
	mPreferences *systray.MenuItem
	mAbout       *systray.MenuItem
	mQuit        *systray.MenuItem
}

// NewSystemTray creates a new system tray icon
func NewSystemTray(testMode bool) *SystemTray {
	return &SystemTray{
		isTestMode:  testMode,
		isRunning:   false,
		isRecording: false,
		onAboutClick: func() {
			log.Println("About clicked (default handler)")
		},
		onQuitClick: func() {
			log.Println("Quit clicked (default handler)")
		},
		onStartStop: func() {
			log.Println("Start/Stop clicked (default handler)")
		},
		onPrefsClick: func() {
			log.Println("Preferences clicked (default handler)")
		},
	}
}

// SetCallbacks sets callbacks for systray menu items
func (s *SystemTray) SetCallbacks(onStartStop, onPrefs, onAbout, onQuit func()) {
	if onStartStop != nil {
		s.onStartStop = onStartStop
	}
	if onPrefs != nil {
		s.onPrefsClick = onPrefs
	}
	if onAbout != nil {
		s.onAboutClick = onAbout
	}
	if onQuit != nil {
		s.onQuitClick = onQuit
	}
}

// Start initializes the system tray icon
func (s *SystemTray) Start() {
	if s.isRunning {
		return
	}

	go systray.Run(s.onReady, s.onExit)
	s.isRunning = true
}

// Stop removes the system tray icon
func (s *SystemTray) Stop() {
	if !s.isRunning {
		return
	}

	systray.Quit()
	s.isRunning = false
}

// UpdateRecordingState updates the recording state in the systray menu
func (s *SystemTray) UpdateRecordingState(isRecording bool) {
	s.isRecording = isRecording

	if s.mStartStop == nil {
		return
	}

	if isRecording {
		s.mStartStop.SetTitle("Stop Recording")
	} else {
		s.mStartStop.SetTitle("Start Recording")
	}
}

// onReady is called when the systray icon is ready
func (s *SystemTray) onReady() {
	// Set icon
	iconBytes, err := resources.GetIconData()
	if err != nil {
		log.Println("Failed to load systray icon:", err)
		systray.SetIcon([]byte{})
	} else {
		systray.SetIcon(iconBytes)
	}

	systray.SetTitle("Ramble")
	systray.SetTooltip("Ramble Speech-to-Text")

	// Create menu items
	s.mStartStop = systray.AddMenuItem("Start Recording", "Start/Stop speech recording")
	systray.AddSeparator()
	s.mPreferences = systray.AddMenuItem("Preferences", "Configure Ramble")
	s.mAbout = systray.AddMenuItem("About", "About Ramble")
	s.mQuit = systray.AddMenuItem("Quit", "Quit Ramble")

	// Handle menu item clicks
	go func() {
		for {
			select {
			case <-s.mStartStop.ClickedCh:
				s.onStartStop()
			case <-s.mPreferences.ClickedCh:
				s.onPrefsClick()
			case <-s.mAbout.ClickedCh:
				s.onAboutClick()
			case <-s.mQuit.ClickedCh:
				s.onQuitClick()
				return
			}
		}
	}()
}

// onExit is called when the systray is exiting
func (s *SystemTray) onExit() {
	// Clean up resources if needed
	s.isRunning = false
}
