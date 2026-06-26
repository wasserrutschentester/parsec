package checks

import (
	"fmt"
	"strings"

	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
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
			Identifier: "matroska_chapters_start_non_zero",
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
				Identifier: "matroska_chapters_non_monotonic",
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
				Identifier: "matroska_chapters_duplicate",
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
					Identifier: "matroska_chapters_too_close",
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
				Identifier: "matroska_chapters_exceed_duration",
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
					Identifier: "matroska_chapters_name_hygiene",
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
				Identifier: "matroska_chapters_name_hygiene",
				Warning:    "Chapter has no display name entry",
				Passed:     false,
				Severity:   "warning",
				Actual:     fmt.Sprintf("chapter %d (starts at %s)", i+1, formatNsToTime(ch.TimeStart)),
			}
		}

		hasNonEmpty, currentNames := checkSingleChapterNameHygiene(ch)

		if !hasNonEmpty {
			return &CheckResult{
				Identifier: "matroska_chapters_name_hygiene",
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
				Identifier: "matroska_chapters_language_hygiene",
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
			Identifier: "matroska_chapters_language_hygiene",
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

func isAligned(timeStart int64, keyframes []int64) (bool, int64) {
	closestDiff := int64(-1)

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
			return true, closestDiff
		}
	}

	return false, closestDiff
}

func getVideoTrackNumberFromEBML(ebml *matroska.EbmlMetadata) uint64 {
	for _, track := range ebml.Tracks {
		if track.Type == "video" {
			return uint64(track.Properties.Number)
		}
	}

	return 0
}

func checkChaptersKeyframeAlignment(filePath string, ebml *matroska.EbmlMetadata, chapters *matroska.Chapters) *CheckResult {
	if filePath == "" {
		return nil
	}

	videoTrackNum := getVideoTrackNumberFromEBML(ebml)
	if videoTrackNum == 0 {
		return nil
	}

	keyframes, err := matroska.ReadKeyframeTimestamps(filePath, videoTrackNum, ebml.Container.Properties.TimestampScale)
	if err != nil {
		return &CheckResult{
			Identifier: "matroska_chapters_keyframe_alignment",
			Warning:    "Failed to read video cues index (SeekHead/Cues may be missing or invalid)",
			Passed:     false,
			Severity:   "warning",
			Actual:     err.Error(),
		}
	}

	if len(keyframes) == 0 {
		return &CheckResult{
			Identifier: "matroska_chapters_keyframe_alignment",
			Warning:    "No video cues/index entries found (seeking might be slow or broken)",
			Passed:     false,
			Severity:   "warning",
			Actual:     "zero cues indexed for video track",
		}
	}

	if chapters == nil || len(chapters.Atoms) == 0 {
		return nil
	}

	var nonAligned []string

	for i, ch := range chapters.Atoms {
		if aligned, diff := isAligned(ch.TimeStart, keyframes); !aligned {
			nonAligned = append(nonAligned, fmt.Sprintf(
				"chapter %d at %s (nearest keyframe is off by %.3fs)",
				i+1,
				formatNsToTime(ch.TimeStart),
				float64(diff)/1e9,
			))
		}
	}

	if len(nonAligned) > 0 {
		return &CheckResult{
			Identifier: "matroska_chapters_keyframe_alignment",
			Warning:    "Chapters are not aligned with video keyframes",
			Passed:     false,
			Severity:   "warning",
			Actual:     strings.Join(nonAligned, "; "),
		}
	}

	return nil
}
