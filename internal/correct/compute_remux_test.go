package correct

import (
	"slices"
	"testing"

	"github.com/spf13/viper"

	"codeberg.org/upPollo/parsec/internal/config"
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

// Map got items

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
func TestComputeMatroskaRemuxTrackOrder(t *testing.T) {
	config.InitDefaults() // preferred language is "de"

	tracks := []matroska.EbmlTrack{
		{ID: 0, Type: "video"},
		{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "eng", Default: true, AudioChannels: 2}},
		{ID: 2, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, AudioChannels: 2}},
	}

	plan := ComputeMatroskaRemux(tracks, "")

	// German (preferred) audio should sort ahead of English.
	want := []int{0, 2, 1}
	if !slices.Equal(plan.TrackOrder, want) {
		t.Errorf("expected track order %v, got %v", want, plan.TrackOrder)
	}
}

//nolint:paralleltest // depends on shared global config state
func TestComputeMatroskaRemuxCompressionDoesNotRemoveLooseDuplicates(t *testing.T) {
	config.InitDefaults()

	tracks := []matroska.EbmlTrack{
		{ID: 0, Type: "video"},
		{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, ContentEncodingAlgorithms: "0", AudioChannels: 2}},
		{ID: 2, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, AudioChannels: 2}},
	}

	plan := ComputeMatroskaRemux(tracks, "")

	if !slices.Equal(plan.StripCompressionIDs, []int{1}) {
		t.Errorf("expected compression strip on track 1, got %v", plan.StripCompressionIDs)
	}

	if len(plan.RemovalCandidates) != 0 {
		t.Errorf("expected no removals from loose duplicate metadata, got %+v", plan.RemovalCandidates)
	}
}

// Same-language audio bloat (several tracks of one language, e.g. a lossless
// track plus a lossy variant) is intentionally NOT auto-removed for now; only
// unwanted-language audio is pruned. See the limitation noted in docs/fix.md.
//
//nolint:paralleltest // depends on shared global config state
func TestComputeMatroskaRemuxKeepsSameLanguageAudio(t *testing.T) {
	config.InitDefaults() // preferred language is "de"

	tracks := []matroska.EbmlTrack{
		{ID: 0, Type: "video"},
		{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Name: "TrueHD", AudioChannels: 6}},
		{ID: 2, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Name: "AC-3", AudioChannels: 6}},
	}

	if got := ComputeMatroskaRemux(tracks, "").RemovalCandidates; len(got) != 0 {
		t.Errorf("expected no removals for same-language audio, got %+v", got)
	}
}

//nolint:paralleltest // depends on shared global config state
func TestComputeMatroskaRemuxUnwantedLanguage(t *testing.T) {
	config.InitDefaults() // preferred language is "de"

	tracks := []matroska.EbmlTrack{
		{ID: 0, Type: "video"},
		{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, AudioChannels: 6}}, // preferred
		{ID: 2, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "jpn", AudioChannels: 2}},                // original
		{ID: 3, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "fra", AudioChannels: 2}},                // unwanted
		{ID: 4, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "zxx", AudioChannels: 2}},                // no dialogue: kept
	}

	// Without the original language, nothing is pruned (safe default).
	if got := ComputeMatroskaRemux(tracks, "").RemovalCandidates; len(got) != 0 {
		t.Errorf("expected no removals without original language, got %+v", got)
	}

	// With Japanese as the original language, only French is unwanted; the
	// 'zxx' (no linguistic content) track must never be flagged for removal.
	plan := ComputeMatroskaRemux(tracks, "jpn")
	if len(plan.RemovalCandidates) != 1 || plan.RemovalCandidates[0].TrackID != 3 {
		t.Errorf("expected only track 3 (fra) flagged as unwanted, got %+v", plan.RemovalCandidates)
	}

	if plan.RemovalCandidates[0].Kind != RemovalUnwantedAudioLang {
		t.Errorf("expected unwanted audio language removal kind, got %q", plan.RemovalCandidates[0].Kind)
	}
}

// A track tagged with a region subtag (e.g. "de-DE") must still be recognized
// as the preferred/original language, matching by base subtag exactly like
// the mdb_unwanted_audio_lang check does (checks.IsWantedAudioLang). Regression
// test for a prior exact-tag comparison that disagreed with the check and
// proposed the preferred-language track itself for removal.
//
//nolint:paralleltest // depends on shared global config state
func TestComputeMatroskaRemuxKeepsRegionTaggedPreferredLanguage(t *testing.T) {
	config.InitDefaults() // preferred language is "de"

	tracks := []matroska.EbmlTrack{
		{ID: 0, Type: "video"},
		{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "de-DE", Default: true, AudioChannels: 6}},
	}

	if got := ComputeMatroskaRemux(tracks, "jpn").RemovalCandidates; len(got) != 0 {
		t.Errorf("expected region-tagged preferred-language track to be kept, got %+v", got)
	}
}

//nolint:paralleltest // depends on shared global config state
func TestComputeMatroskaRemuxEmptyAudioTrack(t *testing.T) {
	config.InitDefaults()

	tracks := []matroska.EbmlTrack{
		{ID: 0, Type: "video"},
		{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, AudioChannels: 6}},
		{ID: 2, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", AudioChannels: 0}},
	}

	plan := ComputeMatroskaRemux(tracks, "")
	if len(plan.RemovalCandidates) != 1 || plan.RemovalCandidates[0].TrackID != 2 {
		t.Errorf("expected track 2 flagged as empty, got %+v", plan.RemovalCandidates)
	}

	if plan.RemovalCandidates[0].Kind != RemovalEmptyTrack {
		t.Errorf("expected empty track removal kind, got %q", plan.RemovalCandidates[0].Kind)
	}
}

//nolint:paralleltest // depends on shared global config state
func TestComputeMatroskaRemuxEmptyAudioTrackDisabled(t *testing.T) {
	config.InitDefaults()
	viper.Set("disabled_checks", []string{"mediainfo_empty_tracks"})

	tracks := []matroska.EbmlTrack{
		{ID: 0, Type: "video"},
		{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", AudioChannels: 0}},
	}

	if got := ComputeMatroskaRemux(tracks, "").RemovalCandidates; len(got) != 0 {
		t.Errorf("expected no removals when mediainfo_empty_tracks is disabled, got %+v", got)
	}
}

//nolint:paralleltest // depends on shared global config state

// When the flag is already set, there is no mismatch.
