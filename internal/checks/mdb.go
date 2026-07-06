package checks

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/text/language"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/mdb"
	mdbSearch "codeberg.org/upPollo/parsec/internal/mdb/search"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/filename"
	"codeberg.org/upPollo/parsec/internal/metadata/mediainfo"
)

// RunMdbChecks performs checks against online media databases (TMDB/TVDB).
//
//nolint:cyclop // multi-step search and verification process against external databases requires many conditional paths
func RunMdbChecks(mi *mediainfo.MediaInfo, meta *metadata.Metadata) []CheckResult {
	var results []CheckResult

	searchResult, searchErr := mdbSearch.InteractiveSearch(meta, true)
	if searchErr != nil {
		return checkMdbError(searchErr)
	}

	if searchResult == nil {
		return checkNoMatch()
	}

	// Propagate discovered IDs back to the metadata object so they can be
	// used by downstream checks (e.g., season completeness grouping)
	if meta.TmdbID == 0 && searchResult.TmdbID > 0 {
		meta.TmdbID = searchResult.TmdbID
	}

	if meta.TvdbID == 0 && searchResult.TvdbID > 0 {
		meta.TvdbID = searchResult.TvdbID
	}

	if meta.ImdbID == "" && searchResult.ImdbID != "" {
		meta.ImdbID = searchResult.ImdbID
	}

	if origLangOverride := config.GetOriginalLanguage(); origLangOverride != "" {
		searchResult.OriginalLanguage = origLangOverride
	}

	mdb.PrintCompactResult(*searchResult)

	if config.IsCheckEnabled(config.CheckMdbUnknownOrigLang) {
		results = append(results, checkUnknownOriginalLang(searchResult)...)
	}

	if config.IsCheckEnabled(config.CheckMdbTitle) {
		results = append(results, checkTitle(meta, searchResult)...)
	}

	if config.IsCheckEnabled(config.CheckMdbMovieYear) {
		results = append(results, checkMovieYear(meta, searchResult)...)
	}

	if meta.IsTV && config.IsCheckEnabled(config.CheckMdbSeriesYear) {
		results = append(results, checkSeriesYear(meta, searchResult)...)
	}

	if meta.IsTV {
		results = append(results, checkEpisode(meta, searchResult)...)
	}

	if mi != nil && config.IsCheckEnabled(config.CheckMdbTrackLanguages) {
		results = append(results, checkTrackLanguages(mi, searchResult)...)
	}

	if mi != nil && config.IsCheckEnabled(config.CheckMdbUnwantedAudioLang) {
		results = append(results, checkUnwantedAudioLang(mi, searchResult)...)
	}

	return results
}

func checkMdbError(err error) []CheckResult {
	return []CheckResult{{
		Identifier: "mdb_error",
		Passed:     false,
		Severity:   "error",
		Warning:    fmt.Sprintf("MDB Error: %v", err),
	}}
}

func checkNoMatch() []CheckResult {
	return []CheckResult{{
		Identifier: "mdb_no_match",
		Passed:     false,
		Severity:   "warning",
		Warning:    "No matching metadata found on TMDB/TVDB.",
	}}
}

func checkTrackLanguages(mi *mediainfo.MediaInfo, result *mdb.SearchResult) []CheckResult {
	var results []CheckResult

	prefLang := config.GetPreferredLanguage()
	origLang := result.OriginalLanguage

	audioLangs := mi.GetAudioLanguages()
	subLangs := mi.GetSubtitleLanguages()

	prefTag := language.Make(prefLang)
	origTag := language.Make(origLang)

	check := func(trackType string, langs []string, targetTag language.Tag, targetStr, label string) {
		if targetStr == "" {
			return
		}

		found := false

		for _, l := range langs {
			if metadata.MatchLanguage(language.Make(l), targetTag) {
				found = true

				break
			}
		}

		res := CheckResult{
			Identifier: fmt.Sprintf("mdb_%s_language_%s", strings.ToLower(trackType), label),
			Passed:     true,
			Expected:   targetStr,
		}

		if !found {
			res.Passed = false
			res.Severity = "warning"
			res.Warning = fmt.Sprintf("%s track in %s language '%s' is missing", trackType, label, targetStr)
		}

		results = append(results, res)
	}

	check("Audio", audioLangs, prefTag, prefLang, "preferred")
	check("Subtitle", subLangs, prefTag, prefLang, "preferred")

	if origLang != "" && !metadata.MatchLanguage(origTag, prefTag) {
		check("Audio", audioLangs, origTag, origLang, "original")
		check("Subtitle", subLangs, origTag, origLang, "original")
	}

	return results
}

func checkUnknownOriginalLang(result *mdb.SearchResult) []CheckResult {
	var results []CheckResult

	origLang := result.OriginalLanguage

	origTag := language.Make(origLang)
	if origLang == "" || origTag == language.Und {
		results = append(results, CheckResult{
			Identifier: config.CheckMdbUnknownOrigLang,
			Passed:     false,
			Severity:   "info",
			Warning:    fmt.Sprintf("original language '%s' is not recognized or missing from TMDB/TVDB", origLang),
			Actual:     origLang,
		})
	}

	return results
}

// IsWantedAudioLang reports whether an audio track's language tag should be
// kept, given the preferred and MDB original language tags. Matching is by
// base ISO 639 subtag (via metadata.MatchLanguage), so e.g. a track tagged
// "de-DE" is wanted when the preferred language is "de". This is the single
// source of truth for wanted/unwanted audio language classification, shared
// by the mdb_unwanted_audio_lang check and internal/correct's remux pruning
// so the two never disagree on which tracks are safe to remove.
func IsWantedAudioLang(langTag, prefTag, origTag language.Tag) bool {
	mulTag := language.Make("mul")
	zxxTag := language.Make("zxx")

	switch {
	case langTag == language.Und || metadata.MatchLanguage(langTag, mulTag):
		return true
	case metadata.MatchLanguage(langTag, zxxTag):
		// no linguistic content (e.g. music-only); never unwanted
		return true
	case metadata.MatchLanguage(langTag, prefTag):
		return true
	case origTag != language.Und && metadata.MatchLanguage(langTag, origTag):
		return true
	default:
		return false
	}
}

func checkUnwantedAudioLang(mi *mediainfo.MediaInfo, result *mdb.SearchResult) []CheckResult {
	var results []CheckResult

	prefLang := config.GetPreferredLanguage()
	origLang := result.OriginalLanguage
	audioLangs := mi.GetAudioLanguages()

	unwantedLangs := []language.Tag{}
	prefTag := language.Make(prefLang)
	origTag := language.Make(origLang)

	for _, lang := range audioLangs {
		langTag := language.Make(lang)

		if !IsWantedAudioLang(langTag, prefTag, origTag) {
			unwantedLangs = append(unwantedLangs, langTag)
		}
	}

	unwantedLangs = metadata.RemoveDuplicates(unwantedLangs)
	if len(unwantedLangs) > 0 {
		results = append(results, CheckResult{
			Identifier: config.CheckMdbUnwantedAudioLang,
			Passed:     false,
			Severity:   "warning",
			Warning:    fmt.Sprintf("Has unwanted audio language track(s): %s", unwantedLangs),
		})
	}

	return results
}

func checkMovieYear(meta *metadata.Metadata, result *mdb.SearchResult) []CheckResult {
	res := CheckResult{
		Identifier: config.CheckMdbMovieYear,
		Passed:     true,
	}
	if !meta.IsTV && meta.Year > 0 && result.Year > 0 {
		res.Expected = strconv.Itoa(result.Year)

		res.Actual = strconv.Itoa(meta.Year)
		if meta.Year != result.Year {
			res.Passed = false
			res.Severity = "warning"
			res.Warning = "Year Mismatch"
		}
	}

	return []CheckResult{res}
}

func checkSeriesYear(meta *metadata.Metadata, result *mdb.SearchResult) []CheckResult {
	res := CheckResult{
		Identifier: config.CheckMdbSeriesYear,
		Passed:     true,
	}
	if meta.Year > 0 && result.Year > 0 && meta.Season < 1900 {
		res.Expected = strconv.Itoa(result.Year)

		res.Actual = strconv.Itoa(meta.Year)
		if meta.Year != result.Year {
			res.Passed = false
			res.Severity = "warning"
			res.Warning = "Year Mismatch"
		}
	}

	return []CheckResult{res}
}

func checkEpisode(meta *metadata.Metadata, result *mdb.SearchResult) []CheckResult {
	if meta.Season == 0 && len(meta.Episodes) == 0 {
		return nil
	}

	var results []CheckResult

	episodes := mdbSearch.FindEpisodes(*result, meta, false)

	existenceCheck := CheckResult{
		Identifier: config.CheckMdbEpisodeExistence,
		Passed:     true,
	}

	if len(episodes) == 0 {
		if config.IsCheckEnabled(config.CheckMdbEpisodeExistence) {
			existenceCheck.Passed = false
			existenceCheck.Severity = "warning"
			existenceCheck.Warning = fmt.Sprintf("Episode S%02dE%v not found on TVDB/TMDB.", meta.Season, meta.Episodes)
			results = append(results, existenceCheck)
		}

		return results
	}

	// build combined dummy episode result for checks
	epResult := mdb.EpisodeResult{
		Name:    mdb.CombineEpisodeNames(episodes),
		Airdate: episodes[0].Airdate,
		Season:  episodes[0].Season,
		Episode: episodes[0].Episode,
	}

	results = append(results, existenceCheck)
	if config.IsCheckEnabled(config.CheckMdbEpisodeTitle) {
		results = append(results, checkEpisodeTitle(meta, epResult)...)
	}

	if config.IsCheckEnabled(config.CheckMdbEpisodeDate) {
		results = append(results, checkSpecialDate(meta, epResult)...)
	}

	return results
}

func checkEpisodeTitle(meta *metadata.Metadata, epResult mdb.EpisodeResult) []CheckResult {
	res := CheckResult{
		Identifier: config.CheckMdbEpisodeTitle,
		Passed:     true,
	}
	if len(meta.EpisodeTitles) > 0 {
		res.Expected = epResult.Name
		res.Actual = strings.Join(meta.EpisodeTitles, " / ")
		normParsed := normalizeForComparison(filename.DeobfuscateTitle(res.Actual))

		normOfficial := normalizeForComparison(filename.ApplyTitleReplacements(epResult.Name))
		if normParsed != normOfficial {
			res.Passed = false
			res.Severity = "warning"
			res.Warning = "Title Mismatch"
		}
	}

	return []CheckResult{res}
}

func checkTitle(meta *metadata.Metadata, result *mdb.SearchResult) []CheckResult {
	res := CheckResult{
		Identifier: config.CheckMdbTitle,
		Passed:     true,
	}
	if meta.Title != "" {
		res.Expected = result.Title
		res.Actual = meta.Title
		normParsed := normalizeForComparison(filename.DeobfuscateTitle(meta.Title))

		normOfficial := normalizeForComparison(filename.ApplyTitleReplacements(result.Title))
		if normParsed != normOfficial {
			res.Passed = false
			res.Severity = "warning"
			res.Warning = "Title Mismatch"
		}
	}

	return []CheckResult{res}
}

func checkSpecialDate(meta *metadata.Metadata, epResult mdb.EpisodeResult) []CheckResult {
	res := CheckResult{
		Identifier: config.CheckMdbEpisodeDate,
		Passed:     true,
	}
	if meta.Season == 0 && meta.Date != "" {
		res.Expected = epResult.Airdate

		res.Actual = meta.Date
		if epResult.Airdate != meta.Date {
			res.Passed = false
			res.Severity = "warning"
			res.Warning = "Date Mismatch"
		}
	}

	return []CheckResult{res}
}

// RunSeasonCompletenessCheck checks if all episodes listed on TVDB/TMDB for a given season are present.
func RunSeasonCompletenessCheck(result *mdb.SearchResult, season int, presentEpisodes []int) []CheckResult {
	if result == nil || season <= 0 {
		return nil
	}

	officialEpisodes, err := mdbSearch.GetSeasonEpisodes(*result, season)
	if err != nil {
		return []CheckResult{{
			Identifier: "mdb_season_completeness_error",
			Passed:     false,
			Severity:   "warning",
			Warning:    fmt.Sprintf("Could not fetch official episode list for Season %d: %v", season, err),
		}}
	}

	presentMap := make(map[int]bool)
	for _, e := range presentEpisodes {
		presentMap[e] = true
	}

	var missing []int

	for _, ep := range officialEpisodes {
		if !presentMap[ep.Episode] {
			missing = append(missing, ep.Episode)
		}
	}

	res := CheckResult{
		Identifier: "mdb_season_completeness",
		Passed:     len(missing) == 0,
		Expected:   strconv.Itoa(len(officialEpisodes)),
		Actual:     strconv.Itoa(len(presentEpisodes)),
	}

	if !res.Passed {
		res.Severity = "warning"
		res.Warning = fmt.Sprintf("Season %d is incomplete. Missing episodes: %v", season, missing)
	}

	return []CheckResult{res}
}
