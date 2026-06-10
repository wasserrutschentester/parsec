package search

import (
	"testing"

	"codeberg.org/upPollo/parsec/internal/mdb"
)

func TestCalculateSimilarity(t *testing.T) {
	tests := []struct {
		s1   string
		s2   string
		want float64
	}{
		{"abc", "abc", 1.0},
		{"abc", "abd", 2.0 / 3.0},
		{"abc", "", 0.0},
		{"", "abc", 0.0},
		{"kitten", "sitting", 1.0 - (3.0 / 7.0)},
		{"flaw", "lawn", 0.5},
	}

	for _, tt := range tests {
		got := mdb.CalculateSimilarity(tt.s1, tt.s2)
		if (got-tt.want) > 0.001 || (tt.want-got) > 0.001 {
			t.Errorf("CalculateSimilarity(%q, %q) = %v, want %v", tt.s1, tt.s2, got, tt.want)
		}
	}
}

func TestAddUniqueAltTitle(t *testing.T) {
	tests := []struct {
		titles         []string
		newTitle       string
		existingTitles []string
		want           []string
	}{
		{[]string{"A"}, "B", []string{"C"}, []string{"A", "B"}},
		{[]string{"A"}, "A", []string{"B"}, []string{"A"}},
		{[]string{"A"}, "B", []string{"B"}, []string{"A"}},
		{[]string{"A"}, "", []string{"B"}, []string{"A"}},
		{nil, "A", nil, []string{"A"}},
	}

	for _, tt := range tests {
		got := addUniqueAltTitle(tt.titles, tt.newTitle, tt.existingTitles...)
		if len(got) != len(tt.want) {
			t.Errorf("addUniqueAltTitle(%v, %q) len = %d, want %d", tt.titles, tt.newTitle, len(got), len(tt.want))
			continue
		}

		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("addUniqueAltTitle(%v, %q) = %v, want %v", tt.titles, tt.newTitle, got, tt.want)
				break
			}
		}
	}
}

// nolint:funlen,cyclop
func TestMergeResults(t *testing.T) {
	tmdbResults := []mdb.SearchResult{
		{
			TmdbID: 1,
			Title:  "Movie 1",
			ImdbID: "tt1",
		},
		{
			TmdbID: 2,
			Title:  "Movie 2",
		},
	}

	tvdbResults := []mdb.SearchResult{
		{
			TvdbID: 101,
			TmdbID: 1,
			Title:  "Movie One",
		},
		{
			TvdbID: 102,
			Title:  "Movie 3",
			ImdbID: "tt3",
		},
	}

	merged := mergeResults(tmdbResults, tvdbResults)

	if len(merged) != 3 {
		t.Errorf("Expected 3 merged results, got %d", len(merged))
	}

	// Check if Movie 1 was merged correctly
	foundMovie1 := false

	for _, r := range merged {
		if r.TmdbID == 1 {
			foundMovie1 = true

			if r.TvdbID != 101 {
				t.Errorf("Movie 1: expected TvdbID 101, got %d", r.TvdbID)
			}

			foundAlt := false

			for _, alt := range r.AltTitle {
				if alt == "Movie One" {
					foundAlt = true
					break
				}
			}

			if !foundAlt {
				t.Errorf("Movie 1: expected 'Movie One' in AltTitles")
			}
		}
	}

	if !foundMovie1 {
		t.Errorf("Movie 1 not found in merged results")
	}

	// Check if Movie 3 was added
	foundMovie3 := false

	for _, r := range merged {
		if r.TvdbID == 102 {
			foundMovie3 = true
		}
	}

	if !foundMovie3 {
		t.Errorf("Movie 3 not found in merged results")
	}
}
