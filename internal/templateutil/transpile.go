package templateutil

import "strings"

var legacyTokenMap = map[string]string{
	"{title}":          "{{.Title}}",
	"{year}":           "{{.YearTag}}",
	"{season_id}":      "{{.SeasonID}}",
	"{season_raw}":     `{{cond (or .IsTV (gt .Season 0)) .Season ""}}`,
	"{season_02}":      `{{cond (or .IsTV (gt .Season 0)) (pad 2 .Season) ""}}`,
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
	"{dual_audio}":     `{{cond .IsDualAudio "Dual-Audio" ""}}`,
	"{subbed}":         `{{cond .IsSubbed "[SUBBED]" ""}}`,
	"{crc32}":          "{{.CRC32}}",
	"{bit_depth}":      `{{cond (gt .BitDepth 8) (cat .BitDepth "bit") ""}}`,
	"{repack}":         "{{.RepackTag}}",
}

// TranspileLegacyTemplate converts legacy {token} placeholders into Go template syntax.
func TranspileLegacyTemplate(tmpl string) string {
	result := tmpl
	for legacyKey, modernKey := range legacyTokenMap {
		result = strings.ReplaceAll(result, legacyKey, modernKey)
	}

	return result
}
