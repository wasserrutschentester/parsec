package filename

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"codeberg.org/n0ne/parsec/internal/metadata"
)

func compareMetadata(got, want metadata.Metadata) string {
	var diffs []string
	vGot := reflect.ValueOf(got)
	vWant := reflect.ValueOf(want)
	typeOfS := vGot.Type()

	for i := 0; i < vGot.NumField(); i++ {
		fieldGot := vGot.Field(i).Interface()
		fieldWant := vWant.Field(i).Interface()
		if !reflect.DeepEqual(fieldGot, fieldWant) {
			diffs = append(diffs, fmt.Sprintf("%s: got %v, want %v", typeOfS.Field(i).Name, fieldGot, fieldWant))
		}
	}
	return strings.Join(diffs, "\n")
}

// Test cases for filename parsing
func TestParse(t *testing.T) {
	tests := []struct {
		filename string
		expected metadata.Metadata
	}{
		{
			filename: "Film.Titel.2000.GERMAN.1080p.ARD.WEB-DL.AAC2.0.H.264-GRP.mkv",
			expected: metadata.Metadata{
				Title:         "Film.Titel",
				Year:          2000,
				Language:      "GERMAN",
				Resolution:    "1080p",
				Service:       "ARD",
				Source:        "WEB-DL",
				AudioCodec:    "AAC",
				AudioChannels: "2.0",
				VideoCodec:    "H.264",
				Group:         "GRP.mkv",
				IsTV:          false,
			},
		},
		{
			filename: "Das.Traumschiff.S2026E03.Honululu.GERMAN.1080p.ZDF.WEB-DL.AAC2.0.H.264-GRP",
			expected: metadata.Metadata{
				Title:         "Das.Traumschiff",
				Season:        2026,
				Episode:       3,
				EpisodeTitle:  "Honululu",
				Language:      "GERMAN",
				Resolution:    "1080p",
				Service:       "ZDF",
				Source:        "WEB-DL",
				AudioCodec:    "AAC",
				AudioChannels: "2.0",
				VideoCodec:    "H.264",
				Group:         "GRP",
				IsTV:          true,
			},
		},
		{
			filename: "Anderer.Film.1969.German.720p.WEB-DL.DDP5.1.H.264-GRP.mkv",
			expected: metadata.Metadata{
				Title:         "Anderer.Film",
				Year:          1969,
				Language:      "German",
				Resolution:    "720p",
				Source:        "WEB-DL",
				AudioCodec:    "DDP",
				AudioChannels: "5.1",
				VideoCodec:    "H.264",
				Group:         "GRP.mkv",
				IsTV:          false,
			},
		},
		{
			filename: "Film.2024.1080p.WEB-DL.AAC2.0.H.265-GRP.mkv",
			expected: metadata.Metadata{
				Title:         "Film",
				Year:          2024,
				Resolution:    "1080p",
				Source:        "WEB-DL",
				AudioCodec:    "AAC",
				AudioChannels: "2.0",
				VideoCodec:    "H.265",
				Group:         "GRP.mkv",
				IsTV:          false,
			},
		},
		{
			filename: "Daily.Show.2024-05-24.720p.WEB-DL.AAC2.0.H.264-GRP",
			expected: metadata.Metadata{
				Title:         "Daily.Show",
				Date:          "2024-05-24",
				Resolution:    "720p",
				Source:        "WEB-DL",
				AudioCodec:    "AAC",
				AudioChannels: "2.0",
				VideoCodec:    "H.264",
				Group:         "GRP",
				IsTV:          true,
			},
		},
		{
			filename: "Movie.2024.1080p.BluRay.DDP5.1.x264-GRP",
			expected: metadata.Metadata{
				Title:         "Movie",
				Year:          2024,
				Resolution:    "1080p",
				Source:        "BluRay",
				AudioCodec:    "DDP",
				AudioChannels: "5.1",
				VideoCodec:    "x264",
				Group:         "GRP",
				IsTV:          false,
			},
		},
		{
			filename: "Series.S01E02.Multi.1080p.Netflix.WEBRip.DDP5.1.x265-GRP",
			expected: metadata.Metadata{
				Title:         "Series",
				Season:        1,
				Episode:       2,
				Language:      "Multi",
				Resolution:    "1080p",
				Service:       "Netflix",
				Source:        "WEBRip",
				AudioCodec:    "DDP",
				AudioChannels: "5.1",
				VideoCodec:    "x265",
				Group:         "GRP",
				IsTV:          true,
			},
		},
		{
			filename: "Film.2024.GERMAN.DL.WITH.AD.1080p.BluRay.DDP5.1.x264-GRP",
			expected: metadata.Metadata{
				Title:         "Film",
				Year:          2024,
				Language:      "GERMAN",
				LanguageExt:   "DL",
				Accessibility: "WITH.AD",
				HasAudioDesc:  true,
				Resolution:    "1080p",
				Source:        "BluRay",
				AudioCodec:    "DDP",
				AudioChannels: "5.1",
				VideoCodec:    "x264",
				Group:         "GRP",
				IsTV:          false,
			},
		},
		{
			filename: "Moneyland.Die.dunklen.Geschaefte.der.Finanzindustrie.2025.GERMAN.DL.with.Audio.Description.1080p.ARTE.WEB-DL.AAC2.0.H.265-NoGroup",
			expected: metadata.Metadata{
				Title:         "Moneyland.Die.dunklen.Geschaefte.der.Finanzindustrie",
				Year:          2025,
				Language:      "GERMAN",
				LanguageExt:   "DL",
				Accessibility: "with.Audio.Description",
				HasAudioDesc:  true,
				Resolution:    "1080p",
				Service:       "ARTE",
				Source:        "WEB-DL",
				AudioCodec:    "AAC",
				AudioChannels: "2.0",
				VideoCodec:    "H.265",
				Group:         "NoGroup",
				IsTV:          false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := Parse(tt.filename)
			if !reflect.DeepEqual(*got, tt.expected) {
				t.Errorf("Differences found:\n%s", compareMetadata(*got, tt.expected))
			}
		})
	}
}

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
			if (got != nil) != tt.want {
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
			if (got != nil) != tt.want {
				t.Errorf("CheckCharacterSequences() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMissingYear(t *testing.T) {
	tests := []struct {
		filename string
		expected metadata.Metadata
	}{
		{
			filename: "Film.Titel.GERMAN.1080p.ARD.WEB-DL.AAC2.0.H.264-GRP.mkv",
			expected: metadata.Metadata{
				Title:         "Film.Titel",
				Language:      "GERMAN",
				Resolution:    "1080p",
				Service:       "ARD",
				Source:        "WEB-DL",
				AudioCodec:    "AAC",
				AudioChannels: "2.0",
				VideoCodec:    "H.264",
				Group:         "GRP.mkv",
			},
		},
		{
			filename: "Simple.Movie.1080p.x264-GRP",
			expected: metadata.Metadata{
				Title:      "Simple.Movie",
				Resolution: "1080p",
				VideoCodec: "x264",
				Group:      "GRP",
			},
		},
		{
			filename: "Repack.Movie.REPACK.720p.WEB-DL.AAC2.0.x264-GRP",
			expected: metadata.Metadata{
				Title:         "Repack.Movie",
				Repack:        true,
				Resolution:    "720p",
				Source:        "WEB-DL",
				AudioCodec:    "AAC",
				AudioChannels: "2.0",
				VideoCodec:    "x264",
				Group:         "GRP",
			},
		},
		{
			filename: "Repack-end.Movie.1080p.BluRay.x264.REPACK-GRP",
			expected: metadata.Metadata{
				Title:      "Repack-end.Movie",
				Repack:     true,
				Resolution: "1080p",
				Source:     "BluRay",
				VideoCodec: "x264",
				Group:      "GRP",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := Parse(tt.filename)
			if !reflect.DeepEqual(*got, tt.expected) {
				t.Errorf("Differences found:\n%s", compareMetadata(*got, tt.expected))
			}
		})
	}
}

func TestDeobfuscateTitle(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Gloeckner", "Glöckner"},
		{"Ueber", "über"},
		{"neue", "neue"},
		{"Aerzte", "ärzte"},
		{"Koeln", "Köln"},
		{"Muenchen", "München"},
		{"Baeume", "Bäume"},
		{"Modehaeuser", "Modehäuser"},
		{"Bloede Buehnenduesen", "Blöde Bühnendüsen"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := DeobfuscateTitle(tt.input)
			if got != tt.expected {
				t.Errorf("DeobfuscateTitle(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNormalizeTitle(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"München", "Muenchen"},
		{"Blöde Bühnendüsen", "Bloede.Buehnenduesen"},
		{"Film & Dokumentation", "Film.und.Dokumentation"},
		{"Das.Traumschiff.(S01_E01)", "Das.Traumschiff"},
		{"Bam.Fernsehfilm.Deutschland.2023", "Bam.2023"},
		{"FooMärchenfilm.Österreich.1990", "Foo.1990"},
		{"Test...Sequence.-..Fix", "Test.Sequence.Fix"},
		{"Café.Smørebrød", "Cafe.Smoerebroed"},
		{"Title with (parentheses) and \"quotes\"", "Title.with.parentheses.and.quotes"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeTitle(tt.input)
			if got != tt.expected {
				t.Errorf("NormalizeTitle(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNormalizeService(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"SWR", "ARD"},
		{"RBB", "ARD"},
		{"WDR", "ARD"},
		{"MDR", "ARD"},
		{"NDR", "ARD"},
		{"BR", "ARD"},
		{"HR", "ARD"},
		{"rbtv", "ARD"},
		{"ZDFneo", "ZDF"},
		{"ZDFkultur", "ZDF"},
		{"ZDFtivi", "ZDF"},
		{"ZDF", "ZDF"},
		{"Netflix", "NF"},
		{"ARD", "ARD"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeService(tt.input)
			if got != tt.expected {
				t.Errorf("NormalizeService(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
