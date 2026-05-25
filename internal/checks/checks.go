package checks

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"codeberg.org/n0ne/parsec/internal/mdb"
	mdbSearch "codeberg.org/n0ne/parsec/internal/mdb/search"
	"codeberg.org/n0ne/parsec/internal/metadata"
	"codeberg.org/n0ne/parsec/internal/metadata/filename"
	"codeberg.org/n0ne/parsec/internal/metadata/mediainfo"
)

func RunGenericChecks(meta *metadata.Metadata) {
	if err := CheckYear(meta); err != nil {
		fmt.Println(err)
	}
	if err := CheckStreaming(meta); err != nil {
		fmt.Println(err)
	}
	if err := CheckTvSpecial(meta); err != nil {
		fmt.Println(err)
	}
}

func CheckYear(meta *metadata.Metadata) error {
	if meta.Year == 0 && !meta.IsTV {
		return fmt.Errorf("year is missing for this Movie")
	}

	if meta.Year > 0 && meta.Season > 1900 {
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
	isWeb := strings.Contains(strings.ToUpper(meta.Source), "WEB")
	if isWeb && strings.Contains(strings.ToUpper(videoTrack.ScanType), "INTERLACED") {
		fmt.Println("QA Warning: WEB source should not be Interlaced.")
	}

	// 2. Non-standard Framerate
	CheckFrameRate(videoTrack)

	// 3. Low Bitrate
	CheckBitRate(videoTrack)

	// 4. Inconsistent Track Durations
	checkDurations(mi)

	// 5. Redundant Audio Tracks
	CheckRedundantAudio(mi)

	// 6. Non-standard Resolution
	CheckResolution(videoTrack)
}

func CheckRedundantAudio(mi *mediainfo.MediaInfo) {
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
			fmt.Printf("QA Warning: Redundant audio tracks detected for language '%s' (%d tracks)\n", lang, count)
		}
	}
}

func CheckResolution(videoTrack *mediainfo.Track) {
	width, _ := strconv.Atoi(videoTrack.Width)
	height, _ := strconv.Atoi(videoTrack.Height)

	if width == 0 || height == 0 {
		return
	}

	// 1. Modulo check (should be at least mod-2)
	if width%2 != 0 || height%2 != 0 {
		fmt.Printf("QA Warning: Non-standard resolution: %dx%d (not divisible by 2)\n", width, height)
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
		fmt.Printf("QA Warning: Non-standard width detected: %d\n", width)
	}

	// 3. Aspect Ratio check (sanity)
	if height > width {
		fmt.Printf("QA Warning: Unusual aspect ratio: height (%d) is greater than width (%d)\n", height, width)
	}
}

func CheckFrameRate(videoTrack *mediainfo.Track) {
	if videoTrack.FrameRate == "" {
		return
	}
	fps, err := strconv.ParseFloat(videoTrack.FrameRate, 64)
	if err != nil {
		return
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
		fmt.Printf("QA Warning: Non-standard framerate detected: %.3f fps\n", fps)
	}
}

func CheckBitRate(videoTrack *mediainfo.Track) {
	if videoTrack.BitRate == "" {
		return
	}
	bitrate, err := strconv.Atoi(videoTrack.BitRate)
	if err != nil {
		return
	}
	height, _ := strconv.Atoi(videoTrack.Height)
	threshold := 0
	if height >= 1080 {
		threshold = 2000000 // 2 Mbps
	} else if height >= 720 {
		threshold = 1000000 // 1 Mbps
	} else if height >= 540 {
		threshold = 500000 // 500 kbps
	}

	if threshold > 0 && bitrate < threshold {
		fmt.Printf("QA Warning: Low bitrate detected for %dp: %d bps\n", height, bitrate)
	}
}

func checkDurations(mi *mediainfo.MediaInfo) {
	var videoDur float64
	for _, track := range mi.Media.Tracks {
		if track.Type == "Video" && track.Duration != "" {
			videoDur, _ = strconv.ParseFloat(track.Duration, 64)
			break
		}
	}

	if videoDur == 0 {
		fmt.Println("Video duration is missing")
		return
	}

	for _, track := range mi.Media.Tracks {
		if (track.Type == "Audio" || track.Type == "Text") && track.Duration != "" {
			dur, _ := strconv.ParseFloat(track.Duration, 64)
			diff := dur - videoDur
			percentDiff := diff / videoDur * -100
			if diff > 5.0 {
				fmt.Printf("QA Warning: %s track (ID %s) is significantly longer than video (diff: %.1fs)\n", track.Type, track.ID, diff)
			} else if diff < -20.0 && track.Type == "Audio" {
				fmt.Printf("QA Warning: Audio track (ID %s) is significantly shorter than video (diff: %.1fs)\n", track.ID, diff)
			} else if percentDiff > 10.0 {
				fmt.Printf("QA Warning: Subtitle track (ID %s) is %.1f%% shorter than video (diff: %.1fs)\n", track.ID, percentDiff, diff)
			}
		}
	}
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

func RunMdbChecks(meta *metadata.Metadata, imdbID string, tmdbID, tvdbID int) {
	var searchResult *mdb.SearchResult
	var searchErr error

	if imdbID != "" || tmdbID > 0 || tvdbID > 0 {
		searchResult, searchErr = mdbSearch.SearchByID(imdbID, tmdbID, tvdbID, meta.IsTV)
	} else {
		searchQuery := filename.DeobfuscateTitle(meta.Title)
		searchResult, searchErr = mdbSearch.FuzzySearch(searchQuery, meta.Year, meta.IsTV)
	}

	if searchErr != nil {
		fmt.Printf("MDB Error: Could not fetch metadata from TMDB/TVDB: %v\n", searchErr)
		return
	}
	if searchResult == nil {
		fmt.Println("MDB Warning: No matching metadata found on TMDB/TVDB.")
		return
	}

	fmt.Printf("MDB Matched: %s (%d)\n", searchResult.Title, searchResult.Year)

	CheckTitle(meta, searchResult)
	CheckMovieYear(meta, searchResult)
	if meta.IsTV {
		CheckSeriesYear(meta, searchResult)
		CheckEpisode(meta, searchResult)
	}
}

func CheckMovieYear(meta *metadata.Metadata, result *mdb.SearchResult) {
	if !meta.IsTV && meta.Year > 0 && result.Year > 0 && meta.Year != result.Year {
		fmt.Printf("MDB Warning: Year mismatch. Filename: %d, TMDB: %d\n", meta.Year, result.Year)
	}
}

func CheckSeriesYear(meta *metadata.Metadata, result *mdb.SearchResult) {
	if meta.Year > 0 && result.Year > 0 && meta.Year != result.Year && meta.Season < 1900 {
		fmt.Printf("MDB Warning: Series start year mismatch. Filename: %d, TMDB/TVDB: %d\n", meta.Year, result.Year)
	}
}

func CheckEpisode(meta *metadata.Metadata, result *mdb.SearchResult) {
	if meta.Season > 0 || meta.Episode > 0 {
		epResult := mdbSearch.FindEpisode(*result, meta.Season, meta.Episode)
		if epResult.Name == "" {
			fmt.Printf("MDB Warning: Episode S%02dE%02d not found on TVDB/TMDB.\n", meta.Season, meta.Episode)
		} else {
			fmt.Printf("MDB Episode: %s (S%02dE%02d)\n", epResult.Name, epResult.Season, epResult.Episode)
			CheckEpisodeTitle(meta, epResult)
			CheckSpecialDate(meta, epResult)
		}
	}
}

func CheckEpisodeTitle(meta *metadata.Metadata, epResult mdb.EpisodeResult) {
	if meta.EpisodeTitle != "" {
		normParsed := NormalizeForComparison(filename.DeobfuscateTitle(meta.EpisodeTitle))
		normOfficial := NormalizeForComparison(epResult.Name)
		if normParsed != normOfficial {
			fmt.Printf("MDB Warning: Episode title mismatch.\n  Filename: %s\n  TVDB:     %s\n", meta.EpisodeTitle, epResult.Name)
		}
	}
}

func CheckTitle(meta *metadata.Metadata, result *mdb.SearchResult) {
	if meta.Title != "" {
		normParsed := NormalizeForComparison(filename.DeobfuscateTitle(meta.Title))
		normOfficial := NormalizeForComparison(result.Title)
		if normParsed != normOfficial {
			fmt.Printf("MDB Warning: Title mismatch.\n  Filename: %s\n  TMDB/TVDB: %s\n", meta.Title, result.Title)
		}
	}
}

func CheckSpecialDate(meta *metadata.Metadata, epResult mdb.EpisodeResult) {
	if meta.Season == 0 && meta.Date != "" {
		if epResult.Airdate != meta.Date {
			fmt.Printf("MDB Warning: Special episode date mismatch. Filename: %s, TVDB: %s\n", meta.Date, epResult.Airdate)
		}
	}
}
