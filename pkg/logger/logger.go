package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// LogLevel determines which messages are logged
type LogLevel int

const (
	// LevelDebug logs everything including detailed debug information
	LevelDebug LogLevel = iota
	// LevelInfo logs informational messages, warnings, and errors
	LevelInfo
	// LevelWarning logs warnings and errors only
	LevelWarning
	// LevelError logs only errors
	LevelError
	// LevelSilent disables all logging
	LevelSilent
)

// Category represents a subsystem or component for more granular logging
type Category string

const (
	// CategoryAudio for audio-related logs
	CategoryAudio Category = "AUDIO"
	// CategoryUI for user interface logs
	CategoryUI Category = "UI"
	// CategoryTranscription for transcription-related logs
	CategoryTranscription Category = "TRANSCR"
	// CategoryApp for general application logs
	CategoryApp Category = "APP"
	// CategorySystem for system-related logs
	CategorySystem Category = "SYSTEM"
)

var (
	// Default log level
	currentLevel LogLevel = LevelInfo

	// Protect level changes
	mu sync.Mutex

	// Default output is stderr
	output io.Writer = os.Stderr

	// Color support
	useColors = true

	// Suppress repetitive errors
	lastError     string
	errorCount    int
	suppressLines bool
)

// Colors for different log levels (ANSI escape codes)
var (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
)

// SetLevel changes the current logging level
func SetLevel(level LogLevel) {
	mu.Lock()
	defer mu.Unlock()
	currentLevel = level
}

// SetOutput changes where logs are written
func SetOutput(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	output = w
	log.SetOutput(w)
}

// EnableColors turns on ANSI color in log output
func EnableColors(enable bool) {
	mu.Lock()
	defer mu.Unlock()
	useColors = enable
}

// SuppressASLAWarnings prevents certain ALSA warnings from flooding the logs
func SuppressASLAWarnings(suppress bool) {
	mu.Lock()
	defer mu.Unlock()
	suppressLines = suppress
}

// formatLog creates a formatted log message with timestamp, level and category
func formatLog(level LogLevel, category Category, message string) string {
	levelStr := "INFO"
	prefix := ""

	if useColors {
		prefix = colorReset
		switch level {
		case LevelDebug:
			levelStr = "DEBUG"
			prefix = colorBlue
		case LevelInfo:
			levelStr = "INFO"
			prefix = colorGreen
		case LevelWarning:
			levelStr = "WARN"
			prefix = colorYellow
		case LevelError:
			levelStr = "ERROR"
			prefix = colorRed
		}
	} else {
		switch level {
		case LevelDebug:
			levelStr = "DEBUG"
		case LevelInfo:
			levelStr = "INFO"
		case LevelWarning:
			levelStr = "WARN"
		case LevelError:
			levelStr = "ERROR"
		}
	}

	timestamp := time.Now().Format("2006/01/02 15:04:05")

	if useColors {
		return fmt.Sprintf("%s%s [%s] [%s] %s%s",
			prefix, timestamp, levelStr, category, message, colorReset)
	}

	return fmt.Sprintf("%s [%s] [%s] %s",
		timestamp, levelStr, category, message)
}

// shouldLog determines if a message should be logged based on current level
func shouldLog(level LogLevel) bool {
	mu.Lock()
	defer mu.Unlock()
	return level >= currentLevel
}

// isSupressedALSALine checks if a line contains common ALSA warnings that can be suppressed
func isSupressedALSALine(message string) bool {
	if !suppressLines {
		return false
	}

	// Common ALSA warning patterns that should be suppressed
	suppressPatterns := []string{
		"ALSA lib pcm.c:2721:(snd_pcm_open_noupdate) Unknown PCM",
		"ALSA lib pcm_route.c:878:(find_matching_chmap) Found no matching channel map",
	}

	for _, pattern := range suppressPatterns {
		if strings.Contains(message, pattern) {
			return true
		}
	}

	return false
}

// Debug logs at debug level
func Debug(category Category, format string, args ...interface{}) {
	if shouldLog(LevelDebug) {
		message := fmt.Sprintf(format, args...)
		if !isSupressedALSALine(message) {
			log.Println(formatLog(LevelDebug, category, message))
		}
	}
}

// Info logs at info level
func Info(category Category, format string, args ...interface{}) {
	if shouldLog(LevelInfo) {
		message := fmt.Sprintf(format, args...)
		if !isSupressedALSALine(message) {
			log.Println(formatLog(LevelInfo, category, message))
		}
	}
}

// Warning logs at warning level
func Warning(category Category, format string, args ...interface{}) {
	if shouldLog(LevelWarning) {
		message := fmt.Sprintf(format, args...)
		if !isSupressedALSALine(message) {
			log.Println(formatLog(LevelWarning, category, message))
		}
	}
}

// Error logs at error level
func Error(category Category, format string, args ...interface{}) {
	if shouldLog(LevelError) {
		message := fmt.Sprintf(format, args...)

		// Detect repeated errors
		if message == lastError {
			errorCount++
			// Only log every 5th occurrence of the same error
			if errorCount%5 != 0 {
				return
			}
			message = fmt.Sprintf("%s (repeated %d times)", message, errorCount)
		} else {
			lastError = message
			errorCount = 1
		}

		log.Println(formatLog(LevelError, category, message))
	}
}

// Initialize sets up the logger with default settings
func Initialize() {
	// Set up standard logger flags
	log.SetFlags(0) // We'll handle formatting ourselves
	log.SetOutput(output)

	// Log startup message
	Info(CategoryApp, "Logger initialized")
}

// GetStandardLogWriter returns an io.Writer that can be used with standard Go logger
// Messages written to this writer will be logged at the specified level and category
func GetStandardLogWriter(level LogLevel, category Category) io.Writer {
	return &logWriter{level: level, category: category}
}

// logWriter implements io.Writer to integrate with standard loggers
type logWriter struct {
	level    LogLevel
	category Category
}

// Write implements io.Writer
func (w *logWriter) Write(p []byte) (n int, err error) {
	if shouldLog(w.level) {
		message := strings.TrimSpace(string(p))
		if !isSupressedALSALine(message) {
			log.Println(formatLog(w.level, w.category, message))
		}
	}
	return len(p), nil
}
