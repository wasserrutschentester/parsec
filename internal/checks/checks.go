package checks

import (
	"fmt"
	"regexp"
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

type CheckResult struct {
	Identifier  string             `json:"identifier"`
	Description string             `json:"description"`
	Passed      bool               `json:"passed"`
	Severity    string             `json:"severity,omitempty"` // "info", "warning", "error"
	Warning     string             `json:"warning,omitempty"`
	Tracks      []TrackCheckResult `json:"tracks,omitempty"`
	// For non-track checks (e.g. MDB diffs)
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
}

type TrackCheckResult struct {
	ID        string   `json:"id"`
	Type      string   `json:"type"`
	Passed    bool     `json:"passed"`
	TypeOrder int      `json:"type_order"`
	Codec     string   `json:"codec,omitempty"`
	Name      string   `json:"name,omitempty"`
	Language  string   `json:"language,omitempty"`
	Flags     []string `json:"flags,omitempty"`
	Warning   string   `json:"warning,omitempty"`
}

func RunGenericChecks(meta *metadata.Metadata) []CheckResult {
	var results []CheckResult
	results = append(results, CheckYear(meta)...)

	if config.IsCheckEnabled("generic_streaming") {
		results = append(results, CheckStreaming(meta)...)
	}
	if config.IsCheckEnabled("generic_tv_special") {
		results = append(results, CheckTvSpecial(meta)...)
	}
	return results
}

func CheckYear(meta *metadata.Metadata) []CheckResult {
	var results []CheckResult

	if config.IsCheckEnabled("generic_year_missing") {
		res := CheckResult{
			Identifier:  "generic_year_missing",
			Description: "Year is missing for this Movie",
			Passed:      true,
		}
		if meta.Year == 0 && !meta.IsTV {
			res.Passed = false
			res.Severity = "warning"
			res.Warning = "year is missing for this Movie"
		}
		results = append(results, res)
	}

	if config.IsCheckEnabled("generic_year_redundant") {
		res := CheckResult{
			Identifier:  "generic_year_redundant",
			Description: "Redundant Year: The Season already indicates the year",
			Passed:      true,
		}
		if meta.Year > 0 && meta.Season > 1900 {
			res.Passed = false
			res.Severity = "info"
			res.Warning = fmt.Sprintf("redundant Year: The Season (%d) already indicates the year", meta.Season)
		}
		results = append(results, res)
	}

	return results
}

func CheckStreaming(meta *metadata.Metadata) []CheckResult {
	res := CheckResult{
		Identifier:  "generic_streaming",
		Description: "Streaming Service Tag for WEB source",
		Passed:      true,
	}

	isWeb := strings.Contains(meta.Source, "WEB")
	if isWeb && meta.Service == "" {
		res.Passed = false
		res.Severity = "warning"
		res.Warning = "Streaming Service Tag is missing for WEB source"
	}

	if !isWeb && meta.Service != "" {
		res.Passed = false
		res.Severity = "info"
		res.Warning = "Streaming Service Tag is not supported for non-WEB source"
	}

	return []CheckResult{res}
}

func CheckTvSpecial(meta *metadata.Metadata) []CheckResult {
	res := CheckResult{
		Identifier:  "generic_tv_special",
		Description: "Date and Episode Title for TV Specials",
		Passed:      true,
	}

	if meta.IsTV && meta.Season == 0 {
		if meta.Date == "" {
			res.Passed = false
			res.Severity = "warning"
			res.Warning = "Date is missing for TV Special"
		} else if meta.EpisodeTitle == "" {
			res.Passed = false
			res.Severity = "warning"
			res.Warning = "Episode Title is missing for TV Special"
		}
	}
	return []CheckResult{res}
}

func RunMediaInfoChecks(mi *mediainfo.MediaInfo, meta *metadata.Metadata) []CheckResult {
	var results []CheckResult
	var videoTrack *mediainfo.Track
	for i := range mi.Media.Tracks {
		if mi.Media.Tracks[i].Type == "Video" {
			videoTrack = &mi.Media.Tracks[i]
			break
		}
	}

	if videoTrack == nil {
		return []CheckResult{{
			Identifier:  "mediainfo_no_video",
			Description: "Presence of video track",
			Passed:      false,
			Severity:    "error",
			Warning:     "No video track found",
		}}
	}

	// 1. Interlaced WEB
	if config.IsCheckEnabled("mediainfo_interlaced_web") {
		res := CheckResult{
			Identifier:  "mediainfo_interlaced_web",
			Description: "WEB source should not be Interlaced",
			Passed:      true,
		}
		isWeb := strings.Contains(strings.ToUpper(meta.Source), "WEB")
		if isWeb && strings.Contains(strings.ToUpper(videoTrack.ScanType), "INTERLACED") {
			res.Passed = false
			res.Severity = "warning"
			res.Warning = "WEB source should not be Interlaced."
			res.Tracks = []TrackCheckResult{trackToResult(videoTrack, false, res.Warning)}
		}
		results = append(results, res)
	}

	// 2. Non-standard Framerate
	if config.IsCheckEnabled("mediainfo_framerate") {
		results = append(results, CheckFrameRate(videoTrack)...)
	}

	// 3. Low Bitrate
	if config.IsCheckEnabled("mediainfo_bitrate") {
		results = append(results, CheckBitRate(videoTrack)...)
	}

	// 4. Inconsistent Track Durations
	if config.IsCheckEnabled("mediainfo_durations") {
		results = append(results, checkDurations(mi)...)
	}

	// 5. Redundant Audio Tracks
	if config.IsCheckEnabled("mediainfo_redundant_audio") {
		results = append(results, CheckRedundantAudio(mi)...)
	}

	// 6. Non-standard Resolution
	if config.IsCheckEnabled("mediainfo_resolution") {
		results = append(results, CheckResolution(videoTrack)...)
	}

	return results
}

func trackToResult(t *mediainfo.Track, passed bool, warning string) TrackCheckResult {
	order := 0
	if t.TypeOrder != nil {
		order = *t.TypeOrder
	}
	return TrackCheckResult{
		ID:        t.ID,
		Type:      t.Type,
		TypeOrder: order,
		Codec:     t.Format,
		Name:      t.Title,
		Language:  t.Language,
		Passed:    passed,
		Warning:   warning,
	}
}

func CheckRedundantAudio(mi *mediainfo.MediaInfo) []CheckResult {
	res := CheckResult{
		Identifier:  "mediainfo_redundant_audio",
		Description: "Redundant audio tracks for the same language",
		Passed:      true,
	}
	langCounts := make(map[string][]*mediainfo.Track)
	for i := range mi.Media.Tracks {
		track := &mi.Media.Tracks[i]
		if track.Type == "Audio" {
			title := strings.ToLower(track.Title)
			if strings.Contains(title, "commentary") || strings.Contains(title, "description") {
				continue
			}
			lang := track.Language
			if lang == "" {
				lang = "Unknown"
			}
			langCounts[lang] = append(langCounts[lang], track)
		}
	}

	for _, tracks := range langCounts {
		if len(tracks) > 1 {
			res.Passed = false
			res.Severity = "warning"
			for _, t := range tracks {
				res.Tracks = append(res.Tracks, trackToResult(t, false, "Redundant track"))
			}
		}
	}
	return []CheckResult{res}
}

func CheckResolution(videoTrack *mediainfo.Track) []CheckResult {
	res := CheckResult{
		Identifier:  "mediainfo_resolution",
		Description: "Standard resolution and modulo check",
		Passed:      true,
	}
	width := videoTrack.Width
	height := videoTrack.Height

	if width == 0 || height == 0 {
		return []CheckResult{res}
	}

	var trackWarnings []string

	// 1. Modulo check (should be at least mod-2)
	if width%2 != 0 || height%2 != 0 {
		trackWarnings = append(trackWarnings, fmt.Sprintf("non-standard: %dx%d (not mod-2)", width, height))
	}

	// 2. Standard Widths (common for scene/P2P)
	standardWidths := []int{3840, 1920, 1280, 1024, 960, 854, 768, 720, 640}
	isStandardWidth := false
	for _, w := range standardWidths {
		if width == w {
			isStandardWidth = true
			break
		}
	}

	if !isStandardWidth {
		trackWarnings = append(trackWarnings, fmt.Sprintf("non-standard width: %d", width))
	}

	// 3. Aspect Ratio check (sanity)
	if height > width {
		trackWarnings = append(trackWarnings, fmt.Sprintf("unusual aspect ratio (h > w): %d > %d", height, width))
	}

	if len(trackWarnings) > 0 {
		res.Passed = false
		res.Severity = "warning"
		res.Tracks = []TrackCheckResult{trackToResult(videoTrack, false, strings.Join(trackWarnings, "; "))}
	}

	return []CheckResult{res}
}

func CheckFrameRate(videoTrack *mediainfo.Track) []CheckResult {
	res := CheckResult{
		Identifier:  "mediainfo_framerate",
		Description: "Standard framerate check",
		Passed:      true,
	}
	fps := videoTrack.FrameRate
	if fps == 0 {
		return []CheckResult{res}
	}
	standardFPS := []float64{23.976, 24, 25, 29.97, 30, 50, 59.94, 60}
	isStandard := false
	for _, s := range standardFPS {
		if (fps > s-0.1) && (fps < s+0.1) {
			isStandard = true
			break
		}
	}
	if !isStandard {
		res.Passed = false
		res.Severity = "warning"
		res.Tracks = []TrackCheckResult{trackToResult(videoTrack, false, fmt.Sprintf("non-standard framerate: %.3f fps", fps))}
	}
	return []CheckResult{res}
}

func CheckBitRate(videoTrack *mediainfo.Track) []CheckResult {
	res := CheckResult{
		Identifier:  "mediainfo_bitrate",
		Description: "Minimum bitrate check for resolution",
		Passed:      true,
	}
	bitrate := videoTrack.BitRate
	if bitrate == 0 {
		return []CheckResult{res}
	}
	height := videoTrack.Height
	threshold := 0
	if height >= 1080 {
		threshold = 2000000 // 2 Mbps
	} else if height >= 720 {
		threshold = 1000000 // 1 Mbps
	} else if height >= 540 {
		threshold = 500000 // 500 kbps
	}

	if threshold > 0 && bitrate < threshold {
		res.Passed = false
		res.Severity = "warning"
		res.Tracks = []TrackCheckResult{trackToResult(videoTrack, false, fmt.Sprintf("low bitrate for %dp: %d bps", height, bitrate))}
	}
	return []CheckResult{res}
}

func checkDurations(mi *mediainfo.MediaInfo) []CheckResult {
	res := CheckResult{
		Identifier:  "mediainfo_durations",
		Description: "Inconsistent track durations",
		Passed:      true,
	}
	var videoDur float64
	for i := range mi.Media.Tracks {
		if mi.Media.Tracks[i].Type == "Video" && mi.Media.Tracks[i].Duration != 0 {
			videoDur = mi.Media.Tracks[i].Duration
			break
		}
	}

	if videoDur == 0 {
		res.Passed = false
		res.Severity = "error"
		res.Warning = "Video duration is missing"
		return []CheckResult{res}
	}

	for i := range mi.Media.Tracks {
		track := &mi.Media.Tracks[i]
		if (track.Type == "Audio" || track.Type == "Text") && track.Duration != 0 {
			dur := track.Duration
			diff := dur - videoDur
			percentDiff := diff / videoDur * -100
			var trackWarning string
			if diff > 5.0 {
				trackWarning = fmt.Sprintf("significantly longer (diff: %.1fs)", diff)
			} else if diff < -20.0 && track.Type == "Audio" {
				trackWarning = fmt.Sprintf("significantly shorter (diff: %.1fs)", diff)
			} else if percentDiff > 10.0 {
				trackWarning = fmt.Sprintf("%.1f%% shorter (diff: %.1fs)", percentDiff, diff)
			}

			if trackWarning != "" {
				res.Passed = false
				res.Severity = "warning"
				res.Tracks = append(res.Tracks, trackToResult(track, false, trackWarning))
			}
		}
	}

	return []CheckResult{res}
}

func NormalizeForComparison(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, ".", " ")
	s = strings.ReplaceAll(s, "-", " ")
	// remove all non-alphanumeric chars (except spaces)
	re := regexp.MustCompile(`[^a-z0-9 ]`)
	s = re.ReplaceAllString(s, "")
	// collapse multiple spaces
	reSpaces := regexp.MustCompile(`\s+`)
	s = reSpaces.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

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
