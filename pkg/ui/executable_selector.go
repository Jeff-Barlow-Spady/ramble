package ui

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/jeff-barlow-spady/ramble/pkg/logger"
)

// ExecutableSelectorUI is the interface that should be implemented by UI components
// that allow the user to select an executable
type ExecutableSelectorUI interface {
	SelectExecutable(executables []string) (string, error)
}

// TerminalExecutableSelector implements selection via terminal UI
type TerminalExecutableSelector struct{}

// NewTerminalExecutableSelector creates a new terminal-based executable selector
func NewTerminalExecutableSelector() *TerminalExecutableSelector {
	return &TerminalExecutableSelector{}
}

// SelectExecutable presents the user with a list of executables and lets them choose one
func (s *TerminalExecutableSelector) SelectExecutable(executables []string) (string, error) {
	if len(executables) == 0 {
		return "", errors.New("no executables to select from")
	}

	fmt.Println("\nMultiple Whisper executables found. Please select one:")
	fmt.Println("-------------------------------------------------------")

	// Display the list of executables with more informative descriptions
	for i, exe := range executables {
		// Get the executable name without the path
		exeName := filepath.Base(exe)

		// Determine the type of executable for better description
		var exeType string
		switch {
		case strings.Contains(exeName, "whisper-cpp") || strings.Contains(exeName, "whisper.exe"):
			exeType = "whisper.cpp implementation"
		case strings.Contains(exeName, "whisper-gael"):
			exeType = "whisper-gael Python implementation"
		case strings.HasSuffix(exeName, ".py"):
			exeType = "Python implementation"
		default:
			exeType = "Whisper implementation"
		}

		fmt.Printf("%d. %s (%s)\n   Location: %s\n\n", i+1, exeName, exeType, exe)
	}

	// Prompt for selection
	fmt.Print("Enter your selection (1-" + strconv.Itoa(len(executables)) + "): ")
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		logger.Warning(logger.CategoryTranscription, "Failed to read user input: %v", err)
		return executables[0], fmt.Errorf("failed to read input: %w", err)
	}

	// Parse the selection
	input = strings.TrimSpace(input)
	selectedIndex, err := strconv.Atoi(input)
	if err != nil || selectedIndex < 1 || selectedIndex > len(executables) {
		logger.Warning(logger.CategoryTranscription, "Invalid selection: %s", input)
		fmt.Println("Invalid selection. Using the first executable.")
		return executables[0], nil
	}

	selectedExec := executables[selectedIndex-1]
	fmt.Printf("Selected: %s\n", selectedExec)
	return selectedExec, nil
}

// GUIExecutableSelector implements selection via Fyne GUI
type GUIExecutableSelector struct {
	parentWindow fyne.Window
}

// NewGUIExecutableSelector creates a new GUI-based executable selector
func NewGUIExecutableSelector(parent fyne.Window) *GUIExecutableSelector {
	return &GUIExecutableSelector{
		parentWindow: parent,
	}
}

// SelectExecutable displays a dialog for selecting an executable
func (s *GUIExecutableSelector) SelectExecutable(executables []string) (string, error) {
	if len(executables) == 0 {
		return "", errors.New("no executables to select from")
	}

	// Sort executables by path for a more consistent display
	sort.Strings(executables)

	// Set up communication channels
	selectedExec := ""
	var selectionErr error
	var wg sync.WaitGroup
	wg.Add(1)

	// Run dialog on the main thread using Ramble's RunOnMain helper
	RunOnMain(func() {
		// Create a list of executable items for the radio group
		items := make([]string, len(executables))
		for i, exe := range executables {
			// Get the executable name without the path
			exeName := filepath.Base(exe)

			// Determine the type of executable for better description
			var exeType string
			switch {
			case strings.Contains(exeName, "whisper-cpp") || strings.Contains(exeName, "whisper.exe"):
				exeType = "whisper.cpp implementation"
			case strings.Contains(exeName, "whisper-gael"):
				exeType = "whisper-gael Python implementation"
			case strings.HasSuffix(exeName, ".py"):
				exeType = "Python implementation"
			default:
				exeType = "Whisper implementation"
			}

			items[i] = exeName + " (" + exeType + ")\n" + exe
		}

		// Create radio widget
		radio := widget.NewRadioGroup(items, nil)
		radio.Required = true
		radio.Selected = items[0] // Default selection

		// Create dialog
		content := container.NewVBox(
			widget.NewLabel("Multiple Whisper executables found. Please select one:"),
			radio,
		)

		// Use a custom dialog for better control
		dlg := dialog.NewCustom("Select Whisper Executable", "OK", content, s.parentWindow)

		// Set up the callback for when the dialog is dismissed
		dlg.SetOnClosed(func() {
			defer wg.Done()

			if radio.Selected == "" {
				logger.Warning(logger.CategoryTranscription, "No executable selected")
				selectedExec = executables[0]
				return
			}

			// Find which executable was selected
			for i, item := range items {
				if item == radio.Selected {
					selectedExec = executables[i]
					logger.Info(logger.CategoryTranscription, "User selected executable: %s", selectedExec)
					break
				}
			}
		})

		// Show the dialog
		dlg.Show()
	})

	// Wait for dialog to close
	wg.Wait()

	return selectedExec, selectionErr
}
