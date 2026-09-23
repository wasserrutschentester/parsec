package mdb_test

import (
	"testing"

	"codeberg.org/upPollo/parsec/internal/mdb"
)

func TestTitleTypeIsExcludedFromSearch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		titleType mdb.TitleType
		excluded  bool
	}{
		{mdb.TitleTypeMovie, false},
		{mdb.TitleTypeTVSeries, false},
		{mdb.TitleTypeTVMiniSeries, false},
		{mdb.TitleTypeTVSpecial, false},
		{mdb.TitleTypeTVMovie, false},
		{mdb.TitleTypeTVShort, false},
		{mdb.TitleTypeShort, false},
		{mdb.TitleTypeVideo, false},
		{mdb.TitleTypeUnknown, false},
		{mdb.TitleTypePodcastSeries, true},
		{mdb.TitleTypePodcastEpisode, true},
		{mdb.TitleTypeMusicVideo, true},
		{mdb.TitleTypeVideoGame, true},
	}

	for _, tt := range tests {
		got := tt.titleType.IsExcludedFromSearch()
		if got != tt.excluded {
			t.Errorf("TitleType(%q).IsExcludedFromSearch() = %v, want %v", tt.titleType, got, tt.excluded)
		}
	}
}

func TestTitleTypeIsSpecific(t *testing.T) {
	t.Parallel()

	tests := []struct {
		titleType mdb.TitleType
		specific  bool
	}{
		{mdb.TitleTypeMovie, false},
		{mdb.TitleTypeTVSeries, false},
		{mdb.TitleTypeUnknown, false},
		{mdb.TitleTypePodcastSeries, false},
		{mdb.TitleTypePodcastEpisode, false},
		{mdb.TitleTypeMusicVideo, false},
		{mdb.TitleTypeVideoGame, false},
		{mdb.TitleTypeTVMiniSeries, true},
		{mdb.TitleTypeTVSpecial, true},
		{mdb.TitleTypeTVMovie, true},
		{mdb.TitleTypeTVShort, true},
		{mdb.TitleTypeShort, true},
		{mdb.TitleTypeVideo, true},
	}

	for _, tt := range tests {
		got := tt.titleType.IsSpecific()
		if got != tt.specific {
			t.Errorf("TitleType(%q).IsSpecific() = %v, want %v", tt.titleType, got, tt.specific)
		}
	}
}

func TestFormatTitleType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		titleType mdb.TitleType
		want      string
	}{
		{mdb.TitleTypeMovie, "Movie"},
		{mdb.TitleTypeTVSeries, "TV Series"},
		{mdb.TitleTypeTVMiniSeries, "Mini-Series"},
		{mdb.TitleTypeTVSpecial, "TV Special"},
		{mdb.TitleTypeTVMovie, "TV Movie"},
		{mdb.TitleTypeTVShort, "TV Short"},
		{mdb.TitleTypeShort, "Short Film"},
		{mdb.TitleTypeVideo, "Direct-to-Video"},
		{mdb.TitleTypeVideoGame, "Video Game"},
		{mdb.TitleTypePodcastSeries, "Podcast Series"},
		{mdb.TitleTypePodcastEpisode, "Podcast Episode"},
		{mdb.TitleTypeMusicVideo, "Music Video"},
		{mdb.TitleType("custom"), "custom"},
	}

	for _, tt := range tests {
		got := mdb.FormatTitleType(tt.titleType)
		if got != tt.want {
			t.Errorf("FormatTitleType(%q) = %q, want %q", tt.titleType, got, tt.want)
		}
	}
}

func TestShouldDisplayTitleType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		titleType mdb.TitleType
		isTV      bool
		want      bool
	}{
		{"unknown for movie", mdb.TitleTypeUnknown, false, false},
		{"unknown for TV", mdb.TitleTypeUnknown, true, false},
		{"standard movie", mdb.TitleTypeMovie, false, false},
		{"standard TV series", mdb.TitleTypeTVSeries, true, false},
		{"contradiction: movie classified for TV file", mdb.TitleTypeMovie, true, true},
		{"contradiction: TV series for movie file", mdb.TitleTypeTVSeries, false, true},
		{"non-standard: miniseries for TV", mdb.TitleTypeTVMiniSeries, true, true},
		{"non-standard: TV movie for movie file", mdb.TitleTypeTVMovie, false, true},
		{"non-standard: TV movie for TV file", mdb.TitleTypeTVMovie, true, true},
		{"non-standard: TV special for TV file", mdb.TitleTypeTVSpecial, true, true},
		{"non-standard: short film for movie file", mdb.TitleTypeShort, false, true},
		{"non-standard: video for movie file", mdb.TitleTypeVideo, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := mdb.ShouldDisplayTitleType(tt.titleType, tt.isTV)
			if got != tt.want {
				t.Errorf("ShouldDisplayTitleType(%q, %v) = %v, want %v", tt.titleType, tt.isTV, got, tt.want)
			}
		})
	}
}
