package transcription

import (
	"strings"
	"testing"
)

func TestNewAccumulator(t *testing.T) {
	acc := NewAccumulator()
	if acc == nil {
		t.Fatal("Expected NewAccumulator to return a non-nil value")
	}
	if acc.GetText() != "" {
		t.Errorf("Expected new accumulator to have empty text, got %q", acc.GetText())
	}
}

func TestReset(t *testing.T) {
	acc := NewAccumulator()
	acc.AppendText("This is a test")
	if acc.GetText() == "" {
		t.Fatal("AppendText failed to set text")
	}

	acc.Reset()
	if acc.GetText() != "" {
		t.Errorf("Expected reset to clear text, got %q", acc.GetText())
	}
}

func TestAppendText_EmptyOrShort(t *testing.T) {
	acc := NewAccumulator()

	// Test empty string
	acc.AppendText("")
	if acc.GetText() != "" {
		t.Errorf("Expected empty input to be ignored, got %q", acc.GetText())
	}

	// Test first valid append
	acc.AppendText("First segment")
	if acc.GetText() != "First segment" {
		t.Errorf("Expected text to be set to first segment, got %q", acc.GetText())
	}

	// Test short input (less than 3 words)
	acc.Reset()
	acc.AppendText("First segment")
	acc.AppendText("Too short")
	if acc.GetText() != "First segment" {
		t.Errorf("Expected short input to be ignored, got %q", acc.GetText())
	}
}

func TestAppendText_HighOverlap(t *testing.T) {
	// Test case where new text is mostly overlapping (>80%)
	acc := NewAccumulator()
	acc.AppendText("The quick brown fox jumps over the lazy dog")
	acc.AppendText("The quick brown fox jumps over the lazy dogs")

	// Should replace with the newer, more complete version
	expected := "The quick brown fox jumps over the lazy dogs"
	if acc.GetText() != expected {
		t.Errorf("With high overlap, expected text to be replaced with %q, got %q", expected, acc.GetText())
	}
}

func TestAppendText_MediumOverlap(t *testing.T) {
	// Test case where new text has partial overlap (40-80%)
	acc := NewAccumulator()
	acc.AppendText("The quick brown fox jumps over")
	acc.AppendText("jumps over the lazy dog")

	// Should intelligently append only the non-overlapping part
	expected := "The quick brown fox jumps over the lazy dog"
	if acc.GetText() != expected {
		t.Errorf("With medium overlap, expected smart joining to %q, got %q", expected, acc.GetText())
	}
}

func TestAppendText_LowOverlap(t *testing.T) {
	// Test case where new text has low overlap (<40%)
	acc := NewAccumulator()
	acc.AppendText("The quick brown fox")
	acc.AppendText("jumps over the lazy dog")

	// Should just append with a space
	expected := "The quick brown fox jumps over the lazy dog"
	if acc.GetText() != expected {
		t.Errorf("With low overlap, expected simple append to %q, got %q", expected, acc.GetText())
	}
}

func TestRealWorldScenario(t *testing.T) {
	// Test with real-world transcription fragments that have overlaps
	acc := NewAccumulator()

	// First interim transcription
	acc.AppendText("Was originally offered to work just in the case of")

	// Second interim with repetition
	acc.AppendText("Was originally offered to work just in the case of Richard a double notebooks")

	// Third interim building on previous
	acc.AppendText("Originally offered to work just in the case of Richard and O'Bolm notebooks, but actually can work it")

	// Fourth interim
	acc.AppendText("To work just in the case of Richard and O'Bolm notebooks, but actually to work it generically across the board and a chat system that was built for")

	// Final more complete segment
	acc.AppendText("Which I did on the moment, no books, but actually to work in generic way across the board. And a chat system that was built for one application, but to really be repurposed")

	// Print the result for debugging
	t.Logf("Final text: %q", acc.GetText())

	// Verify we don't have excessive duplication of phrases
	text := acc.GetText()
	duplicatedPhrases := 0

	// Check for phrases that shouldn't be duplicated
	phrasesToCheck := []string{
		"Originally offered",
		"work just in the case",
		"notebooks",
		"across the board",
	}

	for _, phrase := range phrasesToCheck {
		count := 0
		// Use a simple approach to count non-overlapping occurrences
		// This is more accurate than using strings.Index in a loop
		fragments := strings.Split(strings.ToLower(text), strings.ToLower(phrase))
		count = len(fragments) - 1

		if count > 1 {
			t.Logf("Phrase %q appears %d times", phrase, count)
			duplicatedPhrases++
		}
	}

	// Allow at most one duplicated phrase type, as some duplication might be legitimate
	if duplicatedPhrases > 1 {
		t.Errorf("Found too many repeated phrases in output: %q", acc.GetText())
	}
}
