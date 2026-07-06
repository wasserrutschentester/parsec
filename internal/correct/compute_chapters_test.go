package correct

import (
	"os"
	"slices"
	"testing"

	"github.com/spf13/viper"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
)

// First of two German audio tracks lacks the default flag -> should be set.

// Specialized forced subtitle wrongly marked default -> should be cleared.

// Track 1 needs both a flag fix (first of two German tracks, missing
// default) and a name fix (redundant codec word), to verify the two
// computations stay disjoint.

// Map got items

// NewName matches checks.ProposedFontFilename exactly (family-only
// fallback strips spaces via cleanFallbackFontName), not the raw
// internal name, so correct's rename target can never drift from what
// matroska_font_filename_compliance shows as its "Proposed Name".

// Three distinct attachments all embedding the same font name.

// Regression test: when a font has a PostScript name, the rename target
// must use it (matching checks.ProposedFontFilename's priority), not the
// family name. Before this alignment, correct picked whichever name
// GetFontMapping listed first for the attachment, which put the family name
// ahead of the PostScript name - drifting from what the check itself
// proposed in its "Proposed Name" column.

// font1.ttf isn't referenced by anything used, so it must be excluded
// from rename candidates by ComputeFontRenames itself (via
// excludeAttachments), matching matroska_unused_fonts' notion of unused.

// Regression test: the exported UnusedFontAttachments must match by
// family+weight+italic like the matroska_unused_fonts check does, not by
// family alone. A prior version of the fix-side export collapsed to
// family-only matching, which meant a Bold-only attachment was treated as
// "used" merely because some Regular-weight text referenced the same family,
// so correct silently disagreed with what check flagged as unused.

// Only the Regular (400) weight is actually used by any subtitle.

func TestNearestKeyframe(t *testing.T) {
	t.Parallel()

	keyframes := []int64{0, 10_000_000_000, 20_000_000_000}

	tests := []struct {
		name      string
		timeStart int64
		want      int64
	}{
		{"exact match", 10_000_000_000, 10_000_000_000},
		{"closer to lower", 12_000_000_000, 10_000_000_000},
		{"closer to upper", 16_000_000_000, 20_000_000_000},
		{"before first keyframe", -5_000_000_000, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := nearestKeyframe(tt.timeStart, keyframes); got != tt.want {
				t.Errorf("nearestKeyframe(%d) = %d, want %d", tt.timeStart, got, tt.want)
			}
		})
	}
}

//nolint:paralleltest // depends on shared global config state
func TestComputeChapterKeyframeSnaps(t *testing.T) {
	config.InitDefaults()

	filePath := createMockCuesFile(t, []uint64{0, 10000, 20000})

	defer func() {
		_ = os.Remove(filePath)
	}()

	ebml := &matroska.EbmlMetadata{
		Tracks: []matroska.EbmlTrack{
			{ID: 1, Type: "video", Properties: matroska.EbmlTrackProperties{Number: 1}},
		},
		Chapters: []matroska.EbmlChapters{
			{
				Editions: []matroska.EbmlEdition{
					{
						Chapters: []matroska.EbmlChapterAtom{
							{TimeStart: 0},              // aligned
							{TimeStart: 16_000_000_000}, // misaligned, nearest is 20s
						},
					},
				},
			},
		},
	}

	fix := ComputeChapterKeyframeSnaps(filePath, ebml)

	if fix.Changed != 1 {
		t.Fatalf("expected 1 changed chapter, got %d (%+v)", fix.Changed, fix)
	}

	want := []int64{0, 20_000_000_000}
	if !slices.Equal(fix.Times, want) {
		t.Errorf("expected times %v, got %v", want, fix.Times)
	}
}

//nolint:paralleltest // depends on shared global config state
func TestComputeChapterKeyframeSnapsAligned(t *testing.T) {
	config.InitDefaults()

	filePath := createMockCuesFile(t, []uint64{0, 10000})

	defer func() {
		_ = os.Remove(filePath)
	}()

	ebml := &matroska.EbmlMetadata{
		Tracks: []matroska.EbmlTrack{
			{ID: 1, Type: "video", Properties: matroska.EbmlTrackProperties{Number: 1}},
		},
		Chapters: []matroska.EbmlChapters{
			{Editions: []matroska.EbmlEdition{{Chapters: []matroska.EbmlChapterAtom{{TimeStart: 0}}}}},
		},
	}

	if fix := ComputeChapterKeyframeSnaps(filePath, ebml); fix.Changed != 0 {
		t.Errorf("expected no changes for an already aligned chapter, got %+v", fix)
	}
}

//nolint:paralleltest // depends on shared global config state
func TestComputeChapterKeyframeSnapsDisabled(t *testing.T) {
	config.InitDefaults()
	viper.Set("disabled_checks", []string{"matroska_chapters_keyframe_alignment"})

	ebml := &matroska.EbmlMetadata{
		Tracks: []matroska.EbmlTrack{
			{ID: 1, Type: "video", Properties: matroska.EbmlTrackProperties{Number: 1}},
		},
		Chapters: []matroska.EbmlChapters{
			{Editions: []matroska.EbmlEdition{{Chapters: []matroska.EbmlChapterAtom{{TimeStart: 15_000_000_000}}}}},
		},
	}

	if fix := ComputeChapterKeyframeSnaps("", ebml); fix.Changed != 0 {
		t.Errorf("expected no changes when the check is disabled, got %+v", fix)
	}
}
