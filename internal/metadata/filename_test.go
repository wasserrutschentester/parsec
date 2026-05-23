package metadata

import (
	"testing"
)

// Test cases for filename parsing
func TestParseFilename(t *testing.T) {
	tests := []struct {
		filename string
		expected Metadata
	}{
		{
			filename: "Film.Titel.2000.GERMAN.1080p.ARD.WEB-DL.AAC2.0.H.264-GRP.mkv",
			expected: Metadata{
				Resolution:    "1080p",
				AudioCodec:    "AAC",
				AudioChannels: "2.0",
				VideoCodec:    "H.264",
			},
		},
		{
			filename: "Das.Traumschiff.S2026E03.Honululu.GERMAN.1080p.ZDF.WEB-DL.AAC2.0.H.264-GRP",
			expected: Metadata{
				Resolution:    "1080p",
				AudioCodec:    "AAC",
				AudioChannels: "2.0",
				VideoCodec:    "H.264",
			},
		},
		{
			filename: "Anderer.Film.1969.GERMAN.720p.WEB-DL.DDP5.1.H.264-GRP.mkv",
			expected: Metadata{
				Resolution:    "720p",
				AudioCodec:    "DDP",
				AudioChannels: "5.1",
				VideoCodec:    "H.264",
			},
		},
		{
			filename: "Film.2024.1080p.WEB-DL.AAC2.0.H.265-GRP.mkv",
			expected: Metadata{
				Resolution:    "1080p",
				AudioCodec:    "AAC",
				AudioChannels: "2.0",
				VideoCodec:    "H.265",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := ParseFilename(tt.filename)
			if got.Resolution != tt.expected.Resolution {
				t.Errorf("Resolution: got %s, want %s", got.Resolution, tt.expected.Resolution)
			}
			if got.AudioCodec != tt.expected.AudioCodec {
				t.Errorf("AudioCodec: got %s, want %s", got.AudioCodec, tt.expected.AudioCodec)
			}
			if got.AudioChannels != tt.expected.AudioChannels {
				t.Errorf("AudioChannels: got %s, want %s", got.AudioChannels, tt.expected.AudioChannels)
			}
			if got.VideoCodec != tt.expected.VideoCodec {
				t.Errorf("VideoCodec: got %s, want %s", got.VideoCodec, tt.expected.VideoCodec)
			}
		})
	}
}
