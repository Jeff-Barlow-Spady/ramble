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
	isDark    bool
}

// NewRambleTheme creates a new RambleTheme instance
func NewRambleTheme(dark bool) *RambleTheme {
	if dark {
		return &RambleTheme{baseTheme: theme.DarkTheme(), isDark: true}
	}
	return &RambleTheme{baseTheme: theme.LightTheme(), isDark: false}
}

// Color returns the color for a named color element
func (t *RambleTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	// Dark theme colors - based on the example screenshot
	if t.isDark {
		switch name {
		case theme.ColorNameBackground:
			return color.NRGBA{R: 19, G: 26, B: 56, A: 255} // Deep navy blue background
		case theme.ColorNameButton:
			return color.NRGBA{R: 30, G: 36, B: 60, A: 255} // Slightly lighter navy for buttons
		case theme.ColorNameDisabled:
			return color.NRGBA{R: 180, G: 180, B: 180, A: 255} // Light gray for disabled elements
		case theme.ColorNameForeground:
			return color.NRGBA{R: 240, G: 240, B: 245, A: 255} // Bright white text
		case theme.ColorNamePlaceHolder:
			return color.NRGBA{R: 150, G: 150, B: 160, A: 255} // Medium gray for placeholders
		case theme.ColorNamePrimary:
			return color.NRGBA{R: 100, G: 140, B: 240, A: 255} // Bright blue for primary elements
		case theme.ColorNameScrollBar:
			return color.NRGBA{R: 60, G: 70, B: 100, A: 255} // Medium blue for scroll bars
		case theme.ColorNameShadow:
			return color.NRGBA{R: 10, G: 10, B: 10, A: 100} // Near black for shadows
		case theme.ColorNameInputBackground:
			return color.NRGBA{R: 30, G: 36, B: 66, A: 255} // Slightly lighter than background for input fields
		case theme.ColorNameHover:
			return color.NRGBA{R: 40, G: 50, B: 90, A: 255} // Highlight color for hover
		case theme.ColorNameSelection:
			return color.NRGBA{R: 50, G: 80, B: 150, A: 255} // Selection color
		}
	} else {
		// Light theme - make some specific adjustments for better readability
		switch name {
		case theme.ColorNameDisabled:
			// For disabled text in light theme, use dark gray rather than light gray
			return color.NRGBA{R: 30, G: 30, B: 30, A: 255}
		case theme.ColorNameInputBackground:
			// Light input background
			return color.NRGBA{R: 240, G: 240, B: 240, A: 255}
		}
	}

	// Fall back to base theme for any colors not explicitly defined
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
		return t.baseTheme.Size(name) * 1.3 // 30% larger text
	case theme.SizeNamePadding:
		return t.baseTheme.Size(name) * 1.1 // 10% more padding
	case theme.SizeNameInputBorder:
		return 2.0 // Thicker border for input fields
	}
	return t.baseTheme.Size(name)
}
