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
			"[SIGH]", "[SIGHS]", // Additional tokens found in the test transcription
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

	// Remove sound effects in asterisks (like *Boom* *Boom*)
	asteriskPattern := regexp.MustCompile(`\*[^*]+\*`)
	text = asteriskPattern.ReplaceAllString(text, "")

	// Remove parenthetical content (noise markers and hesitations)
	parenPattern := regexp.MustCompile(`\([^)]*(?i)(music|noise|applause|laughter|sighs|sigh)[^)]*\)`)
	text = parenPattern.ReplaceAllString(text, "")

	// Remove bracketed noise markers - expanded pattern to include more token types
	bracketPattern := regexp.MustCompile(`\[(?i)(?:MUSIC|APPLAUSE|LAUGHTER|INAUDIBLE|NOISE|CROSSTALK|SILENCE|SPEAKING FOREIGN LANGUAGE|SPEAKING NON-ENGLISH|SIGH|SIGHS)\]`)
	text = bracketPattern.ReplaceAllString(text, "")

	// Remove repeated short phrases that commonly occur in real-time transcription
	// (words like "Hmm", "Uhh", etc. or repeated correction attempts)
	text = cleanRepeatedPhrases(text)

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

// cleanRepeatedPhrases removes common repetitive patterns in real-time transcription
func cleanRepeatedPhrases(text string) string {
	// Common filler words that tend to be repeated in transcription
	fillerWords := []string{"Hmm", "Um", "Uh", "Uhh", "Hmm", "Like", "So", "Yeah", "Boop"}

	// Remove consecutive repetitions of the same filler word
	for _, word := range fillerWords {
		pattern := regexp.MustCompile(`(?i)(?:\s*` + regexp.QuoteMeta(word) + `\s*,?\s*){2,}`)
		text = pattern.ReplaceAllString(text, " "+word+" ")
	}

	// Look for repeated short phrases (3-5 words) that are characteristic of transcription corrections
	// Split into sentences or fragments
	fragments := strings.Split(text, ".")
	var cleanedFragments []string

	for _, fragment := range fragments {
		fragment = strings.TrimSpace(fragment)
		if fragment == "" {
			continue
		}

		words := strings.Fields(fragment)
		if len(words) < 6 { // Skip very short fragments
			cleanedFragments = append(cleanedFragments, fragment)
			continue
		}

		// Build clean text without repetitions
		var cleanedText []string
		for i := 0; i < len(words); i++ {
			// Skip if this word starts a phrase that's repeated later
			skipCount := 0
			for phraseLen := 3; phraseLen <= 5 && i+phraseLen*2 <= len(words); phraseLen++ {
				// Check if the next phraseLen words are repeated
				matched := true
				for j := 0; j < phraseLen; j++ {
					if words[i+j] != words[i+phraseLen+j] {
						matched = false
						break
					}
				}

				if matched {
					skipCount = phraseLen
					break
				}
			}

			if skipCount > 0 {
				// Add the phrase once and skip ahead
				for j := 0; j < skipCount; j++ {
					cleanedText = append(cleanedText, words[i+j])
				}
				i += (skipCount * 2) - 1 // Skip both the original and the repetition
			} else {
				cleanedText = append(cleanedText, words[i])
			}
		}

		cleanedFragments = append(cleanedFragments, strings.Join(cleanedText, " "))
	}

	return strings.Join(cleanedFragments, ". ")
}
