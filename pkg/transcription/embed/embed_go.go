//go:build whisper_go
// +build whisper_go

// Package embed provides a stub implementation for whisper_go builds
// This avoids build errors when using the whisper.cpp Go bindings
package embed

import (
	"errors"
	"os"
	"sync"
)

// ErrAssetsNotEmbedded indicates embedded assets are not available
var ErrAssetsNotEmbedded = errors.New("embedded assets not available with whisper_go build")

// WhisperExecutableType represents the type of the embedded Whisper executable (stub for whisper_go)
type WhisperExecutableType int

const (
	// WhisperCpp is the C++ implementation of Whisper
	WhisperCpp WhisperExecutableType = iota
	// WhisperGael is the Python implementation by Gael
	WhisperGael
	// WhisperPython is a generic Python implementation
	WhisperPython
	// WhisperCppStream is the C++ implementation that specifically supports stdin streaming
	WhisperCppStream
)

// Variables and functions for temp file cleanup
var (
	tempFiles     = make([]string, 0)
	tempFilesLock sync.Mutex
)

// RegisterTempFile adds a file to be cleaned up later
func RegisterTempFile(path string) {
	tempFilesLock.Lock()
	defer tempFilesLock.Unlock()
	tempFiles = append(tempFiles, path)
}

// CleanupTempFiles removes all temporary files
func CleanupTempFiles() {
	tempFilesLock.Lock()
	defer tempFilesLock.Unlock()

	for _, path := range tempFiles {
		os.RemoveAll(path)
	}
	tempFiles = tempFiles[:0]
}

// HasEmbeddedAssets always returns false for whisper_go builds
func HasEmbeddedAssets() bool {
	return false
}

// GetEmbeddedExecutableType returns the type of the embedded whisper executable (stub for whisper_go)
func GetEmbeddedExecutableType() WhisperExecutableType {
	return WhisperCppStream
}

// ExtractModel returns an error for whisper_go builds
func ExtractModel(modelSize string) (string, error) {
	return "", ErrAssetsNotEmbedded
}

// GetWhisperExecutable returns an error for whisper_go builds
func GetWhisperExecutable() (string, error) {
	return "", ErrAssetsNotEmbedded
}
