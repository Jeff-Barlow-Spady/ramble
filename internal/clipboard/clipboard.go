// Package clipboard provides utilities for working with the system clipboard
package clipboard

import (
	"github.com/atotto/clipboard"
)

// SetText puts text into the system clipboard
func SetText(text string) error {
	return clipboard.WriteAll(text)
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
