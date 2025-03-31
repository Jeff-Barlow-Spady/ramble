package transcription

import (
	"regexp"
	"strings"
)

// NormalizeTranscriptionText cleans up transcription text for better quality
func NormalizeTranscriptionText(text string) string {
	if text == "" {
		return ""
	}

	// Handle special tokens - note that we will detect and filter these at the streaming callback level
	if strings.HasPrefix(text, "[") && strings.HasSuffix(text, "]") {
		// Special token detection for logging
		specialTokensToFilter := []string{
			"[MUSIC]", "[MUSIC PLAYING]", "[APPLAUSE]", "[LAUGHTER]",
			"[NOISE]", "[SILENCE]", "[BLANK_AUDIO]", "[INAUDIBLE]",
			"[CROSSTALK]", "[SPEAKING FOREIGN LANGUAGE]", "[SPEAKING NON-ENGLISH]",
		}

		// Check if this is a special token we should filter
		for _, token := range specialTokensToFilter {
			if strings.Contains(strings.ToUpper(text), strings.ToUpper(token)) {
				return "" // Return empty string to skip this token completely
			}
		}

		// For other special tokens not in our filter list, return as-is
		return text
	}

	// Trim and normalize whitespace
	text = strings.TrimSpace(text)

	// Remove parenthetical content (noise markers)
	parenPattern := regexp.MustCompile(`\([^)]*(?i)(music|noise|applause|laughter)[^)]*\)`)
	text = parenPattern.ReplaceAllString(text, "")

	// Remove bracketed noise markers - expanded pattern to include more token types
	bracketPattern := regexp.MustCompile(`\[(?i)(?:MUSIC|APPLAUSE|LAUGHTER|INAUDIBLE|NOISE|CROSSTALK|SILENCE|SPEAKING FOREIGN LANGUAGE|SPEAKING NON-ENGLISH)\]`)
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
