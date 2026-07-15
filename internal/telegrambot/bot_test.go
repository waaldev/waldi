package telegrambot

import (
	"strings"
	"testing"
)

func TestSplitMessageUnderLimit(t *testing.T) {
	text := "short message"
	chunks := splitMessage(text, telegramMessageLimit)
	if len(chunks) != 1 || chunks[0] != text {
		t.Fatalf("expected single unchanged chunk, got %v", chunks)
	}
}

func TestSplitMessageOverLimit(t *testing.T) {
	line := strings.Repeat("a", 100)
	var lines []string
	for range 60 {
		lines = append(lines, line)
	}
	text := strings.Join(lines, "\n")

	chunks := splitMessage(text, 500)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for _, c := range chunks {
		if len(c) > 500 {
			t.Fatalf("chunk exceeds limit: %d chars", len(c))
		}
	}
	if strings.Join(chunks, "\n") != text {
		t.Fatalf("chunks lost content when rejoined")
	}
}

func TestSplitMessageSingleLineOverLimit(t *testing.T) {
	text := strings.Repeat("b", 250)
	chunks := splitMessage(text, 100)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks for a 250-char line at limit 100, got %d", len(chunks))
	}
	if strings.Join(chunks, "") != text {
		t.Fatalf("chunks lost content when rejoined")
	}
}
