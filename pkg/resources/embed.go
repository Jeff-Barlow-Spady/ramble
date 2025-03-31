// Package resources handles embedded resources for the application
package resources

import (
	"bytes"
	"embed"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
)

//go:embed icons
var embeddedFiles embed.FS

// LoadAppIcon loads the application icon as a Fyne resource
func LoadAppIcon() fyne.Resource {
	iconData, err := embeddedFiles.ReadFile("icons/ramble.png")
	if err != nil {
		// Try fallback icon
		iconData, err = embeddedFiles.ReadFile("icons/R.png")
		if err != nil {
			log.Println("Warning: Could not load app icon:", err)
			return nil
		}
	}

	return fyne.NewStaticResource("AppIcon", iconData)
}

// GetIconData returns the raw icon data as bytes for use with system tray
func GetIconData() ([]byte, error) {
	iconData, err := embeddedFiles.ReadFile("icons/ramble.png")
	if err != nil {
		// Try fallback icon
		iconData, err = embeddedFiles.ReadFile("icons/R.png")
		if err != nil {
			return nil, err
		}
	}
	return iconData, nil
}

// GetRedIconData returns a red-tinted version of the icon for recording state
func GetRedIconData() ([]byte, error) {
	// Get the original icon
	iconData, err := GetIconData()
	if err != nil {
		return nil, err
	}

	// Decode the PNG image
	img, err := png.Decode(bytes.NewReader(iconData))
	if err != nil {
		return nil, err
	}

	// Create a new RGBA image
	bounds := img.Bounds()
	redIcon := image.NewRGBA(bounds)

	// Apply red tint
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			// Get original pixel color
			originalColor := img.At(x, y)
			r, g, b, a := originalColor.RGBA()

			// Convert to 8-bit color components
			r8 := uint8(r >> 8)
			g8 := uint8(g >> 8)
			b8 := uint8(b >> 8)
			a8 := uint8(a >> 8)

			// Create a redder version - enhance red channel and reduce others
			redColor := color.RGBA{
				R: r8,                       // Keep red as is
				G: uint8(float32(g8) * 0.5), // Reduce green
				B: uint8(float32(b8) * 0.5), // Reduce blue
				A: a8,                       // Keep same alpha
			}

			redIcon.Set(x, y, redColor)
		}
	}

	// Encode back to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, redIcon); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// ExtractIcon extracts the application icon to the specified path
func ExtractIcon(targetPath string) error {
	// Create the directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}

	// Read the icon file
	iconData, err := embeddedFiles.ReadFile("icons/ramble.png")
	if err != nil {
		// Try fallback icon
		iconData, err = embeddedFiles.ReadFile("icons/R.png")
		if err != nil {
			return err
		}
	}

	// Write the icon to the target path
	return os.WriteFile(targetPath, iconData, 0644)
}

// ExtractDesktopFile extracts the desktop entry file to the specified path
func ExtractDesktopFile(targetPath, execPath string) error {
	// Create the directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}

	// Try to read the desktop file
	desktopData, err := embeddedFiles.ReadFile("desktop/ramble.desktop")
	if err != nil {
		// If file doesn't exist, create a basic one
		desktopData = []byte(`[Desktop Entry]
Type=Application
Name=Ramble
Comment=Speech-to-Text Application
Exec=` + execPath + `
Icon=ramble
Terminal=false
Categories=Utility;Audio;`)
	} else {
		// Replace the Exec path with the actual executable path
		desktopStr := string(desktopData)
		desktopStr = replaceExecPath(desktopStr, execPath)
		desktopData = []byte(desktopStr)
	}

	// Write the desktop file to the target path
	return os.WriteFile(targetPath, desktopData, 0644)
}

// replaceExecPath replaces the Exec= line in the desktop file with the actual executable path
func replaceExecPath(content, execPath string) string {
	// Replace the Exec= line with the actual executable path
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "Exec=") {
			lines[i] = "Exec=" + execPath
			break
		}
	}
	return strings.Join(lines, "\n")
}

// ListEmbeddedFiles returns a list of all embedded files
func ListEmbeddedFiles() ([]string, error) {
	var files []string
	err := fs.WalkDir(embeddedFiles, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
