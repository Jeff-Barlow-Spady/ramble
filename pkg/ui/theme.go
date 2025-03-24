// Package ui provides the user interface for the transcription app
package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// RambleTheme is a custom theme for the application that ensures disabled text is visible
type RambleTheme struct {
	baseTheme fyne.Theme
}

// NewRambleTheme creates a new RambleTheme instance
func NewRambleTheme(dark bool) *RambleTheme {
	if dark {
		return &RambleTheme{baseTheme: theme.DarkTheme()}
	}
	return &RambleTheme{baseTheme: theme.LightTheme()}
}

// Color returns the color for a named color element
func (t *RambleTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	// Make disabled text entries more visible with higher contrast
	if name == theme.ColorNameDisabled {
		// Use a very light gray with high alpha for better visibility
		return color.NRGBA{R: 220, G: 220, B: 220, A: 255} // Almost white text
	}

	// For disabled background, make it close to regular background for better contrast
	if name == theme.ColorNameInputBackground {
		bg := t.baseTheme.Color(theme.ColorNameBackground, variant)
		if r, _, _, _ := bg.RGBA(); r > 0x8000 { // Is it a light background?
			// Use a slightly darker version for input
			return color.NRGBA{R: 240, G: 240, B: 240, A: 255}
		}
		// For dark theme, use a slightly lighter background
		return color.NRGBA{R: 40, G: 42, B: 48, A: 255}
	}

	// Enhance foreground text color for better readability
	if name == theme.ColorNameForeground {
		// Use a bright white for maximum contrast in dark theme
		return color.NRGBA{R: 240, G: 240, B: 245, A: 255}
	}

	return t.baseTheme.Color(name, variant)
}

// Font returns the font for a text style
func (t *RambleTheme) Font(style fyne.TextStyle) fyne.Resource {
	return t.baseTheme.Font(style)
}

// Icon returns the icon resource for an icon name
func (t *RambleTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.baseTheme.Icon(name)
}

// Size returns the size for a specific element
func (t *RambleTheme) Size(name fyne.ThemeSizeName) float32 {
	// Increase the padding and text size for better readability
	switch name {
	case theme.SizeNameText:
		return t.baseTheme.Size(name) * 1.3 // 30% larger text (increased from 20%)
	case theme.SizeNamePadding:
		return t.baseTheme.Size(name) * 1.1 // 10% more padding
	case theme.SizeNameInputBorder:
		return 2.0 // Thicker border for input fields
	}
	return t.baseTheme.Size(name)
}
