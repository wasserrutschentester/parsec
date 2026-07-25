package correct

import (
	"strings"

	"codeberg.org/upPollo/parsec/internal/checks"
	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
)

var extractChaptersMetadata = matroska.ExtractChapters

// ChapterLanguageFix describes missing chapter display languages to write in
// document order. Empty entries leave the corresponding display unchanged.
type ChapterLanguageFix struct {
	Languages   []string `json:"languages"`
	Changed     int      `json:"changed"`
	NeedsPrompt bool     `json:"needs_prompt"`
}

func getChapterLanguages(chapters []matroska.EbmlChapterAtom) ([]matroska.EbmlDisplay, map[string]bool) {
	var displays []matroska.EbmlDisplay

	known := make(map[string]bool)

	for _, chapter := range chapters {
		for _, display := range chapter.Display {
			displays = append(displays, display)

			lang := strings.ToLower(strings.TrimSpace(display.Language))
			if lang != "" && lang != "und" {
				known[lang] = true
			}
		}
	}

	return displays, known
}

func countMissingLanguages(displays []matroska.EbmlDisplay) int {
	missing := 0

	for _, display := range displays {
		lang := strings.TrimSpace(display.Language)
		if lang == "" || strings.EqualFold(lang, "und") {
			missing++
		}
	}

	return missing
}

// ComputeChapterLanguageFix fills missing chapter display languages when all
// existing display languages agree. Ambiguous files require user input.
func ComputeChapterLanguageFix(filePath string, ebml *matroska.EbmlMetadata) ChapterLanguageFix {
	if !config.IsCheckEnabled(config.CheckMatroskaChaptersLanguageHygiene) {
		return ChapterLanguageFix{}
	}

	chapters := extractChapterAtoms(filePath, ebml)
	displays, known := getChapterLanguages(chapters)

	if countMissingLanguages(displays) == 0 {
		return ChapterLanguageFix{}
	}

	fix := ChapterLanguageFix{Languages: make([]string, len(displays))}

	if len(known) != 1 {
		fix.NeedsPrompt = true

		return fix
	}

	var inferred string
	for lang := range known {
		inferred = lang
	}

	for i, display := range displays {
		lang := strings.TrimSpace(display.Language)
		if lang == "" || strings.EqualFold(lang, "und") {
			fix.Languages[i] = inferred
			fix.Changed++
		}
	}

	return fix
}

// ChapterLanguageFixForLanguage fills every missing chapter display language
// with a language explicitly selected by the user.
func ChapterLanguageFixForLanguage(filePath string, ebml *matroska.EbmlMetadata, language string) ChapterLanguageFix {
	language = strings.TrimSpace(language)
	if language == "" {
		return ChapterLanguageFix{}
	}

	var fix ChapterLanguageFix

	for _, chapter := range extractChapterAtoms(filePath, ebml) {
		for _, display := range chapter.Display {
			lang := strings.TrimSpace(display.Language)
			if lang == "" || strings.EqualFold(lang, "und") {
				fix.Languages = append(fix.Languages, language)
				fix.Changed++
			} else {
				fix.Languages = append(fix.Languages, "")
			}
		}
	}

	return fix
}

// ChapterAlignmentFix describes the timestamp corrections needed to align
// chapters with video keyframes.
// Times is the full list of chapter start times (ns) to write back.
type ChapterAlignmentFix struct {
	Times   []int64            `json:"times"`
	Changed int                `json:"changed"`
	Events  []ChapterSnapEvent `json:"events"`
}

// ComputeChapterKeyframeSnaps returns the chapter timestamp corrections that
// align each misaligned chapter with its nearest video keyframe.
func ComputeChapterKeyframeSnaps(filePath string, ebml *matroska.EbmlMetadata) ChapterAlignmentFix {
	if !config.IsCheckEnabled(config.CheckMatroskaChaptersKeyframeAlignment) {
		return ChapterAlignmentFix{}
	}

	if len(ebml.Chapters) != 1 {
		return ChapterAlignmentFix{}
	}

	var videoTrackNum uint64
	if track := checks.GetVideoTrackFromEBML(ebml); track != nil {
		videoTrackNum = uint64(track.Properties.Number)
	}

	if videoTrackNum == 0 {
		return ChapterAlignmentFix{}
	}

	chapters := extractChapterAtoms(filePath, ebml)
	if len(chapters) == 0 {
		return ChapterAlignmentFix{}
	}

	keyframes, err := matroska.ReadKeyframeTimestamps(filePath, videoTrackNum, ebml.Container.Properties.TimestampScale)
	if err != nil || len(keyframes) == 0 {
		return ChapterAlignmentFix{}
	}

	times, changed := snapChaptersToKeyframes(chapters, keyframes)
	if changed == 0 {
		return ChapterAlignmentFix{}
	}

	var defaultDuration int64
	if track := checks.GetVideoTrackFromEBML(ebml); track != nil {
		defaultDuration = track.Properties.DefaultDuration
	}

	events := buildChapterSnapEvents(chapters, keyframes, defaultDuration)

	return ChapterAlignmentFix{Times: times, Changed: changed, Events: events}
}

func buildChapterSnapEvents(chapters []matroska.EbmlChapterAtom, keyframes []int64, defaultDuration int64) []ChapterSnapEvent {
	var events []ChapterSnapEvent

	for i, ch := range chapters {
		aligned, _, prevKF, nextKF := checks.IsAligned(ch.TimeStart, keyframes)
		if !aligned {
			name := "-"
			if len(ch.Display) > 0 && ch.Display[0].String != "" {
				name = ch.Display[0].String
			}

			events = append(events, ChapterSnapEvent{
				ChapterNum:       i + 1,
				Name:             name,
				OriginalTime:     ch.TimeStart,
				PreviousKeyframe: prevKF,
				NextKeyframe:     nextKF,
				DefaultDuration:  defaultDuration,
			})
		}
	}

	return events
}

// extractChapterAtoms returns the chapter atoms to align.
// In production this extracts via mkvextract; tests can inject known atoms.
func extractChapterAtoms(filePath string, ebml *matroska.EbmlMetadata) []matroska.EbmlChapterAtom {
	if len(ebml.Chapters) > 0 && len(ebml.Chapters[0].Editions) > 0 {
		if chapters := ebml.Chapters[0].Editions[0].Chapters; len(chapters) > 0 {
			return chapters
		}
	}

	extracted, err := extractChaptersMetadata(filePath)
	if err != nil || extracted == nil {
		return nil
	}

	return extracted.Atoms
}

// snapChaptersToKeyframes returns, in chapter order, each chapter's start
// time snapped to its nearest keyframe when misaligned, or unchanged when
// already aligned, plus how many entries were actually snapped.
func snapChaptersToKeyframes(chapters []matroska.EbmlChapterAtom, keyframes []int64) ([]int64, int) {
	times := make([]int64, len(chapters))
	changed := 0

	for i, ch := range chapters {
		aligned, _, _, _ := checks.IsAligned(ch.TimeStart, keyframes)
		if aligned {
			times[i] = ch.TimeStart

			continue
		}

		times[i] = nearestKeyframe(ch.TimeStart, keyframes)
		changed++
	}

	return times, changed
}

// nearestKeyframe returns the keyframe timestamp closest to timeStart.
// keyframes must be non-empty.
func nearestKeyframe(timeStart int64, keyframes []int64) int64 {
	best := keyframes[0]
	bestDiff := absInt64(timeStart - best)

	for _, kf := range keyframes[1:] {
		if diff := absInt64(timeStart - kf); diff < bestDiff {
			bestDiff = diff
			best = kf
		}
	}

	return best
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}

	return v
}
