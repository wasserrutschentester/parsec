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
			template: `{{titleCase .Title}}-{{toUpper .Group}}`,
			sep:      ".",
			want:     "The.dark.knight-GRP",
		},
		{
			name: "Contains function with slice",
			meta: Metadata{
				Title:          "Movie",
				Year:           2023,
				AudioLanguages: []string{"en", "de"},
				Resolution:     "1080p",
				Group:          "GRP",
			},
			template: `{{.Title}}.{{.YearTag}}{{if contains "de" .AudioLanguages}}.German{{end}}-{{.Group}}`,
			sep:      ".",
			want:     "Movie.2023.German-GRP",
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

//nolint:funlen,paralleltest // comprehensive test matrix for codec style determination mutating Metadata struct
func TestDetermineCodecStyle(t *testing.T) {
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
			name:     "WEBRip without encode flag",
			meta:     Metadata{Source: "WEBRip"},
			expected: "encode",
		},
		{
			name:     "Service with WEBRip source without encode flag",
			meta:     Metadata{Service: "NF", Source: "WEBRip"},
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

//nolint:paralleltest // mutates global viper config
func TestFormatVideoCodec_DefaultStyle(t *testing.T) {
	origAVC := viper.GetString("video_codec_avc")
	origHEVC := viper.GetString("video_codec_hevc")

	t.Cleanup(func() {
		viper.Reset()
		config.InitDefaults()
		viper.Set("video_codec_avc", origAVC)
		viper.Set("video_codec_hevc", origHEVC)
	})

	// 1. Explicit default table
	viper.Set("video_codec_style.default.AVC", "H.264")
	viper.Set("video_codec_style.default.HEVC", "H.265")
	viper.Set("video_codec_style.partial.AVC", "CustomAVC")
	// "partial" intentionally omits HEVC to test fallback

	if got := FormatVideoCodec("AVC", "default"); got != "H.264" {
		t.Errorf("FormatVideoCodec('AVC', 'default') = %q, want 'H.264'", got)
	}

	if got := FormatVideoCodec("AVC", ""); got != "H.264" {
		t.Errorf("FormatVideoCodec('AVC', '') = %q, want 'H.264'", got)
	}

	// Unknown style falls back to default table
	if got := FormatVideoCodec("AVC", "unknown_style"); got != "H.264" {
		t.Errorf("FormatVideoCodec('AVC', 'unknown_style') = %q, want 'H.264'", got)
	}

	// Missing codec in specific style falls back to default table
	if got := FormatVideoCodec("HEVC", "partial"); got != "H.265" {
		t.Errorf("FormatVideoCodec('HEVC', 'partial') fallback = %q, want 'H.265'", got)
	}

	// 2. Default style set as an alias to another style
	viper.Set("video_codec_style.default", "encode")
	viper.Set("video_codec_style.encode.AVC", "x264")
	viper.Set("video_codec_style.encode.HEVC", "x265")

	if got := FormatVideoCodec("AVC", "default"); got != "x264" {
		t.Errorf("FormatVideoCodec('AVC', 'default') alias = %q, want 'x264'", got)
	}

	if got := FormatVideoCodec("HEVC", "default"); got != "x265" {
		t.Errorf("FormatVideoCodec('HEVC', 'default') alias = %q, want 'x265'", got)
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

func TestTemplateContext_IsPack(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		meta Metadata
		want bool
	}{
		{
			name: "Regular Season Pack S01",
			meta: Metadata{Season: 1, Episodes: nil},
			want: true,
		},
		{
			name: "Season 0 Specials Pack S00",
			meta: Metadata{Season: 0, IsTV: true, Episodes: nil},
			want: true,
		},
		{
			name: "Single TV Episode S01E01",
			meta: Metadata{Season: 1, IsTV: true, Episodes: []int{1}},
			want: false,
		},
		{
			name: "Movie not TV",
			meta: Metadata{Season: 0, IsTV: false, Episodes: nil},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := tt.meta.ToTemplateContext()
			if ctx.IsPack != tt.want {
				t.Errorf("IsPack = %v, want %v", ctx.IsPack, tt.want)
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

//nolint:paralleltest // mutates global viper config
func TestFormatVideoCodec_LegacyOverrides(t *testing.T) {
	origAVC := viper.GetString("video_codec_avc")
	origHEVC := viper.GetString("video_codec_hevc")

	t.Cleanup(func() {
		viper.Set("video_codec_avc", origAVC)
		viper.Set("video_codec_hevc", origHEVC)
	})

	// When not set, default style preserves raw codec
	viper.Set("video_codec_avc", "")
	viper.Set("video_codec_hevc", "")

	if got := FormatVideoCodec("AVC", "default"); got != "AVC" {
		t.Errorf("expected raw 'AVC', got %q", got)
	}

	if got := FormatVideoCodec("HEVC", "default"); got != "HEVC" {
		t.Errorf("expected raw 'HEVC', got %q", got)
	}

	// Legacy settings override default style
	viper.Set("video_codec_avc", "x264")
	viper.Set("video_codec_hevc", "x265")

	if got := FormatVideoCodec("AVC", "default"); got != "x264" {
		t.Errorf("expected overridden 'x264', got %q", got)
	}

	if got := FormatVideoCodec("HEVC", "default"); got != "x265" {
		t.Errorf("expected overridden 'x265', got %q", got)
	}

	// Specific styles take precedence over legacy overrides
	if got := FormatVideoCodec("AVC", "remux"); got != "AVC" {
		t.Errorf("remux style should take precedence over legacy override; got %q", got)
	}
}

//nolint:paralleltest // uses default config template
func TestDefaultTemplate_RemuxAndEncode(t *testing.T) {
	config.InitDefaults()

	remux := Metadata{
		Title:         "Movie",
		Year:          2024,
		Resolution:    "1080p",
		Source:        "BluRay",
		IsRemux:       true,
		AudioCodec:    "DTS-HD MA",
		AudioChannels: "5.1",
		VideoCodec:    "AVC",
		Group:         "GRP",
	}

	wantRemux := "Movie.2024.1080p.BluRay.REMUX.DTS-HD.MA5.1.AVC-GRP"
	remuxName := remux.GetReleaseName()

	if remuxName != wantRemux {
		t.Errorf("Remux release name = %q, want %q", remuxName, wantRemux)
	}

	encode := Metadata{
		Title:         "Movie",
		Year:          2024,
		Resolution:    "1080p",
		Source:        "BluRay",
		IsEncode:      true,
		AudioCodec:    "DTS-HD MA",
		AudioChannels: "5.1",
		VideoCodec:    "AVC",
		Group:         "GRP",
	}

	wantEncode := "Movie.2024.1080p.BluRay.DTS-HD.MA5.1.x264-GRP"
	encodeName := encode.GetReleaseName()

	if encodeName != wantEncode {
		t.Errorf("Encode release name = %q, want %q", encodeName, wantEncode)
	}
}

//nolint:funlen,paralleltest // table-driven test covering multiple template helpers
func TestTemplate_PipingAndUnifiedHelpers(t *testing.T) {
	config.InitDefaults()

	meta := Metadata{
		Title:      "The.Great.Movie",
		Year:       2024,
		Date:       "2024-05-18",
		Resolution: "1080p",
		Group:      "grp",
	}

	tCtx := meta.ToTemplateContextWithRaw("mi-mock", "search-mock", "ebml-mock", "ep-mock")

	// Verify raw fields
	if tCtx.RawMediaInfo != "mi-mock" || tCtx.RawEbmlMetadata != "ebml-mock" ||
		tCtx.RawSearchResult != "search-mock" || tCtx.RawEpisodeResults != "ep-mock" {
		t.Errorf("Raw fields mismatch: %+v", tCtx)
	}

	tests := []struct {
		name     string
		template string
		want     string
	}{
		{
			name:     "Piping trimPrefix",
			template: `{{.Title | trimPrefix "The."}}-{{.Group}}`,
			want:     "Great.Movie-grp",
		},
		{
			name:     "Piping trimSuffix",
			template: `{{.Title | trimSuffix ".Movie"}}-{{.Group}}`,
			want:     "The.Great-grp",
		},
		{
			name:     "Piping replace",
			template: `{{.Title | replace "Great" "Super"}}-{{.Group}}`,
			want:     "The.Super.Movie-grp",
		},
		{
			name:     "Casing toUpper, toLower, titleCase",
			template: `{{.Title | toLower}}.{{.Group | toUpper}}`,
			want:     "the.great.movie.GRP",
		},
		{
			name:     "FormatDate",
			template: `{{.Title}}.{{.Date | formatDate "20060102"}}-{{.Group}}`,
			want:     "The.Great.Movie.20240518-grp",
		},
		{
			name:     "List with join",
			template: `{{list "A" "B" "C" | join "."}}`,
			want:     "A.B.C",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tCtx.Render(tt.template)
			if got != tt.want {
				t.Errorf("Render(%q) = %q, want %q", tt.template, got, tt.want)
			}
		})
	}
}
