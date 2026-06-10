package checks

import (
	"testing"

	"codeberg.org/upPollo/parsec/internal/mdb"
	"codeberg.org/upPollo/parsec/internal/metadata"
)

func TestCheckTitle(t *testing.T) {
	tests := []struct {
		name      string
		metaTitle string
		resTitle  string
		wantWarn  bool
	}{
		{"Match", "Movie Title", "Movie Title", false},
		{"Mismatch", "Movie Title", "Different Title", true},
		{"Normalization Match", "Movie.Title", "Movie Title", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := &metadata.Metadata{Title: tt.metaTitle}
			res := &mdb.SearchResult{Title: tt.resTitle}
			results := checkTitle(meta, res)
			hasFailure := false

			for _, r := range results {
				if !r.Passed {
					hasFailure = true

					break
				}
			}

			if tt.wantWarn && !hasFailure {
				t.Errorf("checkTitle() expected warnings, got none")
			}

			if !tt.wantWarn && hasFailure {
				t.Errorf("checkTitle() expected no warnings, got failure")
			}
		})
	}
}

func TestCheckMovieYear(t *testing.T) {
	tests := []struct {
		name     string
		metaYear int
		resYear  int
		wantWarn bool
	}{
		{"Match", 2023, 2023, false},
		{"Mismatch", 2023, 2022, true},
		{"Meta Zero", 0, 2023, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := &metadata.Metadata{Year: tt.metaYear, IsTV: false}
			res := &mdb.SearchResult{Year: tt.resYear}
			results := checkMovieYear(meta, res)
			hasFailure := false

			for _, r := range results {
				if !r.Passed {
					hasFailure = true

					break
				}
			}

			if tt.wantWarn && !hasFailure {
				t.Errorf("checkMovieYear() expected warnings, got none")
			}

			if !tt.wantWarn && hasFailure {
				t.Errorf("checkMovieYear() expected no warnings, got failure")
			}
		})
	}
}
