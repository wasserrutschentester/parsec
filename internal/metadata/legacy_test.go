package metadata

import (
	"testing"

	"github.com/spf13/viper"

	"codeberg.org/upPollo/parsec/internal/config"
)

//nolint:funlen,paralleltest // comprehensive table-driven test cases for legacy tokens; mutates global viper state
func TestLegacyTemplatesCompatibility(t *testing.T) {
	config.InitDefaults()

	tests := []struct {
		name     string
		meta     Metadata
		template string
		wordSep  string
		want     string
	}{
		{
			name: "All tokens populated",
			meta: Metadata{
				Title:         "Sample Movie",
				Year:          2023,
				Season:        1,
				Episodes:      []int{2},
				EpisodeTitles: []string{"Chapter Two"},
				Date:          "2023-01-15",
				Edition:       "Extended",
				LanguageISO:   "de",
				LanguageExtra: "DL",
				Accessibility: "Descriptive",
				Resolution:    "1080p",
				Service:       "NF",
				Source:        "WEB-DL",
				HDR:           "HDR10",
				AudioCodec:    "DDP",
				AudioChannels: "5.1",
				AudioExtra:    "Atmos",
				VideoCodec:    "H.265",
				Group:         "GRP",
				IsDualAudio:   true,
				IsSubbed:      true,
				CRC32:         "DEADBEEF",
				BitDepth:      10,
				IsRepack:      true,
				RepackLevel:   2,
			},
			template: "{title}.{year}.{season_id}{episode_id}.{season_raw}.{season_02}.{episode_raw}.{episode_02}.{episode_title}.{date}.{cut_edition}.{language}.{language_ext}.{accessibility}.{resolution}.{service}.{source}.{hdr}.{audio_codec}.{audio_channels}.{audio_meta}.{video_codec}.{dual_audio}.{subbed}.{crc32}.{bit_depth}.{repack}-{group}",
			wordSep:  ".",
			want:     "Sample.Movie.2023.S01E02.1.01.2.02.Chapter.Two.2023-01-15.Extended.GERMAN.DL.Descriptive.1080p.NF.WEB-DL.HDR10.DDP.5.1.Atmos.H.265.Dual-Audio.[SUBBED].DEADBEEF.10bit.REPACK2-GRP",
		},
		{
			name: "Empty / Zero year and season omitted",
			meta: Metadata{
				Title:       "Timeless Title",
				Year:        0,
				Season:      0,
				Resolution:  "1080p",
				Source:      "WEB-DL",
				VideoCodec:  "H.264",
				Group:       "GRP",
				BitDepth:    8, // 8-bit should be omitted for {bit_depth}
				IsRepack:    false,
				IsDualAudio: false,
				IsSubbed:    false,
			},
			template: "{title}.{year}.{season_raw}.{season_02}.{resolution}.{source}.{bit_depth}.{dual_audio}.{subbed}.{repack}-{group}",
			wordSep:  ".",
			want:     "Timeless.Title.1080p.WEB-DL-GRP",
		},
		{
			name: "Multi-episode range formatting",
			meta: Metadata{
				Title:      "Show",
				Season:     2,
				Episodes:   []int{1, 2, 3},
				Resolution: "720p",
				Source:     "HDTV",
				VideoCodec: "H.264",
				Group:      "GRP",
				IsTV:       true,
			},
			template: "{title}.{season_id}{episode_id}.{resolution}.{source}.{video_codec}-{group}",
			wordSep:  ".",
			want:     "Show.S02E01-E03.720p.HDTV.H.264-GRP",
		},
		{
			name: "Multi-episode with episode_raw and episode_02",
			meta: Metadata{
				Title:    "Show",
				Episodes: []int{5, 6},
				Group:    "GRP",
				IsTV:     true,
			},
			template: "{title}.E{episode_raw}.E{episode_02}-{group}",
			wordSep:  ".",
			want:     "Show.E5-6.E05-06-GRP",
		},
		{
			name: "Anime format with space separator and brackets",
			meta: Metadata{
				Title:       "Attack on Titan",
				Season:      4,
				Episodes:    []int{28},
				Source:      "BluRay",
				Resolution:  "1080p",
				VideoCodec:  "x264",
				AudioCodec:  "FLAC",
				IsDualAudio: true,
				CRC32:       "A1B2C3D4",
				Group:       "AnimeGrp",
				IsTV:        true,
			},
			template: "[{group}] {title} - {season_id}{episode_id} - ({source} {resolution} {video_codec} {audio_codec}) {dual_audio} [{crc32}]",
			wordSep:  " ",
			want:     "[AnimeGrp] Attack on Titan - S04E28 - (BluRay 1080p x264 FLAC) Dual-Audio [A1B2C3D4]",
		},
		{
			name: "German Scene standard template",
			meta: Metadata{
				Title:         "Der Pate",
				Year:          1972,
				Edition:       "Remastered",
				LanguageISO:   "de",
				Resolution:    "2160p",
				Source:        "UHD.BluRay",
				AudioCodec:    "DTS-HD MA",
				AudioChannels: "5.1",
				HDR:           "HDR",
				VideoCodec:    "HEVC",
				Group:         "CINE",
			},
			template: "{title}.{year}.{cut_edition}.{language}.{resolution}.{source}.{audio_codec}.{audio_channels}.{hdr}.{video_codec}-{group}",
			wordSep:  ".",
			want:     "Der.Pate.1972.Remastered.GERMAN.2160p.UHD.BluRay.DTS-HD.MA.5.1.HDR.HEVC-CINE",
		},
		{
			name: "Repack level 1 vs repack level 3",
			meta: Metadata{
				Title:       "Show",
				IsRepack:    true,
				RepackLevel: 1,
				Group:       "GRP",
			},
			template: "{title}.{repack}-{group}",
			wordSep:  ".",
			want:     "Show.REPACK-GRP",
		},
		{
			name: "Repack level 3",
			meta: Metadata{
				Title:       "Show",
				IsRepack:    true,
				RepackLevel: 3,
				Group:       "GRP",
			},
			template: "{title}.{repack}-{group}",
			wordSep:  ".",
			want:     "Show.REPACK3-GRP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origSep := viper.GetString("word_separator")

			if tt.wordSep != "" {
				viper.Set("word_separator", tt.wordSep)
			}

			t.Cleanup(func() {
				viper.Set("word_separator", origSep)
			})

			got := tt.meta.render(tt.template)
			if got != tt.want {
				t.Errorf("render(%q) =\n got  %q\n want %q", tt.template, got, tt.want)
			}
		})
	}
}
