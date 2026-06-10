package checks

import (
	"testing"
)

func TestNormalizeForComparison(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"Movie.Title.2023", "movie title 2023"},
		{"Movie-Title-2023", "movie title 2023"},
		{"Movie Title (2023)", "movie title 2023"},
		{"Movie   Title", "movie title"},
	}

	for _, tt := range tests {
		got := normalizeForComparison(tt.input)
		if got != tt.want {
			t.Errorf("normalizeForComparison(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
