// Package transcription provides speech-to-text functionality
package transcription

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/jeff-barlow-spady/ramble/pkg/logger"
)

const (
	// WhisperBaseURL is the base URL for Whisper models
	WhisperBaseURL = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-"
)

// WhisperModelFilenames maps model size to filename
var WhisperModelFilenames = map[ModelSize]string{
	ModelTiny:   "tiny.en.bin",
	ModelBase:   "base.en.bin",
	ModelSmall:  "small.en.bin",
	ModelMedium: "medium.en.bin",
	ModelLarge:  "large-v3.en.bin",
}

// DownloadModel downloads a Whisper model if it doesn't exist
func DownloadModel(modelPath string, modelSize ModelSize) (string, error) {
	// Check if model already exists
	if _, err := os.Stat(modelPath); err == nil {
		// Model already exists, return its path
		return modelPath, nil
	}

	// Determine model filename
	modelFilename := fmt.Sprintf("ggml-%s.en.bin", modelSize)

	// Create directory for model if it doesn't exist
	modelDir := filepath.Dir(modelPath)
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return "", fmt.Errorf("%w: failed to create model directory %s: %v",
			ErrModelDownloadFailed, modelDir, err)
	}

	// Get the model file
	modelFile := filepath.Join(modelDir, modelFilename)

	// Check if we have to download or if it already exists
	if _, err := os.Stat(modelFile); os.IsNotExist(err) {
		// Model doesn't exist, download it
		logger.Info(logger.CategoryTranscription, "Downloading whisper model %s to %s", modelSize, modelFile)

		// Ensure the download directory exists
		if err := os.MkdirAll(filepath.Dir(modelFile), 0755); err != nil {
			return "", fmt.Errorf("%w: failed to create download directory: %v",
				ErrModelDownloadFailed, err)
		}

		// Download the model
		if err := downloadModelFile(modelFile, modelFilename); err != nil {
			return "", fmt.Errorf("%w: %v", ErrModelDownloadFailed, err)
		}

		logger.Info(logger.CategoryTranscription, "Download complete: %s", modelFile)
	} else {
		logger.Info(logger.CategoryTranscription, "Using existing model file: %s", modelFile)
	}

	return modelFile, nil
}

// downloadModelFile downloads a Whisper model file from HuggingFace
func downloadModelFile(outputPath, modelFilename string) error {
	// HuggingFace URL for whisper.cpp models
	baseURL := "https://huggingface.co/ggerganov/whisper.cpp/resolve/main"
	modelURL := fmt.Sprintf("%s/%s", baseURL, modelFilename)

	// Create temporary file
	tempFile, err := os.CreateTemp(filepath.Dir(outputPath), "model-download-*.bin")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	// Perform HTTP request
	resp, err := http.Get(modelURL)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: HTTP response error: %s", ErrModelDownloadFailed, resp.Status)
	}

	// Download with progress
	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		return fmt.Errorf("%w: download failed: %v", ErrModelDownloadFailed, err)
	}

	// Close the file
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	// Move the temporary file to the target location
	if err := os.Rename(tempFile.Name(), outputPath); err != nil {
		return fmt.Errorf("%w: failed to move downloaded file to final destination: %v",
			ErrModelDownloadFailed, err)
	}

	return nil
}

// progressWriter tracks download progress
type progressWriter struct {
	total        int64
	downloaded   *int64
	lastReported *int64
}

// Write updates progress and logs it periodically
func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	*pw.downloaded += int64(n)

	// Only report progress every 10MB or at the end
	if pw.total > 0 && (*pw.downloaded-*pw.lastReported > 10*1024*1024 || *pw.downloaded == pw.total) {
		percentage := float64(*pw.downloaded) / float64(pw.total) * 100
		downloadedMB := float64(*pw.downloaded) / 1024 / 1024
		totalMB := float64(pw.total) / 1024 / 1024

		logger.Info(logger.CategoryTranscription, "Downloaded %.1f MB of %.1f MB (%.1f%%)",
			downloadedMB, totalMB, percentage)
		*pw.lastReported = *pw.downloaded
	}

	return n, nil
}
