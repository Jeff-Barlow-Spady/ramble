package ui

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"time"

	"errors"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	appASCIIBanner = `
 ██████╗  █████╗ ███╗   ███╗██████╗ ██╗     ███████╗
 ██╔══██╗██╔══██╗████╗ ████║██╔══██╗██║     ██╔════╝
 ██████╔╝███████║██╔████╔██║██████╔╝██║     █████╗
 ██╔══██╗██╔══██║██║╚██╔╝██║██╔══██╗██║     ██╔══╝
 ██║  ██║██║  ██║██║ ╚═╝ ██║██████╔╝███████╗███████╗
 ╚═╝  ╚═╝╚═╝  ╚═╝╚═╝     ╚═╝╚═════╝ ╚══════╝╚══════╝
            Speech-to-Text Service
`
	appVersion = "v0.1.0"
)

// Define some styles
var (
	appStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#61E3FA")).
			Background(lipgloss.Color("#1E1E2E")).
			Padding(1, 2)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A9B1D6"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9ECE6A")).
			MarginTop(1)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F7768E"))

	frameStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7AA2F7")).
			Padding(1, 2)
)

// TerminalModel is the TUI model
type TerminalModel struct {
	spinner       spinner.Model
	audioLevels   []float32
	text          string
	isRecording   bool
	statusMessage string
	errorMessage  string
	width         int
	height        int
	mutex         sync.Mutex
	ready         bool
	hotkeyStr     string
	logMessages   []string      // Store log messages
	maxLogLines   int           // Maximum number of log lines to show in view
	statusChan    chan struct{} // Channel for keyboard shortcuts
	logScrollPos  int           // Current scroll position in logs
	maxLogHistory int           // Maximum number of log messages to keep in history
}

// NewTerminalModel creates a new TUI model
func NewTerminalModel(hotkeyStr string) TerminalModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#9ECE6A"))

	return TerminalModel{
		spinner:       s,
		audioLevels:   make([]float32, 20),
		hotkeyStr:     hotkeyStr,
		isRecording:   false,
		statusMessage: "Ready",
		maxLogLines:   10, // Display 10 log lines at a time
		logMessages:   make([]string, 0),
		statusChan:    make(chan struct{}, 1),
		ready:         false,
		logScrollPos:  0,
		maxLogHistory: 500, // Keep up to 500 log messages in history
	}
}

// Init initializes the model
func (m *TerminalModel) Init() tea.Cmd {
	// Return multiple commands for initialization
	return tea.Batch(
		spinner.Tick,              // Start the spinner
		tea.EnterAltScreen,        // Use alt screen for cleaner UI
		tickEvery(time.Second/10), // Force refresh 10 times per second for smooth updates
	)
}

// tickMsg is sent when the ticker fires
type tickMsg time.Time

// tickEvery returns a command that ticks at the specified interval
func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update updates the model based on messages
func (m *TerminalModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case " ", "r":
			// Space or 'r' toggles recording
			select {
			case m.statusChan <- struct{}{}:
				// Signal sent, move on
			default:
				// Channel is full, just continue
			}
			return m, nil

		// Add keyboard navigation for logs
		case "up":
			// Scroll logs up (show older logs)
			if m.logScrollPos < len(m.logMessages)-m.maxLogLines {
				m.logScrollPos++
			}
			return m, nil

		case "down":
			// Scroll logs down (show newer logs)
			if m.logScrollPos > 0 {
				m.logScrollPos--
			}
			return m, nil

		case "pgup":
			// Page up - scroll a page at a time
			m.logScrollPos += m.maxLogLines
			if m.logScrollPos > len(m.logMessages)-m.maxLogLines {
				m.logScrollPos = len(m.logMessages) - m.maxLogLines
			}
			if m.logScrollPos < 0 {
				m.logScrollPos = 0
			}
			return m, nil

		case "pgdown":
			// Page down - scroll a page at a time
			m.logScrollPos -= m.maxLogLines
			if m.logScrollPos < 0 {
				m.logScrollPos = 0
			}
			return m, nil

		case "home":
			// Jump to newest logs
			m.logScrollPos = 0
			return m, nil

		case "end":
			// Jump to oldest logs
			m.logScrollPos = len(m.logMessages) - m.maxLogLines
			if m.logScrollPos < 0 {
				m.logScrollPos = 0
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tickMsg:
		// Just force a refresh and schedule the next one
		return m, tickEvery(time.Second / 10)
	}

	return m, nil
}

// UpdateText updates the transcribed text
func (m *TerminalModel) UpdateText(text string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.text = text
}

// UpdateAudioLevel updates the audio level visualization
func (m *TerminalModel) UpdateAudioLevel(level float32) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Shift all levels one position to the right
	copy(m.audioLevels[1:], m.audioLevels)
	m.audioLevels[0] = level
}

// SetRecordingState updates the recording state
func (m *TerminalModel) SetRecordingState(isRecording bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.isRecording = isRecording
	if isRecording {
		m.statusMessage = "Recording..."
	} else {
		m.statusMessage = "Ready"
	}
}

// SetError sets an error message
func (m *TerminalModel) SetError(err string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.errorMessage = err
}

// AddLogMessage adds a log message to the display
func (m *TerminalModel) AddLogMessage(msg string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Add new message to the beginning
	m.logMessages = append([]string{msg}, m.logMessages...)

	// Trim history if it exceeds our maximum size
	if len(m.logMessages) > m.maxLogHistory {
		m.logMessages = m.logMessages[:m.maxLogHistory]
	}
}

// View renders the TUI
func (m *TerminalModel) View() string {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.ready {
		return "Initializing..."
	}

	var s strings.Builder

	// Build the banner
	banner := appStyle.Render(appASCIIBanner)
	s.WriteString(banner)

	// Status indicator
	statusIndicator := ""
	if m.isRecording {
		statusIndicator = m.spinner.View() + " "
	}
	statusLine := statusStyle.Render(statusIndicator + "Status: " + m.statusMessage)
	s.WriteString("\n" + statusLine)

	// Hotkey info with added scroll help
	hotkeyInfo := infoStyle.Render("Hotkey: " + m.hotkeyStr + " | Press 'r' or SPACE to toggle recording | Press 'q' to quit | Scroll logs: ↑/↓ arrows")
	s.WriteString("\n" + hotkeyInfo)

	// Audio visualization
	audioViz := renderAudioVisualization(m.audioLevels, m.isRecording)
	s.WriteString("\n\n" + audioViz)

	// Text output in a frame
	textArea := "No transcription yet..."
	if m.text != "" {
		textArea = m.text
	}
	framedText := frameStyle.Width(m.width - 4).Render(textArea)
	s.WriteString("\n\n" + framedText)

	// Error message (if any)
	if m.errorMessage != "" {
		errorMsg := errorStyle.Render("Error: " + m.errorMessage)
		s.WriteString("\n\n" + errorMsg)
	}

	// Log messages with a heading and styled container
	if len(m.logMessages) > 0 {
		// Create a styled log frame
		var logContent strings.Builder

		// Show scroll position information
		if m.logScrollPos > 0 {
			logContent.WriteString(fmt.Sprintf("Recent Activity (Scrolled - Page %d/%d):\n",
				m.logScrollPos+1, (len(m.logMessages)-1)/m.maxLogLines+1))
		} else {
			logContent.WriteString("Recent Activity (Latest):\n")
		}

		// Calculate visible range
		startIdx := m.logScrollPos
		endIdx := startIdx + m.maxLogLines
		if endIdx > len(m.logMessages) {
			endIdx = len(m.logMessages)
		}

		// Show scroll indicators
		if startIdx > 0 {
			logContent.WriteString("↑ More logs above ↑\n")
		}

		// Display the selected log messages
		for i, msg := range m.logMessages[startIdx:endIdx] {
			// Show the newest message with a special marker if we're at the top
			if i == 0 && startIdx == 0 {
				logContent.WriteString("→ " + msg + "\n")
			} else {
				logContent.WriteString("• " + msg + "\n")
			}
		}

		// Show indicator for more logs below
		if endIdx < len(m.logMessages) {
			logContent.WriteString("↓ More logs below ↓\n")
		}

		// Create a styled log container
		logFrame := lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#A9B1D6")).
			Padding(0, 1).
			Render(logContent.String())

		s.WriteString("\n\n" + logFrame)
	}

	return s.String()
}

// renderAudioVisualization creates a text-based visualization of audio levels
func renderAudioVisualization(levels []float32, isRecording bool) string {
	var s strings.Builder
	s.WriteString("Audio Level: ")

	// Base color for inactive state
	baseColor := "#555555"
	if isRecording {
		baseColor = "#7AA2F7"
	}

	// Use block elements for the visualization
	const width = 30
	s.WriteString("[")
	for i := 0; i < width; i++ {
		ratio := float32(i) / float32(width)
		threshold := float32(1.0 - ratio)

		// Find the level to display (using the most recent levels that fit in our width)
		levelIdx := i % len(levels)
		level := levels[levelIdx]

		// Choose character and color based on level
		var char string
		var color string

		if level >= threshold {
			char = "█"
			color = getColorForLevel(level)
		} else {
			char = " "
			color = baseColor
		}

		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(char))
	}
	s.WriteString("]")

	return s.String()
}

// getColorForLevel returns a color based on audio level
func getColorForLevel(level float32) string {
	switch {
	case level > 0.8:
		return "#F7768E" // Red for high levels
	case level > 0.5:
		return "#FF9E64" // Orange for medium-high levels
	case level > 0.3:
		return "#E0AF68" // Yellow for medium levels
	default:
		return "#9ECE6A" // Green for low levels
	}
}

// TerminalUI manages the terminal user interface
type TerminalUI struct {
	program       *tea.Program
	model         *TerminalModel
	initializedCh chan struct{}
	logCh         chan string
	statusChan    chan struct{} // Channel for keyboard shortcuts
}

// NewTerminalUI creates a new terminal UI
func NewTerminalUI(hotkeyStr string) *TerminalUI {
	model := NewTerminalModel(hotkeyStr)
	program := tea.NewProgram(&model)

	ui := &TerminalUI{
		program:       program,
		model:         &model,
		initializedCh: make(chan struct{}),
		logCh:         make(chan string, 10),
		statusChan:    model.statusChan,
	}

	// Start log channel handler
	go func() {
		for msg := range ui.logCh {
			ui.model.AddLogMessage(msg)
			ui.program.Send(tea.Tick(0, func(t time.Time) tea.Msg {
				return nil // Just trigger a redraw
			}))
		}
	}()

	return ui
}

// Start begins the terminal UI in a goroutine
func (t *TerminalUI) Start() {
	go func() {
		// Run the terminal UI
		if err := t.program.Start(); err != nil {
			if !errors.Is(err, tea.ErrProgramKilled) {
				// Only log non-normal exit errors
				t.AddLog("Terminal UI error: " + err.Error())
			}
		}
		close(t.initializedCh)
	}()
}

// Stop terminates the terminal UI
func (t *TerminalUI) Stop() {
	t.program.Quit()
}

// UpdateText updates the transcribed text
func (t *TerminalUI) UpdateText(text string) {
	t.model.UpdateText(text)
}

// UpdateAudioLevel updates the audio level visualization
func (t *TerminalUI) UpdateAudioLevel(level float32) {
	t.model.UpdateAudioLevel(level)
}

// SetRecordingState updates the recording state
func (t *TerminalUI) SetRecordingState(isRecording bool) {
	t.model.SetRecordingState(isRecording)
}

// SetError sets an error message
func (t *TerminalUI) SetError(err string) {
	t.model.SetError(err)
}

// AddLog adds a log message to the display
func (t *TerminalUI) AddLog(msg string) {
	select {
	case t.logCh <- msg:
		// Message sent
	default:
		// Channel full, drop message
	}
}

// RunBlocking runs the TUI in the current goroutine (blocking)
func (t *TerminalUI) RunBlocking() error {
	if err := t.program.Start(); err != nil {
		if !errors.Is(err, tea.ErrProgramKilled) {
			return err
		}
	}
	return nil
}

// GetStatusChannel returns a channel that receives toggle signals
func (t *TerminalUI) GetStatusChannel() <-chan struct{} {
	return t.statusChan
}

// LogConsumer is an interface for components that can consume log messages
type LogConsumer interface {
	AddLog(message string)
}

// LogBuffer is a thread-safe buffer that captures logs for display in the UI
// It implements io.Writer so it can be used with the logger
type LogBuffer struct {
	buffer   bytes.Buffer
	mu       sync.Mutex
	consumer LogConsumer
}

// Write implements io.Writer
func (lb *LogBuffer) Write(p []byte) (n int, err error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	// Write to our internal buffer
	n, err = lb.buffer.Write(p)

	// Extract complete lines and pass to consumer if available
	if lb.consumer != nil {
		// Convert buffer to string and extract lines
		data := lb.buffer.String()
		lines := strings.Split(data, "\n")

		// Only process if we have at least one complete line (ending with newline)
		if len(lines) > 1 {
			// Pass all complete lines to the consumer
			for _, line := range lines[:len(lines)-1] {
				// Skip empty lines
				if strings.TrimSpace(line) != "" {
					lb.consumer.AddLog(strings.TrimSpace(line))
				}
			}

			// Keep incomplete line (if any) in the buffer
			lb.buffer.Reset()
			if lines[len(lines)-1] != "" {
				lb.buffer.WriteString(lines[len(lines)-1])
			}
		}
	}

	return n, err
}

// SetLogConsumer sets the consumer that will receive log messages
func (lb *LogBuffer) SetLogConsumer(consumer LogConsumer) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.consumer = consumer
}
