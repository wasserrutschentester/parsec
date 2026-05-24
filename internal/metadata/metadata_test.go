package metadata

import (
	"testing"

	"codeberg.org/n0ne/parsec/internal/config"
)

func TestLanguageName(t *testing.T) {
	tests := []struct {
		lang string
		want string
	}{
		{"de", "GERMAN"},
		{"en", "ENGLISH"},
		{"fr", "FRENCH"},
		{"es", "es"},
	}
	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			if got := languageName(tt.lang); got != tt.want {
				t.Errorf("languageName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestChanToNotation(t *testing.T) {
	tests := []struct {
		channels int
		want     string
	}{
		{1, "1.0"},
		{2, "2.0"},
		{5, "5.0"},
		{6, "5.1"},
		{7, "6.1"},
		{8, "7.1"},
		{3, "3"},
	}
	for _, tt := range tests {
		t.Run(string(rune(tt.channels)), func(t *testing.T) {
			if got := chanToNotation(tt.channels); got != tt.want {
				t.Errorf("chanToNotation(%d) = %v, want %v", tt.channels, got, tt.want)
			}
		})
	}
}

func TestAudioCodecName(t *testing.T) {
	tests := []struct {
		codec string
		want  string
	}{
		{"AAC", "AAC"},
		{"AC-3", "DD"},
		{"E-AC-3", "DDP"},
		{"DTS", "DTS"},
	}
	for _, tt := range tests {
		t.Run(tt.codec, func(t *testing.T) {
			if got := audioCodecName(tt.codec); got != tt.want {
				t.Errorf("audioCodecName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVideoCodecName(t *testing.T) {
	tests := []struct {
		codec string
		want  string
	}{
		{"AVC", "H.264"},
		{"HEVC", "H.265"},
		{"AV1", "AV1"},
	}
	for _, tt := range tests {
		t.Run(tt.codec, func(t *testing.T) {
			if got := videoCodecName(tt.codec); got != tt.want {
				t.Errorf("videoCodecName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHeightToResolution(t *testing.T) {
	tests := []struct {
		height int
		want   string
	}{
		{1080, "1080p"},
		{720, "720p"},
		{0, ""},
		{-1, ""},
	}
	for _, tt := range tests {
		t.Run(string(rune(tt.height)), func(t *testing.T) {
			if got := heightToResolution(tt.height); got != tt.want {
				t.Errorf("heightToResolution(%d) = %v, want %v", tt.height, got, tt.want)
			}
		})
	}
}

func TestMetadata_SetDefaults(t *testing.T) {
	config.InitDefaults()
	meta := &Metadata{}
	meta.SetDefaults()

	if meta.Title != "Missing.Title" {
		t.Errorf("SetDefaults() Title = %v, want Missing.Title", meta.Title)
	}
	if meta.Source != config.GetSource() {
		t.Errorf("SetDefaults() Source = %v, want %v", meta.Source, config.GetSource())
	}
	if meta.Group != config.GetGroup() {
		t.Errorf("SetDefaults() Group = %v, want %v", meta.Group, config.GetGroup())
	}
}

func TestMetadata_String(t *testing.T) {
	tests := []struct {
		name string
		meta Metadata
		want string
	}{
		{
			name: "Full metadata",
			meta: Metadata{
				Title:         "Movie",
				Year:          2024,
				Season:        1,
				Episode:       2,
				Language:      "de",
				Resolution:    "1080p",
				Service:       "Netflix",
				Source:        "WEB-DL",
				AudioCodec:    "DDP",
				AudioChannels: "5.1",
				VideoCodec:    "H.265",
				Group:         "GRP",
			},
			want: "Movie.2024.S01E02.GERMAN.1080p.Netflix.WEB-DL.DDP5.1.H.265-GRP",
		},
		{
			name: "Minimal metadata",
			meta: Metadata{
				Title: "Movie",
				Group: "GRP",
			},
			want: "Movie-GRP",
		},
		{
			name: "With Date and Episode Title",
			meta: Metadata{
				Title:        "Show",
				Date:         "2024-05-24",
				EpisodeTitle: "Title",
				Group:        "GRP",
			},
			want: "Show.2024-05-24.Title-GRP",
		},
		{
			name: "Repack",
			meta: Metadata{
				Title:  "Movie",
				Repack: true,
				Group:  "GRP",
			},
			want: "Movie.REPACK-GRP",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.meta.String(); got != tt.want {
				t.Errorf("Metadata.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMetadata_Override(t *testing.T) {
	meta := &Metadata{
		Title: "Old",
		Year:  2000,
	}
	newMeta := &Metadata{
		Title: "New",
	}

	updated := meta.Override(newMeta)
	if !updated {
		t.Errorf("Override() should return true when updated")
	}
	if meta.Title != "New" {
		t.Errorf("Override() Title = %v, want New", meta.Title)
	}
	if meta.Year != 2000 {
		t.Errorf("Override() Year should remain 2000, got %v", meta.Year)
	}

	updated = meta.Override(&Metadata{})
	if updated {
		t.Errorf("Override() should return false when nothing changed")
	}
}
