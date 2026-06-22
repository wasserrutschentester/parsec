package filename

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata"
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

func TestApplyReplacements(t *testing.T) {
	t.Parallel()

	rules := []config.Replacement{
		{Pattern: `(?i)(\d{4})_(\d{2})`, Replacement: "$1-$2"},
		{Pattern: "_", Replacement: "."},
		{Pattern: `(?i)web-rip`, Replacement: "WEBRip"},
		{Pattern: `\[invalid_regex(`, Replacement: "should not crash"}, // invalid regex
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"my_file_name_is_cool", "my.file.name.is.cool"},
		{"movie.2023.web-rip", "movie.2023.WEBRip"},
		{"show.2023_01_02", "show.2023-01.02"},
		{"nothing to change", "nothing to change"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got := ApplyReplacements(tt.input, rules)
			if got != tt.expected {
				t.Errorf("ApplyReplacements(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func runTableTest[T any](t *testing.T, tests []struct {
	input    string
	expected T
}, fn func(string) T, compare func(T, T) string,
) {
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got := fn(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("Differences found:\n%s", compare(got, tt.expected))
			}
		})
	}
}

// Test cases for filename parsing
//
//nolint:funlen // long list of test cases is required to cover various naming conventions
func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected metadata.Metadata
	}{
		{
			input: "Film.Titel.2000.GERMAN.1080p.ARD.WEB-DL.AAC2.0.H.264-GRP.mkv",
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
			input: "Das.Traumschiff.S2026E03.Honululu.GERMAN.1080p.ZDF.WEB-DL.AAC2.0.H.264-GRP",
			expected: metadata.Metadata{
				Title:         "Das.Traumschiff",
				Season:        2026,
				Episodes:      []int{3},
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
			input: "Anderer.Film.1969.German.720p.WEB-DL.DDP5.1.H.264-GRP.mkv",
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
			input: "Film.2024.1080p.WEB-DL.AAC2.0.H.265-GRP.mkv",
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
			input: "Daily.Show.2024-05-24.720p.WEB-DL.AAC2.0.H.264-GRP",
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
			input: "Movie.Name.2023.2160p.DIRECTORS.CUT.mkv",
			expected: metadata.Metadata{
				Title:      "Movie.Name",
				Year:       2023,
				Resolution: "2160p",
				CutEdition: "DIRECTORS.CUT",
				IsTV:       false,
			},
		},
		{
			input: "Movie.Name.2023.2160p.Director's.Cut.mkv",
			expected: metadata.Metadata{
				Title:      "Movie.Name",
				Year:       2023,
				Resolution: "2160p",
				CutEdition: "Director's.Cut",
				IsTV:       false,
			},
		},
		{
			input: "Show.S01E01.1080p.Open.Matte.mkv",
			expected: metadata.Metadata{
				Title:      "Show",
				Season:     1,
				Episodes:   []int{1},
				Resolution: "1080p",
				CutEdition: "Open.Matte",
				IsTV:       true,
			},
		},
		{
			input: "Movie.3D.HSBS.1080p.mkv",
			expected: metadata.Metadata{
				Title:      "Movie",
				Resolution: "1080p",
				CutEdition: "3D.HSBS",
				IsTV:       false,
			},
		},
		{
			input: "Avatar.The.Way.of.Water.2022.3D.SBS.DIRECTORS.CUT.2160p.mkv",
			expected: metadata.Metadata{
				Title:      "Avatar.The.Way.of.Water",
				Year:       2022,
				Resolution: "2160p",
				CutEdition: "DIRECTORS.CUT.3D.SBS",
				IsTV:       false,
			},
		},
		{
			input: "Movie.2024.1080p.BluRay.DDP5.1.x264-GRP",
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
			input: "Series.S01E02.Multi.1080p.Netflix.WEBRip.DDP5.1.x265-GRP",
			expected: metadata.Metadata{
				Title:         "Series",
				Season:        1,
				Episodes:      []int{2},
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
			input: "Film.2024.GERMAN.DL.WITH.AD.1080p.BluRay.DDP5.1.x264-GRP",
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
			input: "Moneyland.Die.dunklen.Geschaefte.der.Finanzindustrie.2025.GERMAN.DL.with.Audio.Description.1080p.ARTE.WEB-DL.AAC2.0.H.265-NoGroup",
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
		{
			input: "ZDF.Magazin.Royale.S00E166.2026-05-29.Die.Colonius-Sprengung.ZMR.vor.Ort.GERMAN.1080p.ZDF.WEB-DL.h264-SLiDE",
			expected: metadata.Metadata{
				Title:        "ZDF.Magazin.Royale",
				Season:       0,
				Episodes:     []int{166},
				Date:         "2026-05-29",
				EpisodeTitle: "Die.Colonius-Sprengung.ZMR.vor.Ort",
				Language:     "GERMAN",
				Resolution:   "1080p",
				Service:      "ZDF",
				Source:       "WEB-DL",
				VideoCodec:   "h264",
				Group:        "SLiDE",
				IsTV:         true,
			},
		},
		{
			input: "ZDF.Magazin.Royale.S2026E166.2026-05-29.Die.Colonius-Sprengung.ZMR.vor.Ort.GERMAN.1080p.ZDF.WEB-DL.h264-SLiDE",
			expected: metadata.Metadata{
				Title:        "ZDF.Magazin.Royale",
				Season:       2026,
				Episodes:     []int{166},
				Date:         "2026-05-29",
				EpisodeTitle: "Die.Colonius-Sprengung.ZMR.vor.Ort",
				Language:     "GERMAN",
				Resolution:   "1080p",
				Service:      "ZDF",
				Source:       "WEB-DL",
				VideoCodec:   "h264",
				Group:        "SLiDE",
				IsTV:         true,
			},
		},
		{
			// Issue 1: Spaces instead of just . as separators
			input: "Film Titel 2000 GERMAN 1080p ARD WEB-DL AAC2.0 H.264-GRP",
			expected: metadata.Metadata{
				Title:         "Film Titel",
				Year:          2000,
				Language:      "GERMAN",
				Resolution:    "1080p",
				Service:       "ARD",
				Source:        "WEB-DL",
				AudioCodec:    "AAC",
				AudioChannels: "2.0",
				VideoCodec:    "H.264",
				Group:         "GRP",
				IsTV:          false,
			},
		},
		{
			// Issue 2: don't match WEB-DL.anything.after.that as group DL.anything.after.that
			input: "Movie.2023.1080p.WEB-DL.Extra.stuff",
			expected: metadata.Metadata{
				Title:      "Movie",
				Year:       2023,
				Resolution: "1080p",
				Source:     "WEB-DL",
				Group:      "",
				IsTV:       false,
			},
		},
		{
			// Issue 3: match WEB without -DL or Rip as a source
			input: "Film.2024.1080p.WEB.AAC2.0.H.264-GRP",
			expected: metadata.Metadata{
				Title:         "Film",
				Year:          2024,
				Resolution:    "1080p",
				Source:        "WEB",
				AudioCodec:    "AAC",
				AudioChannels: "2.0",
				VideoCodec:    "H.264",
				Group:         "GRP",
				IsTV:          false,
			},
		},
		{
			// Issue 4: more other reasonable match options (DTS, TrueHD, etc.)
			input: "Movie.2023.2160p.WEB.TrueHD.7.1.Atmos.H.265-GRP",
			expected: metadata.Metadata{
				Title:         "Movie",
				Year:          2023,
				Resolution:    "2160p",
				Source:        "WEB",
				AudioCodec:    "TrueHD",
				AudioChannels: "7.1",
				AudioMeta:     "Atmos",
				VideoCodec:    "H.265",
				Group:         "GRP",
				IsTV:          false,
			},
		},
		{
			input: "Another.Movie.2024.4K.REMASTERED.1080p.DTS-HD.MA.5.1.AVC-GRP",
			expected: metadata.Metadata{
				Title:         "Another.Movie",
				Year:          2024,
				Resolution:    "1080p",
				CutEdition:    "4K.REMASTERED",
				AudioCodec:    "DTS-HD.MA",
				AudioChannels: "5.1",
				VideoCodec:    "AVC",
				Group:         "GRP",
				IsTV:          false,
			},
		},
		{
			input: "Show.S01E01.720p.HEVC.Opus.mkv",
			expected: metadata.Metadata{
				Title:      "Show",
				Season:     1,
				Episodes:   []int{1},
				Resolution: "720p",
				VideoCodec: "HEVC",
				AudioCodec: "Opus",
				IsTV:       true,
			},
		},
		{
			input: "Movie.2024.UHD.BluRay.REMUX.HEVC.DTS-HD.MA.5.1-GRP",
			expected: metadata.Metadata{
				Title:         "Movie",
				Year:          2024,
				Source:        "UHD.BluRay",
				VideoCodec:    "HEVC",
				AudioCodec:    "DTS-HD.MA",
				AudioChannels: "5.1",
				Group:         "GRP",
			},
		},
		{
			input: "Movie.2024.Blu-Ray.REMUX.1080p.AVC.DTS-HD.MA.5.1-GRP",
			expected: metadata.Metadata{
				Title:         "Movie",
				Year:          2024,
				Source:        "Blu-Ray",
				Resolution:    "1080p",
				VideoCodec:    "AVC",
				AudioCodec:    "DTS-HD.MA",
				AudioChannels: "5.1",
				Group:         "GRP",
			},
		},
		{
			input: "Classic.Movie.PAL.DVD.mkv",
			expected: metadata.Metadata{
				Title:  "Classic.Movie",
				Source: "PAL.DVD",
			},
		},
		{
			input: "Another.Classic.NTSC.DVD.mkv",
			expected: metadata.Metadata{
				Title:  "Another.Classic",
				Source: "NTSC.DVD",
			},
		},
		{
			input: "Movie.DVD5.mkv",
			expected: metadata.Metadata{
				Title:  "Movie",
				Source: "DVD5",
			},
		},
		{
			input: "Movie.DVD9.mkv",
			expected: metadata.Metadata{
				Title:  "Movie",
				Source: "DVD9",
			},
		},
		{
			input: "Movie.PAL.DVD9.mkv",
			expected: metadata.Metadata{
				Title:  "Movie",
				Source: "PAL.DVD9",
			},
		},
		// From TestMissingYear
		{
			input: "Film.Titel.GERMAN.1080p.ARD.WEB-DL.AAC2.0.H.264-GRP.mkv",
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
			input: "Simple.Movie.1080p.x264-GRP",
			expected: metadata.Metadata{
				Title:      "Simple.Movie",
				Resolution: "1080p",
				VideoCodec: "x264",
				Group:      "GRP",
			},
		},
		{
			input: "Repack.Movie.REPACK.720p.WEB-DL.AAC2.0.x264-GRP",
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
			input: "Repack-end.Movie.1080p.BluRay.x264.REPACK-GRP",
			expected: metadata.Metadata{
				Title:      "Repack-end.Movie",
				Repack:     true,
				Resolution: "1080p",
				Source:     "BluRay",
				VideoCodec: "x264",
				Group:      "GRP",
			},
		},
		{
			input: "[Group] Anime Name - S01E01 - (BD 1080p HEVC FLAC) [Dual Audio] [48F1910E].mkv",
			expected: metadata.Metadata{
				Title:        "Anime Name",
				Season:       1,
				Episodes:     []int{1},
				EpisodeTitle: "- (BD",
				Resolution:   "1080p",
				Source:       "BD",
				VideoCodec:   "HEVC",
				AudioCodec:   "FLAC",
				DualAudio:    true,
				CRC32:        "48F1910E",
				Group:        "Group",
				IsTV:         true,
			},
		},
		{
			input: "Anime Name - S01E01 - (BD 1080p HEVC FLAC) [Dual Audio] [48F1910E]-Group.mkv",
			expected: metadata.Metadata{
				Title:        "Anime Name",
				Season:       1,
				Episodes:     []int{1},
				EpisodeTitle: "- (BD",
				Resolution:   "1080p",
				Source:       "BD",
				VideoCodec:   "HEVC",
				AudioCodec:   "FLAC",
				DualAudio:    true,
				CRC32:        "48F1910E",
				Group:        "Group.mkv",
				IsTV:         true,
			},
		},
		{
			input: "Anime.Name.S01E01.1080p.BluRay.Opus2.0.x264-Hi10P-Group.mkv",
			expected: metadata.Metadata{
				Title:         "Anime.Name",
				Season:        1,
				Episodes:      []int{1},
				Resolution:    "1080p",
				Source:        "BluRay",
				AudioCodec:    "Opus",
				AudioChannels: "2.0",
				VideoCodec:    "x264",
				Group:         "Group.mkv",
				IsTV:          true,
			},
		},
		{
			input: "Kaeptn.Blaubaers.Seemannsgarn.S01E01-E06.Wie.das.Schiff.zur.Klippe.kam.uvm.GERMAN.1080p.ATV.WEB-DL.h264-SLiDE",
			expected: metadata.Metadata{
				Title:        "Kaeptn.Blaubaers.Seemannsgarn",
				Season:       1,
				Episodes:     []int{1, 2, 3, 4, 5, 6},
				EpisodeTitle: "Wie.das.Schiff.zur.Klippe.kam.uvm",
				Language:     "GERMAN",
				Resolution:   "1080p",
				Service:      "ATV",
				Source:       "WEB-DL",
				VideoCodec:   "h264",
				Group:        "SLiDE",
				IsTV:         true,
			},
		},
		{
			input: "ShowName.S02E01-E03.Episode.Title.Here.ENGLISH.720p.WEB-DL.x264-Group",
			expected: metadata.Metadata{
				Title:        "ShowName",
				Season:       2,
				Episodes:     []int{1, 2, 3},
				EpisodeTitle: "Episode.Title.Here",
				Language:     "ENGLISH",
				Resolution:   "720p",
				Source:       "WEB-DL",
				VideoCodec:   "x264",
				Group:        "Group",
				IsTV:         true,
			},
		},
	}

	runTableTest(t, tests, func(s string) metadata.Metadata {
		return *Parse(s)
	}, compareMetadata)
}

func TestDeobfuscateTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"Gloeckner", "Glöckner"},
		{"Ueber", "Über"},
		{"neue", "neue"},
		{"Aerzte", "Ärzte"},
		{"Koeln", "Köln"},
		{"Muenchen", "München"},
		{"Baeume", "Bäume"},
		{"Modehaeuser", "Modehäuser"},
		{"Bloede Buehnenduesen", "Blöde Bühnendüsen"},
	}

	runTableTest(t, tests, DeobfuscateTitle, func(got, want string) string {
		if got != want {
			return fmt.Sprintf("got %q, want %q", got, want)
		}

		return ""
	})
}

//nolint:paralleltest // mutates global state via config.InitDefaults()
func TestNormalizeTitle(t *testing.T) {
	config.InitDefaults()

	tests := []struct {
		input    string
		expected string
	}{
		{"München", "Muenchen"},
		{"Blöde Bühnendüsen", "Bloede Buehnenduesen"},
		{"Film & Dokumentation", "Film und Dokumentation"},
		{"Test...Sequence.-..Fix", "Test Sequence Fix"},
		{"Café.Smørebrød", "Cafe.Smoerebroed"},
		{"Title with (parentheses) and \"quotes\"", "Title with parentheses and quotes"},
	}

	runTableTest(t, tests, NormalizeTitle, func(got, want string) string {
		if got != want {
			return fmt.Sprintf("got %q, want %q", got, want)
		}

		return ""
	})
}

func TestNormalizeService(t *testing.T) {
	t.Parallel()

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

	runTableTest(t, tests, NormalizeService, func(got, want string) string {
		if got != want {
			return fmt.Sprintf("got %q, want %q", got, want)
		}

		return ""
	})
}
