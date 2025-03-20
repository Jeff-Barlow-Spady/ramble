// Package transcription provides speech-to-text functionality
package transcription

import (
	"errors"
)

// Common error types for the transcription package
var (
	// ErrExecutableNotFound indicates that no whisper executable could be found
	ErrExecutableNotFound = errors.New("whisper executable not found")

	// ErrExecutableInstallFailed indicates that automatic installation of the whisper executable failed
	ErrExecutableInstallFailed = errors.New("whisper executable installation failed")

	// ErrInvalidExecutablePath indicates that the provided executable path does not exist or is not valid
	ErrInvalidExecutablePath = errors.New("invalid whisper executable path")

	// ErrModelDownloadFailed indicates that downloading the model failed
	ErrModelDownloadFailed = errors.New("failed to download whisper model")

	// ErrModelNotFound indicates that the model was not found in any of the standard locations
	ErrModelNotFound = errors.New("whisper model not found")

	// ErrTranscriptionFailed indicates that the transcription process failed
	ErrTranscriptionFailed = errors.New("transcription process failed")

	// ErrPlatformNotSupported indicates that the platform does not support auto-installation
	ErrPlatformNotSupported = errors.New("platform not supported for whisper auto-installation")
)
