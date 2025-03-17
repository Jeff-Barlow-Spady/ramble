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
	// Get the model filename
	modelFilename, ok := WhisperModelFilenames[modelSize]
	if !ok {
		return "", fmt.Errorf("unknown model size: %s", modelSize)
	}

	// Full path to the model file
	modelFile := filepath.Join(modelPath, modelFilename)

	// Check if the model exists
	if _, err := os.Stat(modelFile); os.IsNotExist(err) {
		logger.Info(logger.CategoryTranscription, "Model file %s not found. Downloading...", modelFile)

		// Create the model directory if it doesn't exist
		if err := os.MkdirAll(modelPath, 0755); err != nil {
			return "", fmt.Errorf("failed to create model directory: %w", err)
		}

		// Download the model
		if err := downloadModelFile(modelFile, modelFilename); err != nil {
			return "", fmt.Errorf("failed to download model: %w", err)
		}

		logger.Info(logger.CategoryTranscription, "Model downloaded successfully: %s", modelFile)
	} else if err != nil {
		return "", fmt.Errorf("error checking model file: %w", err)
	} else {
		logger.Info(logger.CategoryTranscription, "Using existing model file: %s", modelFile)
	}

	return modelFile, nil
}

// downloadModelFile downloads a Whisper model file from HuggingFace
func downloadModelFile(outputPath, modelFilename string) error {
	// Create the URL
	url := WhisperBaseURL + modelFilename

	// Get the model size first
	resp, err := http.Head(url)
	if err != nil {
		return err
	}

	// Check if the response is valid
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download model: HTTP %s", resp.Status)
	}

	contentLength := resp.ContentLength
	var totalSize int64
	if contentLength > 0 {
		totalSize = contentLength / (1024 * 1024) // Convert to MB
		logger.Info(logger.CategoryTranscription, "Downloading model (%d MB). This may take a while...", totalSize)
	} else {
		logger.Info(logger.CategoryTranscription, "Downloading model. Size unknown. This may take a while...")
	}

	// Open the URL
	resp, err = http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the output file
	out, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Create a progress tracker
	downloaded := int64(0)
	lastReported := int64(0)

	// Create a wrapped reader for progress tracking
	reader := io.TeeReader(resp.Body, &progressWriter{
		total:        contentLength,
		downloaded:   &downloaded,
		lastReported: &lastReported,
	})

	// Copy the data
	_, err = io.Copy(out, reader)
	if err != nil {
		// Clean up the partial file on error
		out.Close()
		os.Remove(outputPath)
		return err
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
