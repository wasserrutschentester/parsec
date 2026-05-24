package metadata

import (
	"reflect"
	"testing"

	"codeberg.org/n0ne/parsec/internal/config"
)

func TestMediaInfo_GetAudioLanguages(t *testing.T) {
	mi := &MediaInfo{
		Media: Media{
			Tracks: []Track{
				{Type: "General"},
				{Type: "Video"},
				{Type: "Audio", Language: "de"},
				{Type: "Audio", Language: "en"},
				{Type: "Audio", Language: "de"}, // Duplicate
			},
		},
	}

	want := []string{"de", "en"}
	got := mi.GetAudioLanguages()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetAudioLanguages() = %v, want %v", got, want)
	}
}

func TestMediaInfo_GetSubtitleLanguages(t *testing.T) {
	mi := &MediaInfo{
		Media: Media{
			Tracks: []Track{
				{Type: "Text", Language: "de"},
				{Type: "Text", Language: "en"},
			},
		},
	}

	want := []string{"de", "en"}
	got := mi.GetSubtitleLanguages()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetSubtitleLanguages() = %v, want %v", got, want)
	}
}

func TestMediaInfo_GetLanguageTag(t *testing.T) {
	config.InitDefaults() // preferred_language = "de"

	tests := []struct {
		name   string
		tracks []Track
		want   string
	}{
		{
			name: "Single language (German)",
			tracks: []Track{
				{Type: "Audio", Language: "de"},
			},
			want: "GERMAN",
		},
		{
			name: "Dual language (German/English)",
			tracks: []Track{
				{Type: "Audio", Language: "de"},
				{Type: "Audio", Language: "de"},
				{Type: "Audio", Language: "en"},
			},
			want: "GERMAN.DL",
		},
		{
			name: "Multi language (3+)",
			tracks: []Track{
				{Type: "Audio", Language: "de"},
				{Type: "Audio", Language: "en"},
				{Type: "Audio", Language: "fr"},
			},
			want: "GERMAN.ML",
		},
		{
			name: "Subbed (English audio, German subs)",
			tracks: []Track{
				{Type: "Audio", Language: "en"},
				{Type: "Text", Language: "de"},
			},
			want: "GERMAN.SUBBED",
		},
		{
			name: "Not subbed (English audio, French subs, preferred is de)",
			tracks: []Track{
				{Type: "Audio", Language: "en"},
				{Type: "Text", Language: "fr"},
			},
			want: "ENGLISH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mi := &MediaInfo{
				Media: Media{
					Tracks: tt.tracks,
				},
			}
			if got := mi.GetLanguageTag(); got != tt.want {
				t.Errorf("GetLanguageTag() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMediaInfo_GetMediaMetadata(t *testing.T) {
	mi := &MediaInfo{
		Media: Media{
			Tracks: []Track{
				{
					Type:   "Video",
					Height: "1080",
					Format: "AVC",
				},
				{
					Type:     "Audio",
					Format:   "E-AC-3",
					Channels: "6",
					Language: "de",
				},
			},
		},
	}

	got := mi.GetMediaMetadata()
	want := &Metadata{
		Resolution:    "1080p",
		VideoCodec:    "H.264",
		AudioCodec:    "DDP",
		AudioChannels: "5.1",
		Language:      "GERMAN",
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetMediaMetadata() mismatch")
		// Use a better diff if available, but for now this is fine
	}
}
