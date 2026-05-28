package checks

import (
	"testing"

	"codeberg.org/n0ne/parsec/internal/metadata"
)

func TestCheckYear(t *testing.T) {
	tests := []struct {
		name     string
		year     int
		season   int
		isTV     bool
		wantWarn bool
	}{
		{"Movie with year", 2023, 0, false, false},
		{"Movie without year", 0, 0, false, true},
		{"TV Show", 0, 1, true, false},
		{"Redundant Year", 2023, 2023, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := &metadata.Metadata{Year: tt.year, Season: tt.season, IsTV: tt.isTV}
			results := CheckYear(meta)
			hasFailure := false
			for _, res := range results {
				if !res.Passed {
					hasFailure = true
					break
				}
			}
			if tt.wantWarn && !hasFailure {
				t.Errorf("CheckYear() expected error, got none")
			}
			if !tt.wantWarn && hasFailure {
				t.Errorf("CheckYear() expected no error, got failure")
			}
		})
	}
}

func TestCheckStreaming(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		service  string
		wantWarn bool
	}{
		{"WEB with service", "WEB-DL", "AMZN", false},
		{"WEB missing service", "WEB-DL", "", true},
		{"BluRay with service", "BluRay", "AMZN", true},
		{"BluRay without service", "BluRay", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := &metadata.Metadata{Source: tt.source, Service: tt.service}
			results := CheckStreaming(meta)
			hasFailure := false
			for _, res := range results {
				if !res.Passed {
					hasFailure = true
					break
				}
			}
			if tt.wantWarn && !hasFailure {
				t.Errorf("CheckStreaming() expected error, got none")
			}
			if !tt.wantWarn && hasFailure {
				t.Errorf("CheckStreaming() expected no error, got failure")
			}
		})
	}
}

func TestCheckTvSpecial(t *testing.T) {
	tests := []struct {
		name         string
		isTV         bool
		season       int
		date         string
		episodeTitle string
		wantWarn     bool
	}{
		{"Regular TV", true, 1, "", "", false},
		{"Special with date/title", true, 0, "2023-01-01", "New Year Special", false},
		{"Special missing date", true, 0, "", "New Year Special", true},
		{"Special missing title", true, 0, "2023-01-01", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := &metadata.Metadata{IsTV: tt.isTV, Season: tt.season, Date: tt.date, EpisodeTitle: tt.episodeTitle}
			results := CheckTvSpecial(meta)
			hasFailure := false
			for _, res := range results {
				if !res.Passed {
					hasFailure = true
					break
				}
			}
			if tt.wantWarn && !hasFailure {
				t.Errorf("CheckTvSpecial() expected error, got none")
			}
			if !tt.wantWarn && hasFailure {
				t.Errorf("CheckTvSpecial() expected no error, got failure")
			}
		})
	}
}
