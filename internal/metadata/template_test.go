package metadata

import (
	"testing"

	"github.com/spf13/viper"

	"codeberg.org/upPollo/parsec/internal/config"
)

//nolint:funlen,paralleltest // comprehensive template tests mutating global viper word_separator
func TestMetadata_Render_GoTemplates(t *testing.T) {
	config.InitDefaults()

	tests := []struct {
		name     string
		meta     Metadata
		template string
		sep      string
		want     string
	}{
		{
			name: "Modern template with join and conditions (anime space separated)",
			meta: Metadata{
				Title:       "Frieren",
				Year:        2023,
				Season:      1,
				Episodes:    []int{1, 2, 3, 4},
				Resolution:  "1080p",
				Source:      "CR",
				VideoCodec:  "x264",
				AudioCodec:  "AAC",
				IsDualAudio: true,
				Group:       "SubsPlease",
				IsTV:        true,
			},
			template: `[{{.Group}}] {{.Title}} - {{.SeasonEpisode}} [{{.Resolution}} {{.Source}} {{.VideoCodec}} {{.AudioCodec}}]{{if .IsDualAudio}} [Dual-Audio]{{end}}`,
			sep:      " ",
			want:     "[SubsPlease] Frieren - S01E01-E04 [1080p CR x264 AAC] [Dual-Audio]",
		},
		{
			name: "AKA function with original title",
			meta: Metadata{
				Title:         "Spirited Away",
				OriginalTitle: "Sen to Chihiro no Kamikakushi",
				Year:          2001,
				Resolution:    "1080p",
				Group:         "GRP",
			},
			template: `{{aka .OriginalTitle .Title}}.{{.Resolution}}-{{.Group}}`,
			sep:      ".",
			want:     "Sen.to.Chihiro.no.Kamikakushi.AKA.Spirited.Away.1080p-GRP",
		},
		{
			name: "AKA function with original title and year",
			meta: Metadata{
				Title:         "Parasite",
				OriginalTitle: "Gisaengchung",
				Year:          2019,
				Resolution:    "1080p",
				Group:         "GRP",
			},
			template: `{{aka .OriginalTitle .Title .YearTag}}.{{.Resolution}}-{{.Group}}`,
			sep:      ".",
			want:     "Gisaengchung.2019.AKA.Parasite.1080p-GRP",
		},
		{
			name: "AKA function without original title",
			meta: Metadata{
				Title:      "Inception",
				Year:       2010,
				Resolution: "1080p",
				Group:      "GRP",
			},
			template: `{{aka .OriginalTitle .Title}}.{{.Resolution}}-{{.Group}}`,
			sep:      ".",
			want:     "Inception.1080p-GRP",
		},
		{
			name: "Repack level handling",
			meta: Metadata{
				Title:       "Movie",
				Year:        2024,
				RepackLevel: 2,
				Group:       "GRP",
			},
			template: `{{.Title}}.{{.Year}}.{{.RepackTag}}-{{.Group}}`,
			sep:      ".",
			want:     "Movie.2024.REPACK2-GRP",
		},
		{
			name: "Release version tag",
			meta: Metadata{
				Title:          "Movie",
				Year:           2024,
				ReleaseVersion: 3,
				Group:          "GRP",
			},
			template: `{{.Title}}.{{.Year}}.{{.VersionTag}}-{{.Group}}`,
			sep:      ".",
			want:     "Movie.2024.v3-GRP",
		},
		{
			name: "When helper",
			meta: Metadata{
				Title:      "Movie",
				Year:       2024,
				IsRemux:    true,
				VideoCodec: "AVC",
				Group:      "GRP",
			},
			template: `{{.Title}}.{{.Year}}.{{when .IsRemux "REMUX" "ENCODE"}}.{{.VideoCodec}}-{{.Group}}`,
			sep:      ".",
			want:     "Movie.2024.REMUX.AVC-GRP",
		},
		{
			name: "AudioSpec convenience field",
			meta: Metadata{
				Title:         "Movie",
				Year:          2024,
				AudioCodec:    "DDP",
				AudioChannels: "5.1",
				AudioExtra:    "Atmos",
				Group:         "GRP",
			},
			template: `{{.Title}}.{{.Year}}.{{.AudioSpec}}-{{.Group}}`,
			sep:      ".",
			want:     "Movie.2024.DDP5.1.Atmos-GRP",
		},
		{
			name: "String helpers upper lower title",
			meta: Metadata{
				Title: "the.dark.knight",
				Group: "grp",
			},
			template: `{{title .Title}}-{{upper .Group}}`,
			sep:      ".",
			want:     "The.dark.knight-GRP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.sep != "" {
				viper.Set("word_separator", tt.sep)
			} else {
				viper.Set("word_separator", ".")
			}

			if got := tt.meta.render(tt.template); got != tt.want {
				t.Errorf("render() = %q, want %q", got, tt.want)
			}
		})
	}
}

//nolint:funlen // comprehensive test matrix for codec style determination
func TestDetermineCodecStyle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		meta     Metadata
		expected string
	}{
		{
			name:     "Explicit override",
			meta:     Metadata{CodecStyle: "custom"},
			expected: "custom",
		},
		{
			name:     "Remux",
			meta:     Metadata{IsRemux: true},
			expected: "remux",
		},
		{
			name:     "Encode",
			meta:     Metadata{IsEncode: true},
			expected: "encode",
		},
		{
			name:     "WEB-DL from source",
			meta:     Metadata{Source: "WEB-DL"},
			expected: "web_dl",
		},
		{
			name:     "WEB-DL from service",
			meta:     Metadata{Service: "NF"},
			expected: "web_dl",
		},
		{
			name:     "WEB-DL with encode flag from CDN master",
			meta:     Metadata{Source: "WEB-DL", IsEncode: true},
			expected: "web_dl",
		},
		{
			name:     "WEBRip encode",
			meta:     Metadata{Source: "WEBRip", IsEncode: true},
			expected: "encode",
		},
		{
			name:     "Service with WEBRip source",
			meta:     Metadata{Service: "NF", Source: "WEBRip", IsEncode: true},
			expected: "encode",
		},
		{
			name:     "Default fallback",
			meta:     Metadata{},
			expected: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := DetermineCodecStyle(&tt.meta); got != tt.expected {
				t.Errorf("DetermineCodecStyle() = %q, want %q", got, tt.expected)
			}
		})
	}
}

//nolint:paralleltest // mutates global viper config for video_codec_style
func TestFormatVideoCodec_WithMatrix(t *testing.T) {
	config.InitDefaults()

	// Configure a sample video_codec_style matrix
	viper.Set("video_codec_style.web_dl.AVC", "H.264")
	viper.Set("video_codec_style.web_dl.HEVC", "H.265")
	viper.Set("video_codec_style.remux.AVC", "AVC")
	viper.Set("video_codec_style.remux.HEVC", "HEVC")
	viper.Set("video_codec_style.encode.AVC", "x264")
	viper.Set("video_codec_style.encode.HEVC", "x265")

	tests := []struct {
		codec    string
		style    string
		expected string
	}{
		{"AVC", "web_dl", "H.264"},
		{"HEVC", "web_dl", "H.265"},
		{"AVC", "remux", "AVC"},
		{"HEVC", "remux", "HEVC"},
		{"AVC", "encode", "x264"},
		{"HEVC", "encode", "x265"},
		{"x264", "encode", "x264"},
	}

	for _, tt := range tests {
		if got := FormatVideoCodec(tt.codec, tt.style); got != tt.expected {
			t.Errorf("FormatVideoCodec(%q, %q) = %q, want %q", tt.codec, tt.style, got, tt.expected)
		}
	}
}

func TestTemplateContext_YearTag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		year int
		want string
	}{
		{"Valid year", 2024, "2024"},
		{"Zero year", 0, ""},
		{"Negative year", -1, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			meta := Metadata{Year: tt.year}

			ctx := meta.ToTemplateContext()
			if ctx.YearTag != tt.want {
				t.Errorf("YearTag = %q, want %q", ctx.YearTag, tt.want)
			}
		})
	}
}

func TestCodecStyleMatrix(t *testing.T) {
	t.Parallel()

	// WEB-DL style: AVC -> H.264, HEVC -> H.265
	webDL := Metadata{
		Title:      "Show",
		Source:     "WEB-DL",
		VideoCodec: "AVC",
		Group:      "GRP",
	}

	ctxWeb := webDL.ToTemplateContext()
	if got, want := ctxWeb.VideoCodec, "H.264"; got != want {
		t.Errorf("WebDL style VideoCodec = %q, want %q", got, want)
	}

	// Remux style: AVC -> AVC, HEVC -> HEVC
	remux := Metadata{
		Title:      "Movie",
		Source:     "BluRay",
		IsRemux:    true,
		VideoCodec: "AVC",
		Group:      "GRP",
	}

	ctxRemux := remux.ToTemplateContext()
	if got, want := ctxRemux.VideoCodec, "AVC"; got != want {
		t.Errorf("Remux style VideoCodec = %q, want %q", got, want)
	}

	// Encode style: AVC -> x264, HEVC -> x265
	encode := Metadata{
		Title:      "Movie",
		Source:     "BluRay",
		IsEncode:   true,
		VideoCodec: "AVC",
		Group:      "GRP",
	}

	ctxEncode := encode.ToTemplateContext()
	if got, want := ctxEncode.VideoCodec, "x264"; got != want {
		t.Errorf("Encode style VideoCodec = %q, want %q", got, want)
	}

	// Override
	override := Metadata{
		Title:      "Movie",
		Source:     "WEB-DL",
		CodecStyle: "encode",
		VideoCodec: "HEVC",
		Group:      "GRP",
	}

	ctxOverride := override.ToTemplateContext()
	if got, want := ctxOverride.VideoCodec, "x265"; got != want {
		t.Errorf("Override style VideoCodec = %q, want %q", got, want)
	}
}
