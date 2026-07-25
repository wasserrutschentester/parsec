package checks

import (
	"strings"
	"testing"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/mdb"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/mediainfo"
)

func audioMediaInfo(langs ...string) *mediainfo.MediaInfo {
	tracks := make([]mediainfo.Track, 0, len(langs))
	for _, lang := range langs {
		tracks = append(tracks, mediainfo.Track{Type: "Audio", Language: lang})
	}

	return &mediainfo.MediaInfo{Media: mediainfo.Media{Tracks: tracks}}
}

//nolint:paralleltest // depends on shared global config state
func TestCheckUnwantedAudioLang(t *testing.T) {
	config.InitDefaults() // preferred language is "de"

	tests := []struct {
		name         string
		audioLangs   []string
		originalLang string
		wantUnwanted bool
	}{
		{"only preferred", []string{"ger"}, "ja", false},
		{"preferred and original", []string{"ger", "jpn"}, "ja", false},
		{"foreign track flagged", []string{"ger", "jpn", "fre"}, "ja", true},
		{"zxx is never unwanted", []string{"ger", "zxx"}, "ja", false},
		{"und and mul kept", []string{"ger", "und", "mul"}, "ja", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mi := audioMediaInfo(tt.audioLangs...)
			result := &mdb.SearchResult{OriginalLanguage: tt.originalLang}

			results := checkUnwantedAudioLang(mi, result)
			if got := len(results) > 0; got != tt.wantUnwanted {
				t.Fatalf("checkUnwantedAudioLang() unwanted=%v, want %v (results=%+v)", got, tt.wantUnwanted, results)
			}
		})
	}
}

//nolint:paralleltest // depends on shared global config state
func TestCheckUnwantedAudioLangKeepsZxxAlongsideForeign(t *testing.T) {
	config.InitDefaults() // preferred language is "de"

	mi := audioMediaInfo("ger", "fre", "zxx")
	results := checkUnwantedAudioLang(mi, &mdb.SearchResult{OriginalLanguage: "ja"})

	if len(results) != 1 {
		t.Fatalf("expected one unwanted-language result, got %+v", results)
	}

	// The French track is unwanted, but the no-dialogue 'zxx' track must not be.
	if strings.Contains(results[0].Warning, "zxx") {
		t.Errorf("zxx must not be reported as unwanted: %q", results[0].Warning)
	}
}

//nolint:paralleltest // depends on shared global state
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

//nolint:paralleltest // depends on shared global state
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
