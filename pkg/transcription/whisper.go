// Package transcription provides speech-to-text functionality
package transcription

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/jeff-barlow-spady/ramble/pkg/logger"
	"github.com/jeff-barlow-spady/ramble/pkg/transcription/embed"
)

// Model size options for Whisper
type ModelSize string

const (
	ModelTiny   ModelSize = "tiny"
	ModelBase   ModelSize = "base"
	ModelSmall  ModelSize = "small"
	ModelMedium ModelSize = "medium"
	ModelLarge  ModelSize = "large"
)

// Config holds configuration for the transcriber
type Config struct {
	// Model size to use
	ModelSize ModelSize
	// Path to model files (if empty, uses default location)
	ModelPath string
	// Language code (if empty, auto-detects)
	Language string
	// Whether to enable debug logs
	Debug bool
	// Path to the executable (if empty, auto-detected)
	ExecutablePath string
	// Whether to prefer system-installed whisper executables over auto-installation
	PreferSystemExecutable bool
	// ExecutableFinder to use for finding or installing the executable
	Finder ExecutableFinder
}

// DefaultConfig returns the default configuration for transcription
func DefaultConfig() Config {
	config := Config{
		ModelSize:              ModelTiny, // Use tiny model for streaming - fastest response
		ModelPath:              "",        // Auto-detect or download
		Language:               "en",      // English
		Debug:                  false,     // Disable debug logs by default
		ExecutablePath:         "",        // Auto-detect
		PreferSystemExecutable: true,      // Prefer system executable by default
	}

	// Create a finder with this config
	finder := &DefaultExecutableFinder{config: config}
	config.Finder = finder

	return config
}

// Transcriber interface defines methods for speech-to-text conversion
type Transcriber interface {
	// ProcessAudioChunk processes a chunk of audio data and returns transcribed text
	ProcessAudioChunk(audioData []float32) (string, error)
	// Close frees resources
	Close() error
}

// ConfigurableTranscriber is an enhanced transcriber that supports updating its configuration
type ConfigurableTranscriber interface {
	Transcriber
	// UpdateConfig updates the transcriber's configuration
	UpdateConfig(config Config) error
	// GetModelInfo returns information about the current model
	GetModelInfo() (ModelSize, string)
	// SetStreamingCallback sets a callback for receiving streaming results
	SetStreamingCallback(callback func(text string))
}

// ExecutableFinder defines an interface for finding whisper executables
type ExecutableFinder interface {
	FindExecutable() string
	InstallExecutable() (string, error)
}

// DefaultExecutableFinder is the default implementation for finding executables
type DefaultExecutableFinder struct {
	config Config
}

// New creates a new DefaultExecutableFinder with the given config
func (f *DefaultExecutableFinder) New(config Config) *DefaultExecutableFinder {
	return &DefaultExecutableFinder{
		config: config,
	}
}

// FindExecutable looks for a whisper executable on the system
func (f *DefaultExecutableFinder) FindExecutable() string {
	// Try to find the executable with error handling
	path, err := f.findWhisperExecutable()
	if err != nil {
		// Log the error for debugging
		if f.config.Debug {
			logger.Debug(logger.CategoryTranscription, "Error finding executable: %v", err)
		}
		return ""
	}
	return path
}

// findWhisperExecutable is the internal implementation that can return errors
func (f *DefaultExecutableFinder) findWhisperExecutable() (string, error) {
	// First, check the explicit path if provided
	if f.config.ExecutablePath != "" {
		if stat, err := os.Stat(f.config.ExecutablePath); err == nil && !stat.IsDir() {
			if err := checkExecutablePermissions(f.config.ExecutablePath); err == nil {
				return f.config.ExecutablePath, nil
			}
		}
		// If explicit path is provided but invalid, return an error
		return "", fmt.Errorf("%w: provided executable path does not exist or is not executable: %s",
			ErrInvalidExecutablePath, f.config.ExecutablePath)
	}

	// Check the PATH first (system installed executables)
	executable, err := f.findInPath()
	if err == nil {
		logger.Info(logger.CategoryTranscription, "Found whisper executable in PATH: %s", executable)
		return executable, nil
	}

	// Check common installation locations
	executable, err = f.findCommonInstallLocations()
	if err == nil {
		logger.Info(logger.CategoryTranscription, "Found whisper executable in common location: %s", executable)
		return executable, nil
	}

	// Check package manager locations (Snap, Flatpak, etc.)
	executable, err = f.findPackageManagerLocations()
	if err == nil {
		logger.Info(logger.CategoryTranscription, "Found whisper executable from package manager: %s", executable)
		return executable, nil
	}

	// Check user-specific installations
	executable, err = f.findUserDirectoryInstallations()
	if err == nil {
		logger.Info(logger.CategoryTranscription, "Found whisper executable in user directory: %s", executable)
		return executable, nil
	}

	// No executable found
	return "", ErrExecutableNotFound
}

// Add a helper function to check executable permissions
func checkExecutablePermissions(path string) error {
	// Skip permission check on Windows - different permission model
	if runtime.GOOS == "windows" {
		// Just check if file exists on Windows
		_, err := os.Stat(path)
		return err
	}

	// Check if file is executable on Unix-like systems
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	// Check if the file is executable by the current user
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("file exists but is not executable")
	}

	return nil
}

// findInPath checks if the whisper executable is available in the system PATH
func (f *DefaultExecutableFinder) findInPath() (string, error) {
	// Common executable names
	execNames := []string{"whisper", "whisper.cpp", "whisper-cpp", "whisper-gael", "whisper.exe", "whisper-cpp.exe"}

	for _, name := range execNames {
		path, err := exec.LookPath(name)
		if err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("whisper executable not found in PATH")
}

// findCommonInstallLocations checks standard system paths for the whisper executable
func (f *DefaultExecutableFinder) findCommonInstallLocations() (string, error) {
	var paths []string

	// System-wide binary locations
	if runtime.GOOS == "windows" {
		paths = []string{
			`C:\Program Files\Whisper`,
			`C:\Program Files (x86)\Whisper`,
			`C:\Whisper`,
		}
	} else {
		// Unix-like systems (Linux, macOS)
		paths = []string{
			"/usr/bin",
			"/usr/local/bin",
			"/opt/whisper",
			"/opt/whisper.cpp",
			"/opt/local/bin", // MacPorts
		}
	}

	return f.findExecutableInPaths(paths)
}

// findPackageManagerLocations checks package manager locations for the whisper executable
func (f *DefaultExecutableFinder) findPackageManagerLocations() (string, error) {
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("package manager locations only supported on Linux")
	}

	// Package manager locations on Linux
	paths := []string{
		// Snap
		"/snap/bin",
		"/var/lib/snapd/snap/bin",

		// Flatpak
		"/var/lib/flatpak/exports/bin",

		// PPA and other system package installations
		"/usr/lib/whisper",
		"/usr/lib/whisper.cpp",
		"/usr/lib64/whisper",
		"/usr/share/whisper/bin",

		// AppImage potential locations
		"/tmp/.mount_whisper",
		"/tmp/appimage",

		// Homebrew on Linux
		"/home/linuxbrew/.linuxbrew/bin",
	}

	// Add user's .local Flatpak location
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".local/share/flatpak/exports/bin"))
	}

	return f.findExecutableInPaths(paths)
}

// findUserDirectoryInstallations checks user-specific installation directories
func (f *DefaultExecutableFinder) findUserDirectoryInstallations() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("couldn't determine user home directory: %w", err)
	}

	var paths []string

	if runtime.GOOS == "windows" {
		paths = []string{
			filepath.Join(home, "AppData", "Local", "Whisper"),
			filepath.Join(home, "AppData", "Local", "Programs", "Whisper"),
			filepath.Join(home, "Whisper"),
		}
	} else {
		// Unix-like systems (Linux, macOS)
		paths = []string{
			filepath.Join(home, ".local/bin"),
			filepath.Join(home, ".whisper"),
			filepath.Join(home, ".whisper.cpp"),
			filepath.Join(home, "bin"),
			filepath.Join(home, "Applications"), // macOS

			// Common user-installed location from GitHub repos
			filepath.Join(home, "git/whisper.cpp"),
			filepath.Join(home, "git/whisper"),
			filepath.Join(home, "github/whisper.cpp"),
			filepath.Join(home, "github/whisper"),

			// Ramble app locations
			filepath.Join(home, ".ramble/bin"),
			filepath.Join(home, ".config/ramble/bin"),
		}
	}

	// Add ~/.cargo/bin for Rust-based whisper implementations
	paths = append(paths, filepath.Join(home, ".cargo/bin"))

	// Add ~/go/bin for Go-based whisper implementations
	paths = append(paths, filepath.Join(home, "go/bin"))

	return f.findExecutableInPaths(paths)
}

// findExecutableInPaths checks multiple directories for whisper executables
func (f *DefaultExecutableFinder) findExecutableInPaths(paths []string) (string, error) {
	execNames := []string{"whisper", "whisper.cpp", "whisper-cpp", "whisper-gael"}

	// Add platform-specific extensions
	if runtime.GOOS == "windows" {
		execNames = append(execNames, "whisper.exe", "whisper-cpp.exe", "whisper-gael.exe")
	}

	for _, path := range paths {
		// Skip if path doesn't exist
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}

		for _, name := range execNames {
			execPath := filepath.Join(path, name)
			if _, err := os.Stat(execPath); err == nil {
				if err := checkExecutablePermissions(execPath); err == nil {
					return execPath, nil
				}
			}
		}
	}

	return "", fmt.Errorf("whisper executable not found in common paths")
}

// InstallExecutable tries to install the whisper executable
func (f *DefaultExecutableFinder) InstallExecutable() (string, error) {
	logger.Info(logger.CategoryTranscription, "Attempting to download and install whisper.cpp executable")

	// Create installation directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	installDir := filepath.Join(homeDir, ".local", "share", "ramble", "bin")
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create installation directory: %w", ErrExecutableInstallFailed)
	}

	// Target executable path
	execPath := filepath.Join(installDir, "whisper-cpp")

	// Potential download URLs in order of preference
	// We try specific release URLs first, then fall back to latest
	downloadURLs := []string{
		// Specific version URLs
		"https://github.com/ggerganov/whisper.cpp/releases/download/v1.5.0/whisper-linux-x64",
		"https://github.com/ggerganov/whisper.cpp/releases/download/v1.4.2/whisper-linux-x64",
		// Latest release fallback
		"https://github.com/ggerganov/whisper.cpp/releases/latest/download/whisper-linux-x64",
	}

	// Try each URL until one works
	var dlError error
	for _, downloadURL := range downloadURLs {
		logger.Info(logger.CategoryTranscription, "Attempting to download whisper.cpp from %s", downloadURL)

		err := downloadExecutable(downloadURL, execPath)
		if err == nil {
			// Successful download
			logger.Info(logger.CategoryTranscription, "Successfully installed whisper.cpp to %s", execPath)
			return execPath, nil
		}

		dlError = err
		logger.Warning(logger.CategoryTranscription, "Failed to download from %s: %v", downloadURL, err)
	}

	// If we get here, all downloads failed
	return "", fmt.Errorf("%w: %v", ErrExecutableInstallFailed, dlError)
}

// NewTranscriber creates a new transcription service
func NewTranscriber(config Config) (Transcriber, error) {
	// Ensure we have a finder
	if config.Finder == nil {
		config.Finder = &DefaultExecutableFinder{config: config}
	}

	var execPath string
	var err error

	// Try to use embedded executable first
	execPath, err = embed.GetWhisperExecutable()
	if err == nil {
		logger.Info(logger.CategoryTranscription, "Using embedded whisper executable")
		config.ExecutablePath = execPath
	} else if !errors.Is(err, embed.ErrAssetsNotEmbedded) {
		// If it's not just missing assets, log a warning
		logger.Warning(logger.CategoryTranscription,
			"Failed to extract embedded executable: %v, falling back to system version", err)
	}

	// If we didn't get an embedded executable, use system version
	if execPath == "" {
		execPath, err = ensureExecutablePath(config)
		if err != nil {
			logger.Warning(logger.CategoryTranscription,
				"No whisper executable found: %v", err)
			return &placeholderTranscriber{config: config}, nil
		}
		config.ExecutablePath = execPath
	}

	// Try to use embedded model
	modelPath, err := embed.ExtractModel(string(config.ModelSize))
	if err == nil {
		logger.Info(logger.CategoryTranscription, "Using embedded model: %s", config.ModelSize)
		config.ModelPath = modelPath
	} else {
		// Fall back to downloaded model
		logger.Warning(logger.CategoryTranscription,
			"Embedded model not found: %v, will try to download", err)
		modelPath, err = ensureModel(config)
		if err != nil {
			logger.Warning(logger.CategoryTranscription,
				"Failed to download model: %v", err)
			return &placeholderTranscriber{config: config}, nil
		}
		config.ModelPath = modelPath
	}

	return NewExecutableTranscriber(config)
}

// ensureModel checks if model exists and downloads it if needed
func ensureModel(config Config) (string, error) {
	modelPath := getModelPath(config.ModelPath, config.ModelSize)

	// Download the model if needed
	modelFile, err := DownloadModel(modelPath, config.ModelSize)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrModelDownloadFailed, err)
	}

	return modelFile, nil
}

// isWhisperInstallSupported returns true if this platform supports auto-install
func isWhisperInstallSupported() bool {
	// Currently we only support auto-install on limited platforms
	// This could be expanded in the future
	return runtime.GOOS == "linux" && runtime.GOARCH == "amd64"
}

// downloadExecutable downloads an executable from a URL and saves it to the specified path
func downloadExecutable(url string, destPath string) error {
	// Download the executable
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: HTTP status error: %s", ErrExecutableInstallFailed, resp.Status)
	}

	// Create output file
	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY, 0755) // Make it executable
	if err != nil {
		return fmt.Errorf("%w: failed to create output file: %v", ErrExecutableInstallFailed, err)
	}
	defer out.Close()

	// Copy the data
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		// Clean up on error
		os.Remove(destPath)
		return fmt.Errorf("%w: failed to save executable: %v", ErrExecutableInstallFailed, err)
	}

	return nil
}

// placeholderTranscriber is a simple implementation that doesn't do real transcription
type placeholderTranscriber struct {
	config Config
	mu     sync.Mutex
}

// ProcessAudioChunk returns a placeholder message
func (t *placeholderTranscriber) ProcessAudioChunk(audioData []float32) (string, error) {
	// Only return a message occasionally to avoid flooding the UI
	if len(audioData) > 16000*3 { // Only for chunks > 3 seconds
		return "Speech transcription placeholder (Whisper not available)", nil
	}
	return "", nil
}

// Close does nothing for the placeholder
func (t *placeholderTranscriber) Close() error {
	return nil
}

// UpdateConfig updates the configuration
func (t *placeholderTranscriber) UpdateConfig(config Config) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.config = config
	return nil
}

// GetModelInfo returns information about the current model
func (t *placeholderTranscriber) GetModelInfo() (ModelSize, string) {
	return t.config.ModelSize, "placeholder"
}

// SetStreamingCallback sets a callback for receiving streaming results
func (t *placeholderTranscriber) SetStreamingCallback(callback func(text string)) {
	// Placeholder implementation, no streaming callback
}

// getExecutableTypeName returns a string representation of the executable type
func getExecutableTypeName(execType ExecutableType) string {
	switch execType {
	case ExecutableTypeWhisperCpp:
		return "whisper.cpp"
	case ExecutableTypeWhisperGael:
		return "whisper-gael (Python)"
	default:
		return "unknown"
	}
}

func testTranscriber(t Transcriber) error {
	// Simple smoke test to verify transcriber can execute
	// Create a minimal audio sample to test
	testSample := make([]float32, 100)

	// Try to process it with a short timeout
	done := make(chan error, 1)
	go func() {
		_, err := t.ProcessAudioChunk(testSample)
		done <- err
	}()

	// Wait for completion or timeout
	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Second):
		return fmt.Errorf("transcriber test timed out")
	}
}

// ensureExecutablePath finds or installs a whisper executable
func ensureExecutablePath(config Config) (string, error) {
	// If a specific path is provided, use it
	if config.ExecutablePath != "" {
		if _, err := os.Stat(config.ExecutablePath); err == nil {
			return config.ExecutablePath, nil
		}
		return "", fmt.Errorf("%w: %s", ErrInvalidExecutablePath, config.ExecutablePath)
	}

	// Try to find whisper executable on the system
	execPath := config.Finder.FindExecutable()
	if execPath != "" {
		return execPath, nil
	}

	// If auto-install is supported, try it
	if isWhisperInstallSupported() {
		execPath, err := config.Finder.InstallExecutable()
		if err == nil {
			return execPath, nil
		}
		return "", fmt.Errorf("%w: %v", ErrExecutableInstallFailed, err)
	}

	return "", fmt.Errorf("%w: %s", ErrPlatformNotSupported,
		"speech recognition tools not found; please install whisper.cpp or specify the path manually")
}
