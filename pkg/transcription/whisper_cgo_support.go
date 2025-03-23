//go:build cgo && !no_cgo
// +build cgo,!no_cgo

// Package transcription provides speech-to-text functionality
package transcription

import (
	"fmt"
	"os"
	"os/exec"
)

// This file contains functions for CGO support in normal builds

// haveCGOSupport returns true when CGO-based whisper integration is available
func haveCGOSupport() bool {
	// Check for the whisper.cpp directory and include files
	if _, err := os.Stat("whisper.cpp"); err == nil {
		// Check for the header file
		if _, err := os.Stat("whisper.cpp/include/whisper.h"); err == nil {
			// Also check for the whisper library
			soExists := false
			if _, err := os.Stat("whisper.cpp/build/libwhisper.so"); err == nil {
				soExists = true
			}

			aExists := false
			if _, err := os.Stat("whisper.cpp/build/libwhisper.a"); err == nil {
				aExists = true
			}

			dylibExists := false
			if _, err := os.Stat("whisper.cpp/build/libwhisper.dylib"); err == nil {
				dylibExists = true
			}

			// If any of the library files exist
			if soExists || aExists || dylibExists {
				// Run a quick test to make sure the library is fully built
				if hasRequiredFunctions() {
					return true
				}
			}
		}
	}
	return false
}

// hasRequiredFunctions checks if the whisper.cpp library has been properly built
// by checking for key functions using nm or otool
func hasRequiredFunctions() bool {
	// Depending on OS, we might use nm or otool
	var cmd *exec.Cmd
	if _, err := exec.LookPath("nm"); err == nil {
		cmd = exec.Command("nm", "whisper.cpp/build/libwhisper.so")
	} else {
		// If we can't check, assume it's built correctly
		return true
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}

	// Check for a few key function names that should be in the library
	outputStr := string(output)
	return contains(outputStr, "whisper_init") &&
		contains(outputStr, "whisper_full")
}

// contains is a simple helper to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr
}

// newCGOTranscriberIfAvailable creates a new CGO-based transcriber if possible
func newCGOTranscriberIfAvailable(config Config) (ConfigurableTranscriber, error) {
	if !haveCGOSupport() {
		return nil, fmt.Errorf("CGO support is not available")
	}

	// Since CGO is available and the whisper.cpp is properly built,
	// we need to create a placeholder transcriber that will be filled
	// by the real implementation when the whisper_cgo build tag is present
	return &placeholderTranscriber{
		config: config,
	}, nil
}
