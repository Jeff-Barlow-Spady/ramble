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

	// Convert the key from the event to string for comparison
	keyChar := string(ev.Keychar)

	// Check if the main key matches (case insensitive)
	if !strings.EqualFold(keyChar, config.Key) {
		return false
	}

	// Check if all required modifiers are pressed
	// For gohook, we need to check the raw event flags
	ctrlPressed := (ev.Rawcode & 1) != 0
	shiftPressed := (ev.Rawcode & 2) != 0
	altPressed := (ev.Rawcode & 4) != 0

	// Create a map of which modifiers are pressed
	modifierState := map[string]bool{
		"ctrl":  ctrlPressed,
		"shift": shiftPressed,
		"alt":   altPressed,
	}

	// Verify all required modifiers are pressed
	for _, mod := range config.Modifiers {
		mod = strings.ToLower(mod)
		if !modifierState[mod] {
			return false
		}
	}

	// Ensure no extra modifiers are pressed that aren't in our config
	// This prevents triggering when extra modifiers are held
	extraModifiers := true
	if ctrlPressed && !containsIgnoreCase(config.Modifiers, "ctrl") {
		extraModifiers = false
	}
	if shiftPressed && !containsIgnoreCase(config.Modifiers, "shift") {
		extraModifiers = false
	}
	if altPressed && !containsIgnoreCase(config.Modifiers, "alt") {
		extraModifiers = false
	}

	return extraModifiers
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
