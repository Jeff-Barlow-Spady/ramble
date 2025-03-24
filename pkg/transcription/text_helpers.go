package transcription

import (
	"regexp"
	"strings"
)

// normalizeTranscriptionText cleans up transcription text for better quality
func normalizeTranscriptionText(text string) string {
	if text == "" {
		return ""
	}

	// Handle special tokens
	if strings.HasPrefix(text, "[") && strings.HasSuffix(text, "]") {
		// Return special tokens as-is
		return text
	}

	// Trim and normalize whitespace
	text = strings.TrimSpace(text)

	// Remove parenthetical content (noise markers)
	parenPattern := regexp.MustCompile(`\([^)]*(?i)(music|noise|applause|laughter)[^)]*\)`)
	text = parenPattern.ReplaceAllString(text, "")

	// Remove bracketed noise markers
	bracketPattern := regexp.MustCompile(`\[(?i)(?:MUSIC|APPLAUSE|LAUGHTER|INAUDIBLE|NOISE|CROSSTALK|SILENCE)\]`)
	text = bracketPattern.ReplaceAllString(text, "")

	// Normalize spaces
	spacePattern := regexp.MustCompile(`\s+`)
	text = spacePattern.ReplaceAllString(text, " ")

	// Fix punctuation
	text = strings.ReplaceAll(text, " .", ".")
	text = strings.ReplaceAll(text, " ,", ",")
	text = strings.ReplaceAll(text, " ?", "?")
	text = strings.ReplaceAll(text, " !", "!")

	// Capitalize first letter
	if len(text) > 0 {
		text = strings.ToUpper(text[:1]) + text[1:]
	}

	return strings.TrimSpace(text)
}
