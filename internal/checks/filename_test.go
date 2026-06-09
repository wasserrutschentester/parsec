package checks

import (
	"testing"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata"
)

func TestCheckAllowedCharacters(t *testing.T) {
	tests := []struct {
		filename string
		want     bool // true if invalid characters found
	}{
		{"Valid.Filename-2024", false},
		{"Invalid_Filename", true},
		{"Invalid!Filename", true},
		{"Invalid space", true},
	}
	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got, _ := findNotAllowedCharacters(tt.filename)
			if (got != "") != tt.want {
				t.Errorf("checkAllowedCharacters() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckCharacterSequences(t *testing.T) {
	tests := []struct {
		filename string
		want     bool // true if invalid sequence found
	}{
		{"Valid.Filename.mkv", false},
		{"Invalid...Filename", true},
		{"Invalid.-.Filename", true},
		{"Invalid.-..Filename", true},
	}
	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got, _ := findCharacterSequences(tt.filename)
			if (got != "") != tt.want {
				t.Errorf("checkCharacterSequences() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunFilenameChecks(t *testing.T) {
	config.InitDefaults()

	tests := []struct {
		name     string
		filename string
		meta     *metadata.Metadata
		wantFail bool
	}{
		{
			name:     "Valid",
			filename: "Movie.Title.2023.GERMAN.1080p.BluRay.x264-GROUP",
			meta: &metadata.Metadata{
				Title:      "Movie.Title",
				Year:       2023,
				Language:   "GERMAN",
				Resolution: "1080p",
				Source:     "BluRay",
				VideoCodec: "x264",
				Group:      "GROUP",
			},
			wantFail: false,
		},
		{
			name:     "Invalid character",
			filename: "Movie_Title.2023",
			meta:     &metadata.Metadata{Title: "Movie_Title", Year: 2023},
			wantFail: true,
		},
		{
			name:     "Year missing (Movie)",
			filename: "Movie.Title.GERMAN.1080p.BluRay.x264-GROUP",
			meta: &metadata.Metadata{
				Title:      "Movie.Title",
				Language:   "GERMAN",
				Resolution: "1080p",
				Source:     "BluRay",
				VideoCodec: "x264",
				Group:      "GROUP",
				IsTV:       false,
			},
			wantFail: true,
		},
		{
			name:     "Year redundant (TV)",
			filename: "Series.Title.S2023E01.GERMAN.1080p.WEB-DL.h264-GROUP",
			meta: &metadata.Metadata{
				Title:      "Series.Title",
				Year:       2023,
				Season:     2023,
				Episode:    1,
				Language:   "GERMAN",
				Resolution: "1080p",
				Source:     "WEB-DL",
				VideoCodec: "h264",
				Group:      "GROUP",
				IsTV:       true,
			},
			wantFail: true,
		},
		{
			name:     "Streaming Service Missing (WEB)",
			filename: "Movie.Title.2023.GERMAN.1080p.WEB-DL.x264-GROUP",
			meta: &metadata.Metadata{
				Title:      "Movie.Title",
				Year:       2023,
				Language:   "GERMAN",
				Resolution: "1080p",
				Source:     "WEB-DL",
				VideoCodec: "x264",
				Group:      "GROUP",
			},
			wantFail: true,
		},
		{
			name:     "TV Special Missing Info",
			filename: "Series.Title.S00E01.GERMAN.1080p.WEB-DL.H264-GROUP",
			meta: &metadata.Metadata{
				Title:      "Series.Title",
				Season:     0,
				Episode:    1,
				Language:   "GERMAN",
				Resolution: "1080p",
				Source:     "WEB-DL",
				VideoCodec: "H264",
				Group:      "GROUP",
				IsTV:       true,
			},
			wantFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := RunFilenameChecks(tt.filename, tt.meta)
			hasFailure := false

			for _, r := range results {
				if !r.Passed {
					hasFailure = true
					break
				}
			}

			if hasFailure != tt.wantFail {
				t.Errorf("RunFilenameChecks() [%s] hasFailure = %v, wantFail %v", tt.name, hasFailure, tt.wantFail)
			}
		})
	}
}
