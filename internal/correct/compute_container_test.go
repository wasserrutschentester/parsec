package correct

import (
	"testing"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
)

//nolint:paralleltest // depends on shared global config state

//nolint:funlen,paralleltest // comprehensive table of name-cleaning cases; depends on shared global config state

//nolint:paralleltest // depends on shared global config state

//nolint:paralleltest // depends on shared global config state

//nolint:paralleltest // depends on shared global config state

// First of two German audio tracks lacks the default flag -> should be set.

// Specialized forced subtitle wrongly marked default -> should be cleared.

//nolint:paralleltest // depends on shared global config state

//nolint:paralleltest // depends on shared global config state

// Track 1 needs both a flag fix (first of two German tracks, missing
// default) and a name fix (redundant codec word), to verify the two
// computations stay disjoint.

//nolint:funlen,paralleltest // table-driven policy coverage; depends on shared global config state
func TestComputeContainerFixes(t *testing.T) {
	config.InitDefaults()

	tests := []struct {
		name  string
		ebml  *matroska.EbmlMetadata
		props map[string]string
	}{
		{
			name: "junk title cleared",
			ebml: &matroska.EbmlMetadata{Container: matroska.EbmlContainer{
				Properties: matroska.EbmlContainerProperties{Title: "Movie [1080p] x265"},
			}},
			props: map[string]string{"title": ""},
		},
		{
			name: "clean title untouched",
			ebml: &matroska.EbmlMetadata{Container: matroska.EbmlContainer{
				Properties: matroska.EbmlContainerProperties{Title: "Movie"},
			}},
			props: map[string]string{},
		},
		{
			name: "writing application path cleared",
			ebml: &matroska.EbmlMetadata{Container: matroska.EbmlContainer{
				Properties: matroska.EbmlContainerProperties{WritingApplication: `C:\Users\someone\tool.exe`},
			}},
			props: map[string]string{"writing-application": ""},
		},
		{
			name: "writing application without identifiable info untouched",
			ebml: &matroska.EbmlMetadata{Container: matroska.EbmlContainer{
				Properties: matroska.EbmlContainerProperties{WritingApplication: "mkvmerge v80.0"},
			}},
			props: map[string]string{},
		},
		{
			name: "title matching parsed metadata untouched",
			ebml: &matroska.EbmlMetadata{Container: matroska.EbmlContainer{
				Properties: matroska.EbmlContainerProperties{Title: "Movie (2020)"},
			}},
			props: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := (*metadata.Metadata)(nil)
			if tt.name == "title matching parsed metadata untouched" {
				meta = &metadata.Metadata{Title: "Movie (2020)"}
			}

			got := ComputeContainerFixes(tt.ebml, meta)
			if len(got) != len(tt.props) {
				t.Fatalf("ComputeContainerFixes() = %+v, want %+v", got, tt.props)
			}

			// Map got items
			gotMap := make(map[string]string)
			for _, edit := range got {
				gotMap[edit.Key] = edit.NewValue
			}

			for k, v := range tt.props {
				if gotMap[k] != v {
					t.Errorf("ComputeContainerFixes()[%q] = %q, want %q", k, gotMap[k], v)
				}
			}
		})
	}
}

//nolint:paralleltest // depends on shared global config state

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

//nolint:paralleltest // depends on shared global config state

//nolint:paralleltest // depends on shared global config state

// aligned
// misaligned, nearest is 20s

//nolint:paralleltest // depends on shared global config state

//nolint:paralleltest // depends on shared global config state

//nolint:paralleltest // depends on shared global config state

// preferred language is "de"

// German (preferred) audio should sort ahead of English.

//nolint:paralleltest // depends on shared global config state

// Same-language audio bloat (several tracks of one language, e.g. a lossless
// track plus a lossy variant) is intentionally NOT auto-removed for now; only
// unwanted-language audio is pruned. See the limitation noted in docs/fix.md.
//
//nolint:paralleltest // depends on shared global config state

// preferred language is "de"

//nolint:paralleltest // depends on shared global config state

// preferred language is "de"

// preferred
// original
// unwanted
// no dialogue: kept

// Without the original language, nothing is pruned (safe default).

// With Japanese as the original language, only French is unwanted; the
// 'zxx' (no linguistic content) track must never be flagged for removal.

// A track tagged with a region subtag (e.g. "de-DE") must still be recognized
// as the preferred/original language, matching by base subtag exactly like
// the mdb_unwanted_audio_lang check does (checks.IsWantedAudioLang). Regression
// test for a prior exact-tag comparison that disagreed with the check and
// proposed the preferred-language track itself for removal.
//
//nolint:paralleltest // depends on shared global config state

// preferred language is "de"

//nolint:paralleltest // depends on shared global config state

//nolint:paralleltest // depends on shared global config state

//nolint:paralleltest // depends on shared global config state

// When the flag is already set, there is no mismatch.
