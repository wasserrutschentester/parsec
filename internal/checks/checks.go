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

func RunGenericChecks(meta *metadata.Metadata) []string {
	var warnings []string
	if err := CheckYear(meta); err != nil {
		warnings = append(warnings, err.Error())
	}
	if config.IsCheckEnabled("generic_streaming") {
		if err := CheckStreaming(meta); err != nil {
			warnings = append(warnings, err.Error())
		}
	}
	if config.IsCheckEnabled("generic_tv_special") {
		if err := CheckTvSpecial(meta); err != nil {
			warnings = append(warnings, err.Error())
		}
	}
	return warnings
}

func CheckYear(meta *metadata.Metadata) error {
	if config.IsCheckEnabled("generic_year_missing") && meta.Year == 0 && !meta.IsTV {
		return fmt.Errorf("year is missing for this Movie")
	}

	if config.IsCheckEnabled("generic_year_redundant") && meta.Year > 0 && meta.Season > 1900 {
		return fmt.Errorf("redundant Year: The Season (%d) already indicates the year", meta.Season)
	}

	return nil
}

func CheckStreaming(meta *metadata.Metadata) error {
	isWeb := strings.Contains(meta.Source, "WEB")
	if isWeb && meta.Service == "" {
		return fmt.Errorf("Streaming Service Tag is missing for WEB source")
	}

	if !isWeb && meta.Service != "" {
		return fmt.Errorf("Streaming Service Tag is not supported for non-WEB source")
	}

	return nil
}

func CheckTvSpecial(meta *metadata.Metadata) error {
	if meta.IsTV && meta.Season == 0 {
		if meta.Date == "" {
			return fmt.Errorf("Date is missing for TV Special")
		}
		if meta.EpisodeTitle == "" {
			return fmt.Errorf("Episode Title is missing for TV Special")
		}
	}
	return nil
}

func RunMediaInfoChecks(mi *mediainfo.MediaInfo, meta *metadata.Metadata) {
	var videoTrack *mediainfo.Track
	for i := range mi.Media.Tracks {
		if mi.Media.Tracks[i].Type == "Video" {
			videoTrack = &mi.Media.Tracks[i]
			break
		}
	}

	if videoTrack == nil {
		return
	}

	// 1. Interlaced WEB
	if config.IsCheckEnabled("mediainfo_interlaced_web") {
		isWeb := strings.Contains(strings.ToUpper(meta.Source), "WEB")
		if isWeb && strings.Contains(strings.ToUpper(videoTrack.ScanType), "INTERLACED") {
			ui.PrintWarning("QA Warning: WEB source should not be Interlaced.")
		}
	}

	// 2. Non-standard Framerate
	if config.IsCheckEnabled("mediainfo_framerate") {
		for _, w := range CheckFrameRate(videoTrack) {
			ui.PrintWarning(w)
		}
	}

	// 3. Low Bitrate
	if config.IsCheckEnabled("mediainfo_bitrate") {
		for _, w := range CheckBitRate(videoTrack) {
			ui.PrintWarning(w)
		}
	}

	// 4. Inconsistent Track Durations
	if config.IsCheckEnabled("mediainfo_durations") {
		for _, w := range checkDurations(mi) {
			ui.PrintWarning(w)
		}
	}

	// 5. Redundant Audio Tracks
	if config.IsCheckEnabled("mediainfo_redundant_audio") {
		for _, w := range CheckRedundantAudio(mi) {
			ui.PrintWarning(w)
		}
	}

	// 6. Non-standard Resolution
	if config.IsCheckEnabled("mediainfo_resolution") {
		for _, w := range CheckResolution(videoTrack) {
			ui.PrintWarning(w)
		}
	}
}

func CheckRedundantAudio(mi *mediainfo.MediaInfo) []string {
	var warnings []string
	langCounts := make(map[string]int)
	for _, track := range mi.Media.Tracks {
		if track.Type == "Audio" {
			title := strings.ToLower(track.Title)
			if strings.Contains(title, "commentary") || strings.Contains(title, "description") {
				continue
			}
			lang := track.Language
			if lang == "" {
				lang = "Unknown"
			}
			langCounts[lang]++
		}
	}

	for lang, count := range langCounts {
		if count > 1 {
			warnings = append(warnings, fmt.Sprintf("QA Warning: Redundant audio tracks detected for language '%s' (%d tracks)", lang, count))
		}
	}
	return warnings
}

func CheckResolution(videoTrack *mediainfo.Track) []string {
	var warnings []string
	width := videoTrack.Width
	height := videoTrack.Height

	if width == 0 || height == 0 {
		return nil
	}

	// 1. Modulo check (should be at least mod-2)
	if width%2 != 0 || height%2 != 0 {
		warnings = append(warnings, fmt.Sprintf("QA Warning: Non-standard resolution: %dx%d (not divisible by 2)", width, height))
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
		warnings = append(warnings, fmt.Sprintf("QA Warning: Non-standard width detected: %d", width))
	}

	// 3. Aspect Ratio check (sanity)
	if height > width {
		warnings = append(warnings, fmt.Sprintf("QA Warning: Unusual aspect ratio: height (%d) is greater than width (%d)", height, width))
	}
	return warnings
}

func CheckFrameRate(videoTrack *mediainfo.Track) []string {
	var warnings []string
	fps := videoTrack.FrameRate
	if fps == 0 {
		return nil
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
		warnings = append(warnings, fmt.Sprintf("QA Warning: Non-standard framerate detected: %.3f fps", fps))
	}
	return warnings
}

func CheckBitRate(videoTrack *mediainfo.Track) []string {
	var warnings []string
	bitrate := videoTrack.BitRate
	if bitrate == 0 {
		return nil
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
		warnings = append(warnings, fmt.Sprintf("QA Warning: Low bitrate detected for %dp: %d bps", height, bitrate))
	}
	return warnings
}

func checkDurations(mi *mediainfo.MediaInfo) []string {
	var warnings []string
	var videoDur float64
	for _, track := range mi.Media.Tracks {
		if track.Type == "Video" && track.Duration != 0 {
			videoDur = track.Duration
			break
		}
	}

	if videoDur == 0 {
		warnings = append(warnings, "Video duration is missing")
		return warnings
	}

	for _, track := range mi.Media.Tracks {
		if (track.Type == "Audio" || track.Type == "Text") && track.Duration != 0 {
			dur := track.Duration
			diff := dur - videoDur
			percentDiff := diff / videoDur * -100
			if diff > 5.0 {
				warnings = append(warnings, fmt.Sprintf("QA Warning: %s track #%02d (ID %s) is significantly longer than video (diff: %.1fs)", track.Type, *track.TypeOrder, track.ID, diff))
			} else if diff < -20.0 && track.Type == "Audio" {
				warnings = append(warnings, fmt.Sprintf("QA Warning: Audio track #%02d (ID %s) is significantly shorter than video (diff: %.1fs)", *track.TypeOrder, track.ID, diff))

			} else if percentDiff > 10.0 {
				warnings = append(warnings, fmt.Sprintf("QA Warning: Subtitle track #%02d (ID %s) is %.1f%% shorter than video (diff: %.1fs)", *track.TypeOrder, track.ID, percentDiff, diff))
			}
		}
	}
	return warnings
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

func RunMdbChecks(mi *mediainfo.MediaInfo, meta *metadata.Metadata) []string {
	var warnings []string
	searchResult, searchErr := mdbSearch.InteractiveSearch(meta, true)

	if searchErr != nil {
		warnings = append(warnings, fmt.Sprintf("MDB Error: %v", searchErr))
		return warnings
	}
	if searchResult == nil {
		warnings = append(warnings, "MDB Warning: No matching metadata found on TMDB/TVDB.")
		return warnings
	}

	if config.IsCheckEnabled("mdb_title") {
		warnings = append(warnings, CheckTitle(meta, searchResult)...)
	}
	if config.IsCheckEnabled("mdb_movie_year") {
		warnings = append(warnings, CheckMovieYear(meta, searchResult)...)
	}
	if meta.IsTV && config.IsCheckEnabled("mdb_series_year") {
		warnings = append(warnings, CheckSeriesYear(meta, searchResult)...)
	}
	if meta.IsTV {
		warnings = append(warnings, CheckEpisode(meta, searchResult)...)
	}

	if mi != nil && config.IsCheckEnabled("mdb_track_languages") {
		warnings = append(warnings, CheckTrackLanguages(mi, searchResult)...)
	}
	return warnings
}

func CheckTrackLanguages(mi *mediainfo.MediaInfo, result *mdb.SearchResult) []string {
	var warnings []string
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
		if !found {
			warnings = append(warnings, fmt.Sprintf("QA Warning: %s track in %s language '%s' is missing.", trackType, label, targetStr))
		}
	}

	check("Audio", audioLangs, prefTag, prefLang, "preferred")
	check("Subtitle", subLangs, prefTag, prefLang, "preferred")

	if origLang != "" && origTag != prefTag {
		check("Audio", audioLangs, origTag, origLang, "original")
		check("Subtitle", subLangs, origTag, origLang, "original")
	}
	return warnings
}

func CheckMovieYear(meta *metadata.Metadata, result *mdb.SearchResult) []string {
	var warnings []string
	if !meta.IsTV && meta.Year > 0 && result.Year > 0 && meta.Year != result.Year {
		warnings = append(warnings, fmt.Sprintf("MDB Warning: Year mismatch. Filename: %d, TMDB: %d", meta.Year, result.Year))
	}
	return warnings
}

func CheckSeriesYear(meta *metadata.Metadata, result *mdb.SearchResult) []string {
	var warnings []string
	if meta.Year > 0 && result.Year > 0 && meta.Year != result.Year && meta.Season < 1900 {
		warnings = append(warnings, fmt.Sprintf("MDB Warning: Series start year mismatch. Filename: %d, TMDB/TVDB: %d", meta.Year, result.Year))
	}
	return warnings
}

func CheckEpisode(meta *metadata.Metadata, result *mdb.SearchResult) []string {
	var warnings []string
	if meta.Season > 0 || meta.Episode > 0 {
		epResult := mdbSearch.FindEpisode(*result, meta.Season, meta.Episode)
		if epResult.Name == "" {
			if config.IsCheckEnabled("mdb_episode_existence") {
				warnings = append(warnings, fmt.Sprintf("MDB Warning: Episode S%02dE%02d not found on TVDB/TMDB.", meta.Season, meta.Episode))
			}
		} else {
			if config.IsCheckEnabled("mdb_episode_title") {
				warnings = append(warnings, CheckEpisodeTitle(meta, epResult)...)
			}
			if config.IsCheckEnabled("mdb_episode_date") {
				warnings = append(warnings, CheckSpecialDate(meta, epResult)...)
			}
		}
	}
	return warnings
}

func CheckEpisodeTitle(meta *metadata.Metadata, epResult mdb.EpisodeResult) []string {
	var warnings []string
	if meta.EpisodeTitle != "" {
		normParsed := NormalizeForComparison(filename.DeobfuscateTitle(meta.EpisodeTitle))
		normOfficial := NormalizeForComparison(epResult.Name)
		if normParsed != normOfficial {
			warnings = append(warnings, fmt.Sprintf("MDB Warning: Episode title mismatch.\n  Filename: %s\n  TVDB:     %s", meta.EpisodeTitle, epResult.Name))
		}
	}
	return warnings
}

func CheckTitle(meta *metadata.Metadata, result *mdb.SearchResult) []string {
	var warnings []string
	if meta.Title != "" {
		normParsed := NormalizeForComparison(filename.DeobfuscateTitle(meta.Title))
		normOfficial := NormalizeForComparison(result.Title)
		if normParsed != normOfficial {
			warnings = append(warnings, fmt.Sprintf("MDB Warning: Title mismatch.\n  Filename: %s\n  TMDB/TVDB: %s", meta.Title, result.Title))
		}
	}
	return warnings
}

func CheckSpecialDate(meta *metadata.Metadata, epResult mdb.EpisodeResult) []string {
	var warnings []string
	if meta.Season == 0 && meta.Date != "" {
		if epResult.Airdate != meta.Date {
			warnings = append(warnings, fmt.Sprintf("MDB Warning: Special episode date mismatch. Filename: %s, TVDB: %s", meta.Date, epResult.Airdate))
		}
	}
	return warnings
}
