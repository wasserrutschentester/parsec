package templateutil

import "strings"

// LegacyTokenMap maps legacy {token} placeholders to their Go template equivalents.
var LegacyTokenMap = map[string]string{
	"{title}":          "{{.Title}}",
	"{year}":           "{{.YearTag}}",
	"{season_id}":      "{{.SeasonID}}",
	"{season_raw}":     `{{when (or .IsTV (gt .Season 0)) .Season}}`,
	"{season_02}":      `{{when (or .IsTV (gt .Season 0)) (pad 2 .Season)}}`,
	"{episode_id}":     "{{.EpisodeID}}",
	"{episode_raw}":    `{{eprange "" 0 .Episodes}}`,
	"{episode_02}":     `{{eprange "" 2 .Episodes}}`,
	"{episode_03}":     `{{eprange "" 3 .Episodes}}`,
	"{episode_04}":     `{{eprange "" 4 .Episodes}}`,
	"{episode_title}":  "{{.EpisodeTitle}}",
	"{date}":           "{{.Date}}",
	"{language}":       "{{.LanguageName}}",
	"{language_ext}":   "{{.LanguageExtra}}",
	"{cut_edition}":    "{{.Edition}}",
	"{accessibility}":  "{{.Accessibility}}",
	"{resolution}":     "{{.Resolution}}",
	"{service}":        "{{.Service}}",
	"{source}":         "{{.Source}}",
	"{hdr}":            "{{.HDR}}",
	"{audio_codec}":    "{{.AudioCodec}}",
	"{audio_channels}": "{{.AudioChannels}}",
	"{audio_meta}":     "{{.AudioExtra}}",
	"{video_codec}":    "{{.VideoCodec}}",
	"{group}":          "{{.Group}}",
	"{dual_audio}":     `{{when .IsDualAudio "Dual-Audio"}}`,
	"{subbed}":         `{{when .IsSubbed "[SUBBED]"}}`,
	"{crc32}":          "{{.CRC32}}",
	"{bit_depth}":      `{{when (gt .BitDepth 8) (cat .BitDepth "bit")}}`,
	"{repack}":         "{{.RepackTag}}",
}

var legacyReplacer = func() *strings.Replacer {
	pairs := make([]string, 0, len(LegacyTokenMap)*2)
	for k, v := range LegacyTokenMap {
		pairs = append(pairs, k, v)
	}

	return strings.NewReplacer(pairs...)
}()

// TranspileLegacyTemplate converts legacy {token} placeholders into Go template syntax.
func TranspileLegacyTemplate(tmpl string) string {
	return legacyReplacer.Replace(tmpl)
}
