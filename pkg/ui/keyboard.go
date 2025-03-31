// Package ui provides the user interface for the transcription app
package ui

import (
	"runtime"

	"github.com/jeff-barlow-spady/ramble/pkg/logger"
)

// SimulatePaste provides information about how to paste text
// This function logs useful information for debugging and will be expanded
// in future versions with actual paste simulation if permissions allow
func SimulatePaste() error {
	// Log the operation
	logger.Info(logger.CategoryUI, "Attempting to help with paste operation")

	// Information about system for debugging
	platform := runtime.GOOS

	// Log platform-specific paste shortcut
	if platform == "darwin" {
		logger.Info(logger.CategoryUI, "On macOS, paste shortcut is Command+V")
	} else {
		logger.Info(logger.CategoryUI, "On %s, paste shortcut is Ctrl+V", platform)
	}

	// Currently we only support pasting via clipboard
	// This function would be extended in future versions with actual
	// keyboard simulation if the necessary permissions are available
	logger.Info(logger.CategoryUI, "Text is in clipboard - ready for manual pasting")

	return nil
}
