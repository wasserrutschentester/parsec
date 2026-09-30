package templateutil

import (
	"testing"
)

func TestTranspileLegacyTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Standard legacy template",
			input:    "{title}.{year}.{season_id}{episode_id}.{resolution}-{group}",
			expected: `{{.Title}}.{{.YearTag}}.{{.SeasonID}}{{.EpisodeID}}.{{.Resolution}}-{{.Group}}`,
		},
		{
			name:     "Episode raw and padding",
			input:    "{title}.S{season_02}E{episode_02}",
			expected: `{{.Title}}.S{{when (or .IsTV (gt .Season 0)) (pad 2 .Season)}}E{{eprange "" 2 .Episodes}}`,
		},
		{
			name:     "Conditional legacy tokens",
			input:    "{title}.{dual_audio}.{subbed}.{repack}.{bit_depth}",
			expected: `{{.Title}}.{{when .IsDualAudio "Dual-Audio"}}.{{when .IsSubbed "[SUBBED]"}}.{{.RepackTag}}.{{when (gt .BitDepth 8) (cat .BitDepth "bit")}}`,
		},
		{
			name:     "Language and audio tokens",
			input:    "{language}.{language_ext}.{audio_codec}.{audio_channels}.{audio_meta}",
			expected: "{{.LanguageName}}.{{.LanguageExtra}}.{{.AudioCodec}}.{{.AudioChannels}}.{{.AudioExtra}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := TranspileLegacyTemplate(tt.input)
			if got != tt.expected {
				t.Errorf("TranspileLegacyTemplate() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestValidateTemplate(t *testing.T) {
	t.Parallel()

	validCases := []string{
		`{{join "." .Title .Year}}-{{.Group}}`,
		`{{when .IsDualAudio "Dual-Audio"}}`,
		`{{pad 2 .Season}}`,
		`{{where "Type" "audio" .Tracks}}`,
		`{title}.{year}-{group}`, // Transpiled legacy
	}

	for _, tc := range validCases {
		if err := ValidateTemplate(tc); err != nil {
			t.Errorf("ValidateTemplate(%q) unexpected error: %v", tc, err)
		}
	}

	invalidCases := []string{
		`{{join "." .Title`,      // Unclosed action
		`{{unknownFunc .Title}}`, // Unknown function
		`{{if .Title}}val`,       // Missing {{end}}
		`{title}.{{unclosed`,     // Broken syntax
	}

	for _, tc := range invalidCases {
		if err := ValidateTemplate(tc); err == nil {
			t.Errorf("ValidateTemplate(%q) expected error, got nil", tc)
		}
	}
}
