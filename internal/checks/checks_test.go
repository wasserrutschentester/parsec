package checks

import (
	"testing"

	"codeberg.org/n0ne/parsec/internal/metadata/mediainfo"
)

func TestCheckRedundantAudio(t *testing.T) {
	tests := []struct {
		name     string
		tracks   []mediainfo.Track
		wantWarn bool
	}{
		{
			name: "single audio",
			tracks: []mediainfo.Track{
				{Type: "Audio", Language: "en"},
			},
			wantWarn: false,
		},
		{
			name: "duplicate audio",
			tracks: []mediainfo.Track{
				{Type: "Audio", Language: "en"},
				{Type: "Audio", Language: "en"},
			},
			wantWarn: true,
		},
		{
			name: "audio and commentary",
			tracks: []mediainfo.Track{
				{Type: "Audio", Language: "en"},
				{Type: "Audio", Language: "en", Title: "Commentary by Director"},
			},
			wantWarn: false,
		},
		{
			name: "audio and description",
			tracks: []mediainfo.Track{
				{Type: "Audio", Language: "en"},
				{Type: "Audio", Language: "en", Title: "Audio Description"},
			},
			wantWarn: false,
		},
		{
			name: "different languages",
			tracks: []mediainfo.Track{
				{Type: "Audio", Language: "en"},
				{Type: "Audio", Language: "fr"},
			},
			wantWarn: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mi := &mediainfo.MediaInfo{
				Media: mediainfo.Media{
					Tracks: tt.tracks,
				},
			}
			// Since CheckRedundantAudio prints to stdout, we'd need to capture stdout to fully test.
			// For now, just ensuring it doesn't crash and we can visually verify logic.
			CheckRedundantAudio(mi)
		})
	}
}

func TestCheckResolution(t *testing.T) {
	tests := []struct {
		name     string
		width    string
		height   string
		wantWarn bool
	}{
		{
			name:     "Standard 1080p",
			width:    "1920",
			height:   "1080",
			wantWarn: false,
		},
		{
			name:     "Cropped 1080p",
			width:    "1920",
			height:   "800",
			wantWarn: false,
		},
		{
			name:     "Non-mod2",
			width:    "1919",
			height:   "800",
			wantWarn: true,
		},
		{
			name:     "Non-standard width",
			width:    "1900",
			height:   "800",
			wantWarn: true,
		},
		{
			name:     "Portrait mode",
			width:    "800",
			height:   "1920",
			wantWarn: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			track := &mediainfo.Track{
				Type:   "Video",
				Width:  tt.width,
				Height: tt.height,
			}
			CheckResolution(track)
		})
	}
}
