package checks

import (
	"fmt"
	"strings"

	"codeberg.org/n0ne/parsec/internal/config"
	"codeberg.org/n0ne/parsec/internal/mdb"
	mdbSearch "codeberg.org/n0ne/parsec/internal/mdb/search"
	"codeberg.org/n0ne/parsec/internal/metadata"
	"codeberg.org/n0ne/parsec/internal/metadata/filename"
	"codeberg.org/n0ne/parsec/internal/metadata/mediainfo"
	"codeberg.org/n0ne/parsec/internal/ui"
	"golang.org/x/text/language"
)

func RunMdbChecks(mi *mediainfo.MediaInfo, meta *metadata.Metadata) []CheckResult {
	var results []CheckResult
	searchResult, searchErr := mdbSearch.InteractiveSearch(meta, true)

	if searchErr != nil {
		results = append(results, CheckResult{
			Identifier:  "mdb_error",
			Description: "Error searching MDB",
			Passed:      false,
			Severity:    "error",
			Warning:     fmt.Sprintf("MDB Error: %v", searchErr),
		})
		return results
	}
	if searchResult == nil {
		return []CheckResult{{
			Identifier:  "mdb_no_match",
			Description: "Matching metadata on TMDB/TVDB",
			Passed:      false,
			Severity:    "warning",
			Warning:     "No matching metadata found on TMDB/TVDB.",
		}}
	}

	if config.IsCheckEnabled("mdb_title") {
		results = append(results, CheckTitle(meta, searchResult)...)
	}
	if config.IsCheckEnabled("mdb_movie_year") {
		results = append(results, CheckMovieYear(meta, searchResult)...)
	}
	if meta.IsTV && config.IsCheckEnabled("mdb_series_year") {
		results = append(results, CheckSeriesYear(meta, searchResult)...)
	}
	if meta.IsTV {
		results = append(results, CheckEpisode(meta, searchResult)...)
	}

	if mi != nil && config.IsCheckEnabled("mdb_track_languages") {
		results = append(results, CheckTrackLanguages(mi, searchResult)...)
	}
	return results
}

func CheckTrackLanguages(mi *mediainfo.MediaInfo, result *mdb.SearchResult) []CheckResult {
	var results []CheckResult
	prefLang := config.GetPreferredLanguage()
	origLang := result.OriginalLanguage

	audioLangs := mi.GetAudioLanguages()
	subLangs := mi.GetSubtitleLanguages()

	prefTag := language.Make(prefLang)
	origTag := language.Make(origLang)

	check := func(trackType string, langs []string, targetTag language.Tag, targetStr string, label string) {
		if targetStr == "" {
			return
		}
		found := false
		for _, l := range langs {
			if language.Make(l) == targetTag {
				found = true
				break
			}
		}

		res := CheckResult{
			Identifier:  fmt.Sprintf("mdb_%s_language_%s", strings.ToLower(trackType), label),
			Description: fmt.Sprintf("%s track in %s language", trackType, label),
			Passed:      true,
			Expected:    targetStr,
		}

		if !found {
			res.Passed = false
			res.Severity = "warning"
			res.Warning = fmt.Sprintf("%s track in %s language '%s' is missing.", trackType, label, targetStr)
		}
		results = append(results, res)
	}

	check("Audio", audioLangs, prefTag, prefLang, "preferred")
	check("Subtitle", subLangs, prefTag, prefLang, "preferred")

	if origLang != "" && origTag != prefTag {
		check("Audio", audioLangs, origTag, origLang, "original")
		check("Subtitle", subLangs, origTag, origLang, "original")
	}
	return results
}

func CheckMovieYear(meta *metadata.Metadata, result *mdb.SearchResult) []CheckResult {
	res := CheckResult{
		Identifier:  "mdb_movie_year",
		Description: "Movie Year matches MDB",
		Passed:      true,
	}
	if !meta.IsTV && meta.Year > 0 && result.Year > 0 {
		res.Expected = fmt.Sprintf("%d", result.Year)
		res.Actual = fmt.Sprintf("%d", meta.Year)
		if meta.Year != result.Year {
			res.Passed = false
			res.Severity = "warning"
			res.Warning = ui.FormatDiff("MDB Year", res.Expected, "File Year", res.Actual)
		}
	}
	return []CheckResult{res}
}

func CheckSeriesYear(meta *metadata.Metadata, result *mdb.SearchResult) []CheckResult {
	res := CheckResult{
		Identifier:  "mdb_series_year",
		Description: "Series Start Year matches MDB",
		Passed:      true,
	}
	if meta.Year > 0 && result.Year > 0 && meta.Season < 1900 {
		res.Expected = fmt.Sprintf("%d", result.Year)
		res.Actual = fmt.Sprintf("%d", meta.Year)
		if meta.Year != result.Year {
			res.Passed = false
			res.Severity = "warning"
			res.Warning = ui.FormatDiff("MDB Start Year", res.Expected, "File Year", res.Actual)
		}
	}
	return []CheckResult{res}
}

func CheckEpisode(meta *metadata.Metadata, result *mdb.SearchResult) []CheckResult {
	var results []CheckResult
	if meta.Season > 0 || meta.Episode > 0 {
		epResult := mdbSearch.FindEpisode(*result, meta.Season, meta.Episode)

		existenceCheck := CheckResult{
			Identifier:  "mdb_episode_existence",
			Description: "Episode exists on TVDB/TMDB",
			Passed:      true,
		}

		if epResult.Name == "" {
			if config.IsCheckEnabled("mdb_episode_existence") {
				existenceCheck.Passed = false
				existenceCheck.Severity = "warning"
				existenceCheck.Warning = fmt.Sprintf("Episode S%02dE%02d not found on TVDB/TMDB.", meta.Season, meta.Episode)
				results = append(results, existenceCheck)
			}
		} else {
			results = append(results, existenceCheck)
			if config.IsCheckEnabled("mdb_episode_title") {
				results = append(results, CheckEpisodeTitle(meta, epResult)...)
			}
			if config.IsCheckEnabled("mdb_episode_date") {
				results = append(results, CheckSpecialDate(meta, epResult)...)
			}
		}
	}
	return results
}

func CheckEpisodeTitle(meta *metadata.Metadata, epResult mdb.EpisodeResult) []CheckResult {
	res := CheckResult{
		Identifier:  "mdb_episode_title",
		Description: "Episode Title matches MDB",
		Passed:      true,
	}
	if meta.EpisodeTitle != "" {
		res.Expected = epResult.Name
		res.Actual = meta.EpisodeTitle
		normParsed := NormalizeForComparison(filename.DeobfuscateTitle(meta.EpisodeTitle))
		normOfficial := NormalizeForComparison(epResult.Name)
		if normParsed != normOfficial {
			res.Passed = false
			res.Severity = "warning"
			res.Warning = ui.FormatDiff("Official Title", res.Expected, "Parsed Title", res.Actual)
		}
	}
	return []CheckResult{res}
}

func CheckTitle(meta *metadata.Metadata, result *mdb.SearchResult) []CheckResult {
	res := CheckResult{
		Identifier:  "mdb_title",
		Description: "Title matches MDB",
		Passed:      true,
	}
	if meta.Title != "" {
		res.Expected = result.Title
		res.Actual = meta.Title
		normParsed := NormalizeForComparison(filename.DeobfuscateTitle(meta.Title))
		normOfficial := NormalizeForComparison(result.Title)
		if normParsed != normOfficial {
			res.Passed = false
			res.Severity = "warning"
			res.Warning = ui.FormatDiff("Official Title", res.Expected, "Parsed Title", res.Actual)
		}
	}
	return []CheckResult{res}
}

func CheckSpecialDate(meta *metadata.Metadata, epResult mdb.EpisodeResult) []CheckResult {
	res := CheckResult{
		Identifier:  "mdb_episode_date",
		Description: "Episode Airdate matches MDB",
		Passed:      true,
	}
	if meta.Season == 0 && meta.Date != "" {
		res.Expected = epResult.Airdate
		res.Actual = meta.Date
		if epResult.Airdate != meta.Date {
			res.Passed = false
			res.Severity = "warning"
			res.Warning = ui.FormatDiff("Official Date", res.Expected, "File Date", res.Actual)
		}
	}
	return []CheckResult{res}
}
