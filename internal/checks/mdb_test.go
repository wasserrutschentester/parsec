package checks

import (
	"strings"
	"testing"

	"github.com/spf13/viper"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/mdb"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/mediainfo"
)

func audioMediaInfo(langs ...string) *mediainfo.MediaInfo {
	tracks := make([]mediainfo.Track, 0, len(langs))
	for _, lang := range langs {
		tracks = append(tracks, mediainfo.Track{Type: "Audio", Language: lang})
	}

	return &mediainfo.MediaInfo{Media: mediainfo.Media{Tracks: tracks}}
}

//nolint:paralleltest // depends on shared global config state
func TestCheckUnwantedAudioLang(t *testing.T) {
	config.InitDefaults() // preferred language is "de"

	tests := []struct {
		name         string
		audioLangs   []string
		originalLang string
		wantUnwanted bool
	}{
		{"only preferred", []string{"ger"}, "ja", false},
		{"preferred and original", []string{"ger", "jpn"}, "ja", false},
		{"foreign track flagged", []string{"ger", "jpn", "fre"}, "ja", true},
		{"zxx is never unwanted", []string{"ger", "zxx"}, "ja", false},
		{"und and mul kept", []string{"ger", "und", "mul"}, "ja", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mi := audioMediaInfo(tt.audioLangs...)
			result := &mdb.SearchResult{OriginalLanguage: tt.originalLang}

			results := checkUnwantedAudioLang(mi, result)
			if got := len(results) > 0; got != tt.wantUnwanted {
				t.Fatalf("checkUnwantedAudioLang() unwanted=%v, want %v (results=%+v)", got, tt.wantUnwanted, results)
			}
		})
	}
}

//nolint:paralleltest // depends on shared global config state
func TestCheckUnwantedAudioLangKeepsZxxAlongsideForeign(t *testing.T) {
	config.InitDefaults() // preferred language is "de"

	mi := audioMediaInfo("ger", "fre", "zxx")
	results := checkUnwantedAudioLang(mi, &mdb.SearchResult{OriginalLanguage: "ja"})

	if len(results) != 1 {
		t.Fatalf("expected one unwanted-language result, got %+v", results)
	}

	// The French track is unwanted, but the no-dialogue 'zxx' track must not be.
	if strings.Contains(results[0].Warning, "zxx") {
		t.Errorf("zxx must not be reported as unwanted: %q", results[0].Warning)
	}
}

//nolint:paralleltest // depends on shared global config state
func TestCheckTrackLanguagesSilentFilm(t *testing.T) {
	config.InitDefaults() // preferred language is "de"

	// MediaInfo with only subtitles in German, no audio tracks
	mi := &mediainfo.MediaInfo{
		Media: mediainfo.Media{
			Tracks: []mediainfo.Track{
				{Type: "Text", Language: "ger"},
			},
		},
	}

	// Case 1: Silent film (e.g. Metropolis: origLang "de", SpokenLanguages has "zxx")
	resSilent := &mdb.SearchResult{
		OriginalLanguage: "de",
		SpokenLanguages:  []string{"de", "zxx"},
	}
	results1 := checkTrackLanguages(mi, resSilent)

	for _, r := range results1 {
		if strings.Contains(r.Identifier, "audio") && !r.Passed {
			t.Errorf("silent film should not fail audio track checks, got: %+v", r)
		}
	}

	// Case 2: Non-silent film (e.g. German talkie: origLang "de", SpokenLanguages has "de")
	resTalkie := &mdb.SearchResult{
		OriginalLanguage: "de",
		SpokenLanguages:  []string{"de"},
	}
	results2 := checkTrackLanguages(mi, resTalkie)

	hasAudioFailure := false

	for _, r := range results2 {
		if strings.Contains(r.Identifier, "audio") && !r.Passed {
			hasAudioFailure = true
		}
	}

	if !hasAudioFailure {
		t.Errorf("talkie missing audio track should fail audio check")
	}
}

//nolint:paralleltest // depends on shared global state
func TestCheckTitle(t *testing.T) {
	tests := []struct {
		name      string
		metaTitle string
		resTitle  string
		wantWarn  bool
	}{
		{"Match", "Movie Title", "Movie Title", false},
		{"Mismatch", "Movie Title", "Different Title", true},
		{"Normalization Match", "Movie.Title", "Movie Title", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := &metadata.Metadata{Title: tt.metaTitle}
			res := &mdb.SearchResult{Title: tt.resTitle}
			results := checkTitle(meta, res)
			hasFailure := false

			for _, r := range results {
				if !r.Passed {
					hasFailure = true

					break
				}
			}

			if tt.wantWarn && !hasFailure {
				t.Errorf("checkTitle() expected warnings, got none")
			}

			if !tt.wantWarn && hasFailure {
				t.Errorf("checkTitle() expected no warnings, got failure")
			}
		})
	}
}

//nolint:paralleltest // depends on shared global state
func TestCheckMovieYear(t *testing.T) {
	tests := []struct {
		name     string
		metaYear int
		resYear  int
		wantWarn bool
	}{
		{"Match", 2023, 2023, false},
		{"Mismatch", 2023, 2022, true},
		{"Meta Zero", 0, 2023, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := &metadata.Metadata{Year: tt.metaYear, IsTV: false}
			res := &mdb.SearchResult{Year: tt.resYear}
			results := checkMovieYear(meta, res)
			hasFailure := false

			for _, r := range results {
				if !r.Passed {
					hasFailure = true

					break
				}
			}

			if tt.wantWarn && !hasFailure {
				t.Errorf("checkMovieYear() expected warnings, got none")
			}

			if !tt.wantWarn && hasFailure {
				t.Errorf("checkMovieYear() expected no warnings, got failure")
			}
		})
	}
}

//nolint:cyclop,funlen,paralleltest // depends on shared global state
func TestCheckRuntime(t *testing.T) {
	config.InitDefaults()
	viper.Set("enabled_checks", []string{"all"})

	dur45Min := float64(45 * 60)
	dur10Min := float64(10 * 60)
	mi45 := &mediainfo.MediaInfo{
		Media: mediainfo.Media{
			Tracks: []mediainfo.Track{{Type: "Video", Duration: &dur45Min}},
		},
	}
	mi10 := &mediainfo.MediaInfo{
		Media: mediainfo.Media{
			Tracks: []mediainfo.Track{{Type: "Video", Duration: &dur10Min}},
		},
	}

	meta := &metadata.Metadata{IsTV: false}
	searchRes := &mdb.SearchResult{Runtime: 45}

	// Case 1: Runtime match
	resMatch := checkRuntime(mi45, meta, searchRes)
	if len(resMatch) == 0 || !resMatch[0].Passed {
		t.Fatalf("expected runtime match, got: %+v", resMatch)
	}

	// Case 2: Moderate runtime mismatch (10% - 40%) -> warning
	dur38Min := float64(38 * 60)
	mi38 := &mediainfo.MediaInfo{
		Media: mediainfo.Media{
			Tracks: []mediainfo.Track{{Type: "Video", Duration: &dur38Min}},
		},
	}

	resModerate := checkRuntime(mi38, meta, searchRes)
	if len(resModerate) == 0 || resModerate[0].Passed {
		t.Fatalf("expected moderate runtime mismatch failure, got: %+v", resModerate)
	}

	if resModerate[0].Severity != "warning" {
		t.Errorf("expected Severity = 'warning', got %q", resModerate[0].Severity)
	}

	if resModerate[0].Actual != "38.0 min (-15.6%)" {
		t.Errorf("expected Actual = '38.0 min (-15.6%%%%)', got %q", resModerate[0].Actual)
	}

	// Case 3: Significant runtime mismatch (> 40%) -> error
	resSignificant := checkRuntime(mi10, meta, searchRes)
	if len(resSignificant) == 0 || resSignificant[0].Passed {
		t.Fatalf("expected significant runtime mismatch failure, got: %+v", resSignificant)
	}

	if resSignificant[0].Severity != "error" {
		t.Errorf("expected Severity = 'error', got %q", resSignificant[0].Severity)
	}

	if resSignificant[0].Warning != "Significant Runtime Mismatch" {
		t.Errorf("expected Warning = 'Significant Runtime Mismatch', got %q", resSignificant[0].Warning)
	}

	if resSignificant[0].Expected != "45.0 min" {
		t.Errorf("expected Expected = '45.0 min', got %q", resSignificant[0].Expected)
	}

	if resSignificant[0].Actual != "10.0 min (-77.8%)" {
		t.Errorf("expected Actual = '10.0 min (-77.8%%%%)', got %q", resSignificant[0].Actual)
	}
}

//nolint:paralleltest // depends on shared global state
func TestCheckEpisodeExistence(t *testing.T) {
	config.InitDefaults()
	viper.Set("enabled_checks", []string{"all"})

	meta := &metadata.Metadata{
		IsTV:     true,
		Season:   1,
		Episodes: []int{99},
	}
	searchRes := &mdb.SearchResult{
		Title: "Test Show",
	}

	results := checkEpisode(meta, searchRes)
	if len(results) == 0 || results[0].Passed {
		t.Fatalf("expected episode existence check failure, got: %+v", results)
	}

	r := results[0]
	if r.Warning != "Episode not found on TVDB/TMDB" {
		t.Errorf("expected Warning = 'Episode not found on TVDB/TMDB', got %q", r.Warning)
	}

	if r.Expected != "Episode exists on TVDB/TMDB" {
		t.Errorf("expected Expected = 'Episode exists on TVDB/TMDB', got %q", r.Expected)
	}

	if r.Actual != "S01E[99]" {
		t.Errorf("expected Actual = 'S01E[99]', got %q", r.Actual)
	}
}

func TestCheckTitleTypeMismatch_Movie(t *testing.T) {
	t.Parallel()

	assertTitleTypeMismatch(t, false, mdb.TitleTypeUnknown, true, "", "", "")
	assertTitleTypeMismatch(t, false, mdb.TitleTypeMovie, true, "", "", "")
	assertTitleTypeMismatch(t, false, mdb.TitleTypeTVMovie, true, "", "", "")
	assertTitleTypeMismatch(t, false, mdb.TitleTypeTVEpisode, false, "Title Type Mismatch", "Movie", "TV Episode")
	assertTitleTypeMismatch(t, false, mdb.TitleTypeTVSeries, false, "Title Type Mismatch", "Movie", "TV Series")
	assertTitleTypeMismatch(t, false, mdb.TitleTypeTVMiniSeries, false, "Title Type Mismatch", "Movie", "Mini-Series")
}

func TestCheckTitleTypeMismatch_TV(t *testing.T) {
	t.Parallel()

	assertTitleTypeMismatch(t, true, mdb.TitleTypeTVSeries, true, "", "", "")
	assertTitleTypeMismatch(t, true, mdb.TitleTypeTVMiniSeries, true, "", "", "")
	assertTitleTypeMismatch(t, true, mdb.TitleTypeMovie, false, "Title Type Mismatch", "TV Series", "Movie")
	assertTitleTypeMismatch(t, true, mdb.TitleTypeTVMovie, false, "Title Type Mismatch", "TV Series", "TV Movie")
}

func assertTitleTypeMismatch(t *testing.T, isTV bool, titleType mdb.TitleType, wantPassed bool, wantWarn, wantExp, wantAct string) {
	t.Helper()

	meta := &metadata.Metadata{IsTV: isTV}
	searchRes := &mdb.SearchResult{TitleType: titleType}

	results := checkTitleTypeMismatch(meta, searchRes)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.Passed != wantPassed {
		t.Errorf("expected Passed = %v, got %v", wantPassed, r.Passed)
	}

	if !wantPassed {
		if r.Warning != wantWarn {
			t.Errorf("expected Warning = %q, got %q", wantWarn, r.Warning)
		}

		if r.Expected != wantExp {
			t.Errorf("expected Expected = %q, got %q", wantExp, r.Expected)
		}

		if r.Actual != wantAct {
			t.Errorf("expected Actual = %q, got %q", wantAct, r.Actual)
		}
	}
}
