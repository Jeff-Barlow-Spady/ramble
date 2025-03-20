package embed

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// ErrAssetsNotEmbedded indicates embedded assets are not available
var ErrAssetsNotEmbedded = errors.New("embedded assets not available")

// Use a simpler embed directive that just requires directories to exist
//
//go:embed binaries/linux-amd64/whisper models/tiny.bin models/small.bin
var Assets embed.FS

// HasEmbeddedAssets checks if embedded assets are available and usable
func HasEmbeddedAssets() bool {
	// Check if we have platform-specific binaries
	platform := runtime.GOOS + "-" + runtime.GOARCH
	binaryPath := filepath.Join("binaries", platform, "whisper")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}

	// Try to read the whisper binary file
	_, err := Assets.ReadFile(binaryPath)
	if err != nil {
		return false
	}

	// Try to read the model file (small.bin)
	_, err = Assets.ReadFile(filepath.Join("models", "small.bin"))
	if err != nil {
		return false
	}

	return true
}

// ExtractModel extracts the embedded model file for the specified model size
// to a persistent location on disk. It returns the path to the extracted model
// or an error if extraction fails.
func ExtractModel(modelSize string) (string, error) {
	if !HasEmbeddedAssets() {
		return "", ErrAssetsNotEmbedded
	}

	modelPath := filepath.Join("models", modelSize+".bin")
	modelData, err := Assets.ReadFile(modelPath)
	if err != nil {
		return "", err
	}

	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Create persistent directory - use .local/share/ramble/models to match what the executable expects
	persistentDir := filepath.Join(homeDir, ".local", "share", "ramble", "models")
	if err := os.MkdirAll(persistentDir, 0755); err != nil {
		return "", err
	}

	// Use the "ggml-" prefix and .en suffix to match what the whisper executable expects
	// This matches the downloaded model format from WhisperModelFilenames in downloader.go
	outPath := filepath.Join(persistentDir, "ggml-"+modelSize+".en.bin")

	// Debug: Print the full path where the model is being saved
	fmt.Printf("DEBUG: Saving model to: %s\n", outPath)

	// Check if the file already exists
	if _, err := os.Stat(outPath); err == nil {
		// File already exists, no need to extract again
		fmt.Printf("DEBUG: Model already exists at: %s\n", outPath)
		return outPath, nil
	}

	// Write the model file
	if err := os.WriteFile(outPath, modelData, 0644); err != nil {
		return "", err
	}

	fmt.Printf("DEBUG: Successfully saved model to: %s\n", outPath)
	return outPath, nil
}

// GetWhisperExecutable extracts the platform-specific whisper executable
// to a persistent location on disk. It returns the path to the extracted
// executable or an error if extraction fails.
func GetWhisperExecutable() (string, error) {
	if !HasEmbeddedAssets() {
		return "", ErrAssetsNotEmbedded
	}

	// Determine platform-specific binary path
	platform := runtime.GOOS + "-" + runtime.GOARCH
	binaryPath := filepath.Join("binaries", platform, "whisper")

	// Add .exe extension on Windows
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}

	// Extract binary
	binaryData, err := Assets.ReadFile(binaryPath)
	if err != nil {
		return "", err
	}

	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Create persistent directory
	persistentDir := filepath.Join(homeDir, ".ramble", "bin")
	if err := os.MkdirAll(persistentDir, 0755); err != nil {
		return "", err
	}

	outPath := filepath.Join(persistentDir, "whisper")
	if runtime.GOOS == "windows" {
		outPath += ".exe"
	}

	// Check if the file already exists
	if _, err := os.Stat(outPath); err == nil {
		// File already exists, no need to extract again
		return outPath, nil
	}

	if err := os.WriteFile(outPath, binaryData, 0755); err != nil {
		return "", err
	}

	return outPath, nil
}
