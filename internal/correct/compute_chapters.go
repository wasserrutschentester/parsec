package correct

import (
	"codeberg.org/upPollo/parsec/internal/checks"
	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
)

// ChapterAlignmentFix describes the timestamp corrections needed to align
// chapters with video keyframes.
// Times is the full list of chapter start times (ns) to write back.
type ChapterAlignmentFix struct {
	Times   []int64
	Changed int
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

	return ChapterAlignmentFix{Times: times, Changed: changed}
}

// extractChapterAtoms returns the chapter atoms to align.
// In production this extracts via mkvextract; tests can inject known atoms.
func extractChapterAtoms(filePath string, ebml *matroska.EbmlMetadata) []matroska.EbmlChapterAtom {
	if len(ebml.Chapters[0].Editions) > 0 {
		if chapters := ebml.Chapters[0].Editions[0].Chapters; len(chapters) > 0 {
			return chapters
		}
	}

	extracted, err := matroska.ExtractChapters(filePath)
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
