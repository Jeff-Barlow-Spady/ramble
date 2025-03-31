// Package hotkey provides functionality for detecting global hotkeys
package hotkey

import (
	"fmt"
	"strings"
	"sync"

	hook "github.com/robotn/gohook"
)

// Configuration for the hotkey combinations
type Config struct {
	// Modifier keys (ctrl, shift, alt)
	Modifiers []string
	// Main key (e.g., 's' for Ctrl+Shift+S)
	Key string
}

// DefaultConfig returns a default hotkey configuration (Ctrl+Shift+S)
func DefaultConfig() Config {
	return Config{
		Modifiers: []string{"ctrl", "shift"},
		Key:       "s",
	}
}

// Detector handles hotkey registration and event detection
type Detector struct {
	config Config
	active bool
	mu     sync.Mutex
	stopCh chan struct{}
}

// NewDetector creates a new hotkey detector with the given configuration
func NewDetector(config Config) *Detector {
	return &Detector{
		config: config,
		active: false,
		stopCh: make(chan struct{}),
	}
}

// GetConfig returns the current configuration
func (d *Detector) GetConfig() Config {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.config
}

// Start begins listening for the configured hotkey
// The provided callback will be executed when the hotkey is detected
func (d *Detector) Start(callback func()) error {
	d.mu.Lock()
	if d.active {
		d.mu.Unlock()
		return fmt.Errorf("hotkey detector already running")
	}
	d.active = true
	d.stopCh = make(chan struct{})
	d.mu.Unlock()

	// Start hook events in a separate goroutine
	go func() {
		// Hook for global events
		evChan := hook.Start()
		defer hook.End()

		for {
			select {
			case <-d.stopCh:
				return
			case ev := <-evChan:
				// Only respond to key down events
				if ev.Kind == hook.KeyDown {
					if isHotkeyPressed(ev, d.config) {
						callback()
					}
				}
			}
		}
	}()

	return nil
}

// Stop terminates the hotkey listener
func (d *Detector) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.active {
		return
	}

	d.active = false
	close(d.stopCh)
}

// Helper function to check if the current event matches our hotkey configuration
func isHotkeyPressed(ev hook.Event, config Config) bool {
	// Only process keyboard events with actual characters
	if ev.Keychar == 0 {
		return false
	}

	// Added more strict modifier checking
	// Get the current keychar as a string
	keyChar := string(ev.Keychar)

	// Convert the modifiers and key to lowercase for case-insensitive comparison
	keyLower := strings.ToLower(config.Key)
	keyCharLower := strings.ToLower(keyChar)

	// First check if the key exactly matches our target key
	if keyCharLower != keyLower {
		return false
	}

	// Create a more strict check for modifiers
	// Check the full state for each modifier
	ctrlPressed := (ev.Rawcode & 1) != 0
	shiftPressed := (ev.Rawcode & 2) != 0
	altPressed := (ev.Rawcode & 4) != 0

	// Verify that exactly the right modifiers are pressed
	expectedCtrl := containsIgnoreCase(config.Modifiers, "ctrl")
	expectedShift := containsIgnoreCase(config.Modifiers, "shift")
	expectedAlt := containsIgnoreCase(config.Modifiers, "alt")

	// Check that modifiers match exactly - not having a modifier pressed when
	// it's not in the config counts as a match
	if ctrlPressed != expectedCtrl {
		return false
	}
	if shiftPressed != expectedShift {
		return false
	}
	if altPressed != expectedAlt {
		return false
	}

	// If we get here, the key and modifiers match exactly
	return true
}

// Helper function to check if a string array contains a string (case insensitive)
func containsIgnoreCase(arr []string, str string) bool {
	for _, s := range arr {
		if strings.EqualFold(s, str) {
			return true
		}
	}
	return false
}
