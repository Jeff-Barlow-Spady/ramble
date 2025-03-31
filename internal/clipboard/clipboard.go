// Package clipboard provides utilities for working with the system clipboard
package clipboard

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/atotto/clipboard"
	"github.com/jeff-barlow-spady/ramble/pkg/logger"
)

// SetText puts text into the system clipboard
func SetText(text string) error {
	// Try the primary method first (atotto/clipboard)
	err := clipboard.WriteAll(text)
	if err == nil {
		// Success with primary method
		logger.Debug(logger.CategoryUI, "Text successfully copied to clipboard via primary method")
		return nil
	}

	// Log the error from the primary method
	logger.Warning(logger.CategoryUI, "Primary clipboard method failed: %v", err)

	// If primary method fails, try platform-specific fallbacks
	// This is especially useful in container or remote desktop environments
	var fallbackErr error

	switch runtime.GOOS {
	case "linux":
		// Try xclip
		if hasCommand("xclip") {
			cmd := exec.Command("xclip", "-selection", "clipboard")
			cmd.Stdin = os.Stdin
			stdin, err := cmd.StdinPipe()
			if err == nil {
				go func() {
					defer stdin.Close()
					fmt.Fprint(stdin, text)
				}()
				fallbackErr = cmd.Run()
				if fallbackErr == nil {
					logger.Debug(logger.CategoryUI, "Text copied to clipboard using xclip")
					return nil
				}
			}
		}

		// Try xsel
		if hasCommand("xsel") {
			cmd := exec.Command("xsel", "--clipboard", "--input")
			cmd.Stdin = os.Stdin
			stdin, err := cmd.StdinPipe()
			if err == nil {
				go func() {
					defer stdin.Close()
					fmt.Fprint(stdin, text)
				}()
				fallbackErr = cmd.Run()
				if fallbackErr == nil {
					logger.Debug(logger.CategoryUI, "Text copied to clipboard using xsel")
					return nil
				}
			}
		}

	case "darwin":
		// Try pbcopy on macOS
		if hasCommand("pbcopy") {
			cmd := exec.Command("pbcopy")
			cmd.Stdin = os.Stdin
			stdin, err := cmd.StdinPipe()
			if err == nil {
				go func() {
					defer stdin.Close()
					fmt.Fprint(stdin, text)
				}()
				fallbackErr = cmd.Run()
				if fallbackErr == nil {
					logger.Debug(logger.CategoryUI, "Text copied to clipboard using pbcopy")
					return nil
				}
			}
		}

	case "windows":
		// On Windows, we mostly rely on the atotto/clipboard library
		// But we could implement PowerShell clipboard access as a fallback
		fallbackErr = fmt.Errorf("Windows clipboard fallback not implemented")
	}

	// If we reach here, all methods have failed
	if fallbackErr != nil {
		logger.Error(logger.CategoryUI, "All clipboard methods failed. Last error: %v", fallbackErr)
		return fmt.Errorf("clipboard copy failed: %v, %v", err, fallbackErr)
	}

	return err
}

// GetText retrieves text from the system clipboard
func GetText() (string, error) {
	return clipboard.ReadAll()
}

// AppendText appends text to the current clipboard content
func AppendText(text string) error {
	current, err := GetText()
	if err != nil {
		return err
	}

	return SetText(current + text)
}

// hasCommand checks if a command is available in the PATH
func hasCommand(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}
