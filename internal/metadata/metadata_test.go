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
		{"es", "SPANISH"},
	}
	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			if got := LanguageName(tt.lang); got != tt.want {
				t.Errorf("LanguageName() = %v, want %v", got, tt.want)
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
			if got := ChanToNotation(tt.channels); got != tt.want {
				t.Errorf("ChanToNotation(%d) = %v, want %v", tt.channels, got, tt.want)
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
			if got := AudioCodecName(tt.codec); got != tt.want {
				t.Errorf("AudioCodecName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVideoCodecName(t *testing.T) {
	config.InitDefaults()
	tests := []struct {
		format  string
		version string
		hint    string
		want    string
	}{
		{"AVC", "", "", "H.264"},
		{"HEVC", "", "", "H.265"},
		{"AV1", "", "", "AV1"},
		{"MPEG Video", "Version 2", "", "MPEG2"},
		{"MPEG-4 Visual", "", "XviD", "XviD"},
		{"MPEG-4 Visual", "", "divx", "DivX"},
		{"MPEG-4 Visual", "", "", "MPEG4"},
	}
	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			if got := VideoCodecName(tt.format, tt.version, tt.hint); got != tt.want {
				t.Errorf("VideoCodecName(%s, %s, %s) = %v, want %v", tt.format, tt.version, tt.hint, got, tt.want)
			}
		})
	}
}

func TestHeightToResolution(t *testing.T) {
	tests := []struct {
		name      string
		height    int
		scanType  string
		frameRate float64
		want      string
	}{
		{"1080p", 1080, "Progressive", 23.976, "1080p"},
		{"1080i", 1080, "Interlaced", 25.000, "1080i"},
		{"1080mbaff", 1080, "MBAFF", 25.000, "1080i"},
		{"720p", 720, "Progressive", 50.000, "720p"},
		{"576p PAL", 576, "Progressive", 25.000, "576p"},
		{"576i PAL", 576, "Interlaced", 25.000, "576i"},
		{"480p NTSC", 480, "Progressive", 23.976, "480p"},
		{"Cropped PAL", 544, "Progressive", 25.000, "576p"},
		{"Cropped NTSC", 400, "Progressive", 23.976, "480p"},
		{"4K", 2160, "Progressive", 60.000, "2160p"},
		{"8K", 4320, "Progressive", 60.000, "4320p"},
		{"Invalid", 0, "", 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HeightToResolution(tt.height, tt.scanType, tt.frameRate); got != tt.want {
				t.Errorf("HeightToResolution(%d, %s, %f) = %v, want %v", tt.height, tt.scanType, tt.frameRate, got, tt.want)
			}
		})
	}
}

func TestMetadata_SetDefaults(t *testing.T) {
	config.InitDefaults()
	meta := &Metadata{}
	meta.SetDefaults()

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
				HDR:           "DV.HDR",
				AudioMeta:     "Atmos",
				BitDepth:      10,
				CutEdition:    "Unrated",
			},
			want: "Movie.2024.S01E02.Unrated.GERMAN.1080p.Netflix.WEB-DL.DDP5.1.Atmos.DV.HDR.H.265-GRP",
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
		{
			name: "Literal Brackets",
			meta: Metadata{
				Title:   "Movie",
				Service: "Netflix",
				Group:   "GRP",
			},
			want: "Movie.[Netflix]-GRP",
		},
		{
			name: "Anime Naming",
			meta: Metadata{
				Title:      "Sousou no Frieren",
				Episode:    1,
				Resolution: "1080p",
				Group:      "SubsPlease",
				CRC:        "F02B9CEE",
			},
			want: "[SubsPlease] Sousou no Frieren - 01 (1080p) [F02B9CEE]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.InitDefaults()
			if tt.name == "Literal Brackets" || tt.name == "Empty Literal Brackets Removal" {
				template := "{title}.[{service}]-{group}"
				if got := tt.meta.Render(template); got != tt.want {
					t.Errorf("Metadata.Render() = %v, want %v", got, tt.want)
				}
			} else if tt.name == "Anime Naming" {
				// Anime template: [Group] Title - Episode (Resolution) [CRC]
				template := "[{group}] {title} - {episode_02} ({resolution}) [{crc}]"
				if got := tt.meta.Render(template); got != tt.want {
					t.Errorf("Metadata.Render() = %v, want %v", got, tt.want)
				}
			} else {
				if got := tt.meta.String(); got != tt.want {
					t.Errorf("Metadata.String() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestMetadata_Override(t *testing.T) {
	meta := &Metadata{
		Title:  "Old",
		Year:   2000,
		Repack: true,
	}
	newMeta := &Metadata{
		Title:  "New",
		Repack: false,
	}

	updated := meta.Override(newMeta, true)
	if !updated {
		t.Errorf("Override() should return true when updated")
	}
	if meta.Title != "New" {
		t.Errorf("Override() Title = %v, want New", meta.Title)
	}
	if meta.Year != 2000 {
		t.Errorf("Override() Year should remain 2000, got %v", meta.Year)
	}

	if meta.Repack == false {
		t.Errorf("Override() bool flags should not get overridden by default")
	}

	updated = meta.Override(&Metadata{}, true)
	if updated {
		t.Errorf("Override() should return false when nothing changed")
	}
}
