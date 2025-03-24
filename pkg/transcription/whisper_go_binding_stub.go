//go:build cgo && !whisper_go
// +build cgo,!whisper_go

// This file contains a stub implementation of the WhisperGoTranscriber
// It allows the application to compile without the whisper Go bindings.

package transcription

import (
	"fmt"

	"github.com/jeff-barlow-spady/ramble/pkg/logger"
)

// NewGoBindingTranscriber returns a stub that always returns an error
// This implementation is used when Go bindings are not available or not built with the whisper_go tag
func NewGoBindingTranscriber(config Config) (ConfigurableTranscriber, error) {
	logger.Warning(logger.CategoryTranscription,
		"Go bindings for whisper.cpp not available (compiled without whisper_go tag)")

	return nil, fmt.Errorf("whisper.cpp Go bindings are not available; build with -tags=whisper_go to enable")
}
