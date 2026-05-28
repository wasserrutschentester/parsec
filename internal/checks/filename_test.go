package checks

import (
	"testing"

	"codeberg.org/n0ne/parsec/internal/config"
	"codeberg.org/n0ne/parsec/internal/metadata"
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
			got := CheckAllowedCharacters(tt.filename)
			if (got != "") != tt.want {
				t.Errorf("CheckAllowedCharacters() = %v, want %v", got, tt.want)
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
			got := CheckCharacterSequences(tt.filename)
			if (got != "") != tt.want {
				t.Errorf("CheckCharacterSequences() = %v, want %v", got, tt.want)
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure we use the default template for metadata.String()
			results := RunFilenameChecks(tt.filename, tt.meta)
			hasFailure := false
			for _, r := range results {
				if !r.Passed {
					hasFailure = true
					break
				}
			}
			if hasFailure != tt.wantFail {
				t.Errorf("RunFilenameChecks() hasFailure = %v, wantFail %v", hasFailure, tt.wantFail)
			}
		})
	}
}
