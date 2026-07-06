package correct

import (
	"codeberg.org/upPollo/parsec/internal/checks"
	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
)

// cleanSplitRegex tokenizes a track name for cleaning. Unlike wordSplitRegex it
// does not split on dots, so channel notations such as "5.1" stay intact.

// ComputeMatroskaFlagFixes inspects the tracks of a Matroska file and returns
// the track property edits required to satisfy the auto-fixable flag checks:
// default-flag and original-language assignment. Checks that are disabled in
// the configuration are skipped, mirroring RunMatroskaChecks. Kept separate
// from ComputeMatroskaNameFixes so the two can be previewed and confirmed
// independently.

// ComputeMatroskaNameFixes inspects the tracks of a Matroska file and returns
// the track name edits required to satisfy the auto-fixable name-quality
// checks: junk keyword, codec and redundant-language removal, plus missing
// keyword appension. Checks that are disabled in the configuration are
// skipped, mirroring RunMatroskaChecks.

// ComputeContainerFixes returns the segment-level ("info") property edits
// needed to satisfy the title-, writing-application-, and creation-time
// privacy checks. A matching property is cleared rather than rewritten with
// a guessed replacement, mirroring the conservative junk-removal approach
// used for track names. Checks that are disabled in the configuration are
// skipped.

// Creation time is usually handled separately, but we leave it here if it was here.
// Wait, the old code had `props["date"] = ""` here!
// Let's preserve that.

// ChapterAlignmentFix describes the timestamp corrections needed to align
// chapters with video keyframes, satisfying matroska_chapters_keyframe_alignment.
// Times is the full, ordered list of chapter start times (ns) to write back:
// mkvpropedit rewrites chapters by document position, so already-aligned
// chapters pass their original time through unchanged alongside the snapped
// ones. Changed counts how many entries actually differ from the original.
type ChapterAlignmentFix struct {
	Times   []int64
	Changed int
}

// ComputeChapterKeyframeSnaps returns the chapter timestamp corrections that
// align each misaligned chapter with its nearest video keyframe. Returns a
// zero-value ChapterAlignmentFix (Changed == 0) when the check is disabled,
// the file has no chapters, the video keyframe index can't be read, or
// nothing needs to change.
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

// extractChapterAtoms returns the chapter atoms to align. Real mkvmerge -J
// output never populates ebml.Chapters[0].Editions (it only reports
// num_entries), so in production this always extracts via mkvextract; the
// ebml-provided path exists so tests can inject known atoms without a real
// mkvextract round trip.
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
