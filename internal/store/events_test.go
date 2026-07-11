package store

import "testing"

func TestImpressionSourceValid(t *testing.T) {
	tests := map[ImpressionSource]bool{
		ImpressionDirect:   true,
		ImpressionFeed:     true,
		ImpressionWildcard: true,
		"other":            false,
	}

	for source, want := range tests {
		if got := source.Valid(); got != want {
			t.Fatalf("%q valid = %v, want %v", source, got, want)
		}
	}
}
