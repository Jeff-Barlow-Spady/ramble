package transcription

import (
	"strings"
)

// TranscriptionAccumulator handles the aggregation of transcription text segments
// with intelligent overlap detection to avoid repetition
type TranscriptionAccumulator struct {
	currentText string
}

// NewAccumulator creates a new transcription accumulator
func NewAccumulator() *TranscriptionAccumulator {
	return &TranscriptionAccumulator{
		currentText: "",
	}
}

// GetText returns the current accumulated text
func (a *TranscriptionAccumulator) GetText() string {
	return a.currentText
}

// Reset clears the accumulated text
func (a *TranscriptionAccumulator) Reset() {
	a.currentText = ""
}

// AppendText intelligently adds new text to the current accumulated text,
// handling overlaps and repetition
func (a *TranscriptionAccumulator) AppendText(text string) {
	if text == "" {
		return
	}

	// For the first segment, simply set the text
	if a.currentText == "" {
		a.currentText = text
		return
	}

	// Check for significant overlap before appending
	// Convert to lowercase for better matching
	currentLower := strings.ToLower(a.currentText)
	newLower := strings.ToLower(text)

	// Split into words for overlap analysis
	currentWords := strings.Fields(currentLower)
	newWords := strings.Fields(newLower)

	// Skip if the new text is very short (likely noise)
	if len(newWords) < 3 {
		return
	}

	// Count matching words to detect overlap
	matches := 0
	wordMap := make(map[string]int)

	for _, word := range currentWords {
		wordMap[word]++
	}

	for _, word := range newWords {
		if count, exists := wordMap[word]; exists && count > 0 {
			matches++
			wordMap[word]--
		}
	}

	// Calculate overlap ratio
	overlapRatio := float64(0)
	if len(newWords) > 0 {
		overlapRatio = float64(matches) / float64(len(newWords))
	}

	// If substantial overlap (>80%), replace rather than append
	if overlapRatio > 0.8 {
		// When there's very high overlap, the newer text is likely more accurate
		// If the new text is significantly longer, it's probably more complete
		if len(text) > int(float64(len(a.currentText))*1.2) {
			// New text is at least 20% longer, likely more complete
			a.currentText = text
		} else if len(text) < int(float64(len(a.currentText))*0.8) {
			// New text is significantly shorter - keep existing text
			return
		} else {
			// Similar length - prefer the newer text as it might be more refined
			a.currentText = text
		}
	} else if overlapRatio > 0.4 {
		// Partial overlap - try to find where the new content starts
		// This is a simplified approach - find the last few words of current text
		// and see if they match the beginning of the new text

		// Take up to the last 5 words of current text for comparison
		endCount := 5
		if len(currentWords) < endCount {
			endCount = len(currentWords)
		}

		endPhrase := strings.Join(currentWords[len(currentWords)-endCount:], " ")

		// Check if new text starts with something similar to the end of current text
		// If so, only append the non-overlapping part
		if strings.HasPrefix(newLower, endPhrase) || strings.Contains(newLower, endPhrase) {
			// Find where to start appending (after the overlapping part)
			overlapIndex := strings.Index(newLower, endPhrase)
			if overlapIndex >= 0 {
				// Only append the new part (after the overlap)
				newPart := text[overlapIndex+len(endPhrase):]
				if len(strings.TrimSpace(newPart)) > 0 {
					a.currentText += " " + strings.TrimSpace(newPart)
				}
			} else {
				// Just append with space if we can't find a clean join point
				a.currentText += " " + text
			}
		} else {
			// Try another approach - look for individual overlapping segments
			// This handles cases where words are reordered slightly
			var lastMatchIndex int = -1

			// Find the last word from current text that appears in the new text
			for i := len(currentWords) - 1; i >= 0; i-- {
				word := currentWords[i]
				if index := strings.Index(newLower, word); index >= 0 {
					// We found a match
					lastMatchIndex = index + len(word)
					// Check if this word is at the start of the new text
					if index == 0 || newLower[index-1] == ' ' {
						// Found a word boundary match - only append after this point
						newPart := text[lastMatchIndex:]
						if len(strings.TrimSpace(newPart)) > 0 {
							a.currentText += " " + strings.TrimSpace(newPart)
							return
						}
					}
				}
			}

			// If we couldn't find a good junction point, just append
			a.currentText += " " + text
		}
	} else {
		// Low overlap, standard append
		a.currentText += " " + text
	}
}
