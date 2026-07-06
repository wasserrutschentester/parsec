package checks

import (
	"fmt"
	"strconv"
	"strings"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/types"
)

func formatNsToTime(ns int64) string {
	ms := ns / 1000000
	hours := ms / 3600000
	ms %= 3600000
	minutes := ms / 60000
	ms %= 60000
	seconds := ms / 1000
	ms %= 1000

	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, ms)
}

func checkChaptersStartNonZero(chapters *matroska.Chapters) *CheckResult {
	if chapters == nil || len(chapters.Atoms) == 0 {
		return nil
	}

	firstChapter := chapters.Atoms[0]
	if firstChapter.TimeStart != 0 {
		timeStr := formatNsToTime(firstChapter.TimeStart)

		return &CheckResult{
			Identifier: config.CheckMatroskaChaptersStartNonZero,
			Warning:    "First chapter does not start at 00:00:00",
			Passed:     false,
			Severity:   "warning",
			Actual:     timeStr,
			Expected:   "00:00:00.000",
		}
	}

	return nil
}

func checkChaptersNonMonotonic(chapters *matroska.Chapters) *CheckResult {
	if chapters == nil || len(chapters.Atoms) == 0 {
		return nil
	}

	var lastTime int64 = -1
	for _, ch := range chapters.Atoms {
		if lastTime >= 0 && ch.TimeStart < lastTime {
			return &CheckResult{
				Identifier: config.CheckMatroskaChaptersNonMonotonic,
				Warning:    "Chapter times are not strictly increasing",
				Passed:     false,
				Severity:   "error",
				Actual:     fmt.Sprintf("chapter starts at %s after %s", formatNsToTime(ch.TimeStart), formatNsToTime(lastTime)),
			}
		}

		lastTime = ch.TimeStart
	}

	return nil
}

func checkChaptersDuplicate(chapters *matroska.Chapters) *CheckResult {
	if chapters == nil || len(chapters.Atoms) == 0 {
		return nil
	}

	seenTimes := make(map[int64]bool)
	for _, ch := range chapters.Atoms {
		if seenTimes[ch.TimeStart] {
			return &CheckResult{
				Identifier: config.CheckMatroskaChaptersDuplicate,
				Warning:    "Duplicate chapter timestamps found",
				Passed:     false,
				Severity:   "error",
				Actual:     "duplicate timestamp at " + formatNsToTime(ch.TimeStart),
			}
		}

		seenTimes[ch.TimeStart] = true
	}

	return nil
}

func checkChaptersTooClose(chapters *matroska.Chapters) *CheckResult {
	if chapters == nil || len(chapters.Atoms) == 0 {
		return nil
	}

	var lastTime int64 = -1
	for _, ch := range chapters.Atoms {
		if lastTime >= 0 {
			diff := ch.TimeStart - lastTime
			if diff < 10000000000 {
				return &CheckResult{
					Identifier: config.CheckMatroskaChaptersTooClose,
					Warning:    "Chapter interval is too short (< 10 seconds)",
					Passed:     false,
					Severity:   "warning",
					Actual:     fmt.Sprintf("interval is %.1fs between %s and %s", float64(diff)/1000000000.0, formatNsToTime(lastTime), formatNsToTime(ch.TimeStart)),
				}
			}
		}

		lastTime = ch.TimeStart
	}

	return nil
}

func checkChaptersExceedDuration(ebml *matroska.EbmlMetadata, chapters *matroska.Chapters) *CheckResult {
	if chapters == nil || len(chapters.Atoms) == 0 {
		return nil
	}

	duration := ebml.Container.Properties.Duration
	if duration <= 0 {
		return nil
	}

	for _, ch := range chapters.Atoms {
		if ch.TimeStart > duration {
			return &CheckResult{
				Identifier: config.CheckMatroskaChaptersExceedDuration,
				Warning:    "Chapter timestamp exceeds video duration",
				Passed:     false,
				Severity:   "error",
				Actual:     fmt.Sprintf("chapter at %s (video duration is %s)", formatNsToTime(ch.TimeStart), formatNsToTime(duration)),
			}
		}
	}

	return nil
}

func checkSingleChapterNameHygiene(ch matroska.EbmlChapterAtom) (bool, []string) {
	hasNonEmpty := false

	var currentNames []string

	for _, display := range ch.Display {
		name := strings.TrimSpace(display.String)
		if name != "" {
			hasNonEmpty = true

			currentNames = append(currentNames, name)
		}
	}

	return hasNonEmpty, currentNames
}

func checkConsecutiveDuplicateNames(currentNames, lastNames []string, timeStart int64) *CheckResult {
	for _, currentName := range currentNames {
		for _, lastName := range lastNames {
			if strings.EqualFold(currentName, lastName) {
				return &CheckResult{
					Identifier: config.CheckMatroskaChaptersNameHygiene,
					Warning:    "Consecutive duplicate chapter names found",
					Passed:     false,
					Severity:   "warning",
					Actual:     fmt.Sprintf("consecutive chapters have name %q at %s", currentName, formatNsToTime(timeStart)),
				}
			}
		}
	}

	return nil
}

func checkChaptersNameHygiene(chapters *matroska.Chapters) *CheckResult {
	if chapters == nil || len(chapters.Atoms) == 0 {
		return nil
	}

	var lastNames []string

	for i, ch := range chapters.Atoms {
		if len(ch.Display) == 0 {
			return &CheckResult{
				Identifier: config.CheckMatroskaChaptersNameHygiene,
				Warning:    "Chapter has no display name entry",
				Passed:     false,
				Severity:   "warning",
				Actual:     fmt.Sprintf("chapter %d (starts at %s)", i+1, formatNsToTime(ch.TimeStart)),
			}
		}

		hasNonEmpty, currentNames := checkSingleChapterNameHygiene(ch)

		if !hasNonEmpty {
			return &CheckResult{
				Identifier: config.CheckMatroskaChaptersNameHygiene,
				Warning:    "Chapter display name is empty or only whitespace",
				Passed:     false,
				Severity:   "warning",
				Actual:     fmt.Sprintf("chapter %d (starts at %s)", i+1, formatNsToTime(ch.TimeStart)),
			}
		}

		if i > 0 {
			if res := checkConsecutiveDuplicateNames(currentNames, lastNames, ch.TimeStart); res != nil {
				return res
			}
		}

		if len(currentNames) > 0 {
			lastNames = currentNames
		}
	}

	return nil
}

func getChapterLanguages(ch matroska.EbmlChapterAtom) (map[string]bool, *CheckResult) {
	currentLangs := make(map[string]bool)

	for _, display := range ch.Display {
		lang := strings.TrimSpace(display.Language)
		if lang == "" || strings.EqualFold(lang, "und") {
			chapterName := display.String
			if chapterName == "" {
				chapterName = "(no name)"
			}

			return nil, &CheckResult{
				Identifier: config.CheckMatroskaChaptersLanguageHygiene,
				Warning:    "Chapter display entry has undetermined or missing language",
				Passed:     false,
				Severity:   "warning",
				Actual:     fmt.Sprintf("chapter %q (starts at %s) language is %q", chapterName, formatNsToTime(ch.TimeStart), lang),
			}
		}

		currentLangs[strings.ToLower(lang)] = true
	}

	return currentLangs, nil
}

func checkLanguagesInconsistent(firstLangs, currentLangs map[string]bool, timeStart int64) *CheckResult {
	if len(currentLangs) == 0 || len(firstLangs) == 0 {
		return nil
	}

	matches := true
	if len(currentLangs) != len(firstLangs) {
		matches = false
	} else {
		for l := range currentLangs {
			if !firstLangs[l] {
				matches = false

				break
			}
		}
	}

	if !matches {
		var list1, list2 []string
		for l := range firstLangs {
			list1 = append(list1, l)
		}

		for l := range currentLangs {
			list2 = append(list2, l)
		}

		return &CheckResult{
			Identifier: config.CheckMatroskaChaptersLanguageHygiene,
			Warning:    "Inconsistent chapter languages in edition",
			Passed:     false,
			Severity:   "warning",
			Actual:     fmt.Sprintf("languages %v vs %v at chapter starting at %s", list1, list2, formatNsToTime(timeStart)),
		}
	}

	return nil
}

func checkChaptersLanguageHygiene(chapters *matroska.Chapters) *CheckResult {
	if chapters == nil || len(chapters.Atoms) == 0 {
		return nil
	}

	var firstLangs map[string]bool

	for _, ch := range chapters.Atoms {
		currentLangs, res := getChapterLanguages(ch)
		if res != nil {
			return res
		}

		if firstLangs == nil {
			if len(currentLangs) > 0 {
				firstLangs = currentLangs
			}
		} else {
			if res := checkLanguagesInconsistent(firstLangs, currentLangs, ch.TimeStart); res != nil {
				return res
			}
		}
	}

	return nil
}

func findPrevNextKeyframes(timeStart int64, keyframes []int64) (int64, int64) {
	prevKF := int64(-1)
	nextKF := int64(-1)

	for _, kf := range keyframes {
		if kf <= timeStart {
			if prevKF == -1 || kf > prevKF {
				prevKF = kf
			}
		} else {
			if nextKF == -1 || kf < nextKF {
				nextKF = kf
			}
		}
	}

	return prevKF, nextKF
}

// IsAligned reports whether a chapter timestamp is close enough to a keyframe.
func IsAligned(timeStart int64, keyframes []int64) (bool, int64, int64, int64) {
	closestDiff := int64(-1)
	prevKF, nextKF := findPrevNextKeyframes(timeStart, keyframes)

	for _, kf := range keyframes {
		var (
			currentDiff int64
			inRange     bool
		)

		diff := timeStart - kf

		if diff >= 0 {
			currentDiff = diff
			inRange = diff <= 8_000_000
		} else {
			currentDiff = -diff
			inRange = (-diff) <= 1_000_000 // Allow up to 1ms negative tolerance for rounding errors
		}

		if closestDiff == -1 || currentDiff < closestDiff {
			closestDiff = currentDiff
		}

		if inRange {
			return true, closestDiff, -1, -1
		}
	}

	return false, closestDiff, prevKF, nextKF
}

// GetVideoTrackFromEBML returns the first video track from the EBML metadata.
func GetVideoTrackFromEBML(ebml *matroska.EbmlMetadata) *matroska.EbmlTrack {
	for i, track := range ebml.Tracks {
		if track.Type == "video" {
			return &ebml.Tracks[i]
		}
	}

	return nil
}

func formatSeekLatency(prevKF, timeStart int64, videoTrack *matroska.EbmlTrack) string {
	if prevKF == -1 {
		return "-"
	}

	diff := timeStart - prevKF
	latencyStr := fmt.Sprintf("%.3fs", float64(diff)/1e9)

	if videoTrack != nil && videoTrack.Properties.DefaultDuration > 0 {
		frames := float64(diff) / float64(videoTrack.Properties.DefaultDuration)
		latencyStr += fmt.Sprintf(" (%.0ff)", frames)
	}

	return latencyStr
}

func formatNextKF(nextKF, timeStart int64, videoTrack *matroska.EbmlTrack) string {
	if nextKF == -1 {
		return "-"
	}

	nextStr := formatNsToTime(nextKF)

	if videoTrack != nil && videoTrack.Properties.DefaultDuration > 0 {
		frames := float64(nextKF-timeStart) / float64(videoTrack.Properties.DefaultDuration)
		nextStr += fmt.Sprintf(" (+%.0ff)", frames)
	}

	return nextStr
}

func getAlignedTableRows(chapters *matroska.Chapters, keyframes []int64, videoTrack *matroska.EbmlTrack) [][]string {
	var rows [][]string

	for i, ch := range chapters.Atoms {
		if aligned, _, prevKF, nextKF := IsAligned(ch.TimeStart, keyframes); !aligned {
			name := "-"
			if len(ch.Display) > 0 && ch.Display[0].String != "" {
				name = ch.Display[0].String
			}

			latencyStr := formatSeekLatency(prevKF, ch.TimeStart, videoTrack)

			prevStr := "-"
			if prevKF != -1 {
				prevStr = formatNsToTime(prevKF)
			}

			nextStr := formatNextKF(nextKF, ch.TimeStart, videoTrack)

			rows = append(rows, []string{
				strconv.Itoa(i + 1),
				name,
				formatNsToTime(ch.TimeStart),
				latencyStr,
				prevStr,
				nextStr,
			})
		}
	}

	return rows
}

func checkChaptersKeyframeAlignment(filePath string, ebml *matroska.EbmlMetadata, chapters *matroska.Chapters) *CheckResult {
	if filePath == "" {
		return nil
	}

	videoTrack := GetVideoTrackFromEBML(ebml)
	if videoTrack == nil {
		return nil
	}

	keyframes, err := matroska.ReadKeyframeTimestamps(filePath, uint64(videoTrack.Properties.Number), ebml.Container.Properties.TimestampScale)
	if err != nil {
		return &CheckResult{
			Identifier: config.CheckMatroskaChaptersKeyframeAlignment,
			Warning:    "Failed to read video cues index (SeekHead/Cues may be missing or invalid)",
			Passed:     false,
			Severity:   "warning",
			Actual:     err.Error(),
		}
	}

	if len(keyframes) == 0 {
		return &CheckResult{
			Identifier: config.CheckMatroskaChaptersKeyframeAlignment,
			Warning:    "No video cues/index entries found (seeking might be slow or broken)",
			Passed:     false,
			Severity:   "warning",
			Actual:     "zero cues indexed for video track",
		}
	}

	if chapters == nil || len(chapters.Atoms) == 0 {
		return nil
	}

	rows := getAlignedTableRows(chapters, keyframes, videoTrack)

	if len(rows) > 0 {
		return &CheckResult{
			Identifier: config.CheckMatroskaChaptersKeyframeAlignment,
			Warning:    "Chapters are not aligned with video keyframes",
			Passed:     false,
			Severity:   "warning",
			Table: &types.TableData{
				Headers: []string{"#", "Name", "Timestamp", "Seek Latency", "Previous KF", "Next KF"},
				Rows:    rows,
			},
		}
	}

	return nil
}
