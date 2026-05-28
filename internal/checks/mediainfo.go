package checks

import (
	"fmt"
	"strings"

	"codeberg.org/n0ne/parsec/internal/config"
	"codeberg.org/n0ne/parsec/internal/metadata"
	"codeberg.org/n0ne/parsec/internal/metadata/mediainfo"
	"codeberg.org/n0ne/parsec/internal/ui"
)

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
			Identifier: "mediainfo_no_video",
			Passed:     false,
			Severity:   "error",
			Warning:    "No video track found",
		}}
	}

	// 1. Interlaced WEB
	if config.IsCheckEnabled("mediainfo_interlaced_web") {
		res := CheckResult{
			Identifier: "mediainfo_interlaced_web",
			Passed:     true,
		}
		isWeb := strings.Contains(strings.ToUpper(meta.Source), "WEB")
		if isWeb && strings.Contains(strings.ToUpper(videoTrack.ScanType), "INTERLACED") {
			res.Passed = false
			res.Severity = "warning"
			res.Warning = "WEB source should not be Interlaced"
			res.Tracks = []TrackCheckResult{miTrackToResult(videoTrack, false, res.Warning)}
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

func miTrackToResult(t *mediainfo.Track, passed bool, warning string) TrackCheckResult {
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
		Identifier: "mediainfo_redundant_audio",
		Passed:     true,
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
			res.Warning = "Redundant audio tracks found for the same language"
			for _, t := range tracks {
				res.Tracks = append(res.Tracks, miTrackToResult(t, false, "Redundant track"))
			}
		}
	}
	return []CheckResult{res}
}

func CheckResolution(videoTrack *mediainfo.Track) []CheckResult {
	res := CheckResult{
		Identifier: "mediainfo_resolution",
		Passed:     true,
	}
	width := videoTrack.Width
	height := videoTrack.Height

	if width == 0 || height == 0 {
		return []CheckResult{res}
	}

	var trackWarnings []string

	// 1. Modulo check (should be at least mod-2)
	if width%2 != 0 {
		trackWarnings = append(trackWarnings, "odd width")
	}
	if height%2 != 0 {
		trackWarnings = append(trackWarnings, "odd height")
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
		trackWarnings = append(trackWarnings, "non-standard width")
	}

	// 3. Aspect Ratio check (sanity)
	if height > width {
		trackWarnings = append(trackWarnings, "vertical (h > w)")
	}

	if len(trackWarnings) > 0 {
		res.Passed = false
		res.Severity = "warning"
		res.Warning = fmt.Sprintf("Non-standard Resolution (%dx%d): %s", width, height, strings.Join(trackWarnings, " / "))
	}

	return []CheckResult{res}
}

func CheckFrameRate(videoTrack *mediainfo.Track) []CheckResult {
	res := CheckResult{
		Identifier: "mediainfo_framerate",
		Passed:     true,
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
		res.Warning = "Non-standard framerate"
		res.Tracks = []TrackCheckResult{miTrackToResult(videoTrack, false, fmt.Sprintf("non-standard %s: %.3f fps", ui.Warning.Render("framerate"), fps))}
	}
	return []CheckResult{res}
}

func CheckBitRate(videoTrack *mediainfo.Track) []CheckResult {
	res := CheckResult{
		Identifier: "mediainfo_bitrate",
		Passed:     true,
	}
	bitrate := videoTrack.BitRate
	bitrateKb := float64(bitrate) / float64(1024)
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
		res.Warning = fmt.Sprintf("Low Video bitrate %.1f kb/s for %dp", bitrateKb, height)
	}
	return []CheckResult{res}
}

func checkDurations(mi *mediainfo.MediaInfo) []CheckResult {
	res := CheckResult{
		Identifier: "mediainfo_durations",
		Passed:     true,
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
				trackWarning = fmt.Sprintf("%s (diff: %.1fs)", ui.Error.Render("significantly longer"), diff)
				res.Severity = "error"
			} else if diff < -20.0 && track.Type == "Audio" {
				trackWarning = fmt.Sprintf("%s (diff: %.1fs)", ui.Warning.Render("significantly shorter"), diff)
			} else if percentDiff > 10.0 {
				trackWarning = fmt.Sprintf("%.1f%% %s (diff: %.1fs)", percentDiff, ui.Warning.Render("shorter"), diff)
			}

			if trackWarning != "" {
				res.Passed = false
				if res.Severity == "" {
					res.Severity = "warning"
				}
				res.Warning = "Inconsistent track durations"
				res.Tracks = append(res.Tracks, miTrackToResult(track, false, trackWarning))
			}
		}
	}

	return []CheckResult{res}
}
