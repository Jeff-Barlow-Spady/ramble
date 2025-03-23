package config

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
)

// ThemeMode represents the UI theme mode
type ThemeMode string

const (
	// ThemeModeLight is light theme mode
	ThemeModeLight ThemeMode = "light"
	// ThemeModeDark is dark theme mode
	ThemeModeDark ThemeMode = "dark"
	// ThemeModeSystem uses the system theme preference
	ThemeModeSystem ThemeMode = "system"
)

// Config holds the application configuration
type Config struct {
	// HotKey configuration
	HotKeyCtrl  bool
	HotKeyShift bool
	HotKeyAlt   bool
	HotKeyKey   string

	// Audio configuration
	AudioSampleRate int
	AudioBufferSize int
	AudioChannels   int

	// Whisper configuration
	WhisperModelPath string
	WhisperModelType string

	// UI configuration
	ShowTranscriptionUI bool
	InsertTextAtCursor  bool
	MinimizeToTray      bool // Whether to start minimized to system tray
	TerminalMode        bool // Whether to use terminal UI mode
	SafeMode            bool // Whether to confirm before inserting text
	Theme               *ThemeConfig
	ThemeMode           ThemeMode // The theme mode (light/dark/system)

	// Test mode configuration
	TestMode               bool
	TestModeVisualFeedback bool
}

// ThemeConfig holds the theme configuration
type ThemeConfig struct {
	BackgroundColor color.RGBA
	TextColor       color.RGBA
}

// DefaultTheme returns the default theme configuration
func DefaultTheme() *ThemeConfig {
	return &ThemeConfig{
		BackgroundColor: color.RGBA{R: 0x1E, G: 0x1E, B: 0x2E, A: 0xFF}, // Dark blue-gray
		TextColor:       color.RGBA{R: 0x61, G: 0xE3, B: 0xFA, A: 0xFF}, // Bright cyan
	}
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	// Get the .ramble directory for models
	modelDir := "./models/" // Default fallback
	if dir, err := GetModelDir(); err == nil {
		modelDir = dir
	}

	return &Config{
		// Default hotkey: Ctrl+Shift+S
		HotKeyCtrl:  true,
		HotKeyShift: true,
		HotKeyAlt:   false,
		HotKeyKey:   "s",

		// Default audio settings
		AudioSampleRate: 16000, // 16kHz sample rate for Whisper
		AudioBufferSize: 1024,
		AudioChannels:   1, // Mono

		// Default Whisper settings
		WhisperModelPath: modelDir,
		WhisperModelType: "tiny", // Use tiny model by default

		// Default UI settings
		ShowTranscriptionUI: true,
		InsertTextAtCursor:  true,
		MinimizeToTray:      false, // Don't start minimized by default
		TerminalMode:        false,
		SafeMode:            false, // Don't require confirmation by default
		Theme:               DefaultTheme(),
		ThemeMode:           ThemeModeSystem, // Use system theme by default

		// Default test mode settings - should be false for production use
		// TestMode should only be enabled for the test binary or explicit testing
		TestMode:               false,
		TestModeVisualFeedback: true,
	}
}

// Current holds the active configuration
var Current = DefaultConfig()

// SetTestMode enables test mode with appropriate settings
// Deprecated: Directly set TestMode and related flags instead
func SetTestMode() {
	Current.TestMode = true
	Current.TestModeVisualFeedback = true
}

// ColorToFyneColor converts a Go color.RGBA to a Fyne color
func ColorToFyneColor(c color.RGBA) color.Color {
	return c
}

// GetAppDir returns the path to the .ramble directory
func GetAppDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	appDir := filepath.Join(homeDir, ".ramble")

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create .ramble directory: %w", err)
	}

	return appDir, nil
}

// GetConfigFilePath returns the path to the config file
func GetConfigFilePath() (string, error) {
	appDir, err := GetAppDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(appDir, "config.json"), nil
}

// GetAudioBackupDir returns the path to the audio backup directory
func GetAudioBackupDir() (string, error) {
	appDir, err := GetAppDir()
	if err != nil {
		return "", err
	}

	backupDir := filepath.Join(appDir, "audio_backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create audio backup directory: %w", err)
	}

	return backupDir, nil
}

// GetModelDir returns the path to the model directory
func GetModelDir() (string, error) {
	appDir, err := GetAppDir()
	if err != nil {
		return "", err
	}

	modelDir := filepath.Join(appDir, "models")
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create model directory: %w", err)
	}

	return modelDir, nil
}

// LoadConfig loads the configuration from the config file
func LoadConfig() error {
	configPath, err := GetConfigFilePath()
	if err != nil {
		return fmt.Errorf("failed to get config file path: %w", err)
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Config file doesn't exist, use defaults
		Current = DefaultConfig()
		// Save the default config
		return SaveConfig()
	}

	// Read the config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse JSON data
	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Update current config
	Current = &config

	// Ensure theme is set
	if Current.Theme == nil {
		Current.Theme = DefaultTheme()
	}

	return nil
}

// SaveConfig saves the configuration to the config file
func SaveConfig() error {
	configPath, err := GetConfigFilePath()
	if err != nil {
		return fmt.Errorf("failed to get config file path: %w", err)
	}

	// Create config directory if it doesn't exist
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal config to JSON
	data, err := json.MarshalIndent(Current, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	err = os.WriteFile(configPath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
