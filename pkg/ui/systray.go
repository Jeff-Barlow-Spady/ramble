package ui

import (
	"log"
	"sync"

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
	mShowWindow  *systray.MenuItem

	// Callbacks
	onShowWindow func()

	mu sync.Mutex
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
func (s *SystemTray) SetCallbacks(
	onStartStop func(),
	onPrefsClick func(),
	onAboutClick func(),
	onQuitClick func(),
	onShowWindow func(),
) {
	s.onStartStop = onStartStop
	s.onPrefsClick = onPrefsClick
	s.onAboutClick = onAboutClick
	s.onQuitClick = onQuitClick
	s.onShowWindow = onShowWindow
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
	// Update internal state first
	s.isRecording = isRecording

	// Early return if systray menu isn't initialized yet
	if s.mStartStop == nil {
		return
	}

	// Update menu text safely
	if isRecording {
		s.mStartStop.SetTitle("Stop Recording")
	} else {
		s.mStartStop.SetTitle("Start Recording")
	}

	// Update icon based on recording state
	var err error
	var iconBytes []byte

	if isRecording {
		// Set red icon to indicate recording
		iconBytes, err = resources.GetRedIconData()
	} else {
		// Return to normal icon
		iconBytes, err = resources.GetIconData()
	}

	// Only update icon if we successfully loaded it
	if err == nil && len(iconBytes) > 0 {
		systray.SetIcon(iconBytes)
	} else if err != nil {
		log.Printf("Failed to update systray icon: %v", err)
	}
}

// onReady is called when the systray icon is ready
func (s *SystemTray) onReady() {
	// Set icon
	iconBytes, err := resources.GetIconData()
	if err != nil {
		log.Println("Failed to load systray icon:", err)
		// Use empty icon as fallback, but continue initialization
		iconBytes = []byte{}
	}

	// Set the icon (empty is okay, systray will handle it)
	systray.SetIcon(iconBytes)

	// Set application name and tooltip
	systray.SetTitle("Ramble")
	systray.SetTooltip("Ramble Speech-to-Text")

	// Create menu items
	s.mStartStop = systray.AddMenuItem("Start Recording", "Start/Stop speech recording")
	s.mShowWindow = systray.AddMenuItem("Show Window", "Show the main application window")
	systray.AddSeparator()
	s.mPreferences = systray.AddMenuItem("Preferences", "Configure Ramble")
	s.mAbout = systray.AddMenuItem("About", "About Ramble")
	s.mQuit = systray.AddMenuItem("Quit", "Quit Ramble")

	// Handle menu item clicks in a goroutine to keep the UI responsive
	go func() {
		for {
			select {
			case <-s.mStartStop.ClickedCh:
				if s.onStartStop != nil {
					s.onStartStop()
				}
			case <-s.mShowWindow.ClickedCh:
				if s.onShowWindow != nil {
					s.onShowWindow()
				}
			case <-s.mPreferences.ClickedCh:
				if s.onPrefsClick != nil {
					s.onPrefsClick()
				}
			case <-s.mAbout.ClickedCh:
				if s.onAboutClick != nil {
					s.onAboutClick()
				}
			case <-s.mQuit.ClickedCh:
				if s.onQuitClick != nil {
					s.onQuitClick()
				}
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
