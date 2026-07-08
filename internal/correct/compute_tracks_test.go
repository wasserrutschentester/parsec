package correct

import (
	"testing"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
)

func findEdit(edits []matroska.TrackEdit, number int) (matroska.TrackEdit, bool) {
	for _, e := range edits {
		if e.Number == number {
			return e, true
		}
	}

	return matroska.TrackEdit{}, false
}

func hasPropValueTest(props []matroska.TrackPropertyEdit, key, value string) bool {
	for _, p := range props {
		if p.Key == key && p.Value == value {
			return true
		}
	}

	return false
}

func hasPropKeyTest(props []matroska.TrackPropertyEdit, key string) bool {
	for _, p := range props {
		if p.Key == key {
			return true
		}
	}

	return false
}

//nolint:paralleltest // depends on shared global config state
func TestNeedsMultiLangName(t *testing.T) {
	config.InitDefaults()

	tests := []struct {
		name      string
		trackType string
		lang      string
		trackName string
		want      bool
	}{
		{"mul without name", "audio", "mul", "", true},
		{"mul with single language", "audio", "mul", "English", true},
		{"mul with two languages", "audio", "mul", "English German", false},
		{"non-mul empty name", "audio", "ger", "", false},
		{"mul on irrelevant track", "video", "mul", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			track := matroska.EbmlTrack{
				Type:       tt.trackType,
				Properties: matroska.EbmlTrackProperties{Language: tt.lang, Name: tt.trackName},
			}
			if got := NeedsMultiLangName(track); got != tt.want {
				t.Errorf("NeedsMultiLangName(%q, %q) = %v, want %v", tt.lang, tt.trackName, got, tt.want)
			}
		})
	}
}

//nolint:funlen,paralleltest // comprehensive test
func TestFixedTrackName(t *testing.T) {
	config.InitDefaults()

	tests := []struct {
		name  string
		track matroska.EbmlTrack
		want  string
	}{
		{
			name:  "removes simple codec keeps channel notation",
			track: matroska.EbmlTrack{Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "und", Name: "AC3 5.1"}},
			want:  "5.1",
		},
		{
			name:  "removes junk keyword and redundant language",
			track: matroska.EbmlTrack{Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Name: "German STEREO"}},
			want:  "",
		},
		{
			name:  "preserves DTS-HD compound token",
			track: matroska.EbmlTrack{Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "und", Name: "DTS-HD MA"}},
			want:  "DTS-HD MA",
		},
		{
			name:  "removes standalone DTS only",
			track: matroska.EbmlTrack{Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "und", Name: "Commentary DTS"}},
			want:  "Commentary",
		},
		{
			name:  "appends SDH keyword for hearing impaired flag",
			track: matroska.EbmlTrack{Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "und", Name: "Subtitles", HearingImpaired: true}},
			want:  "Subtitles SDH",
		},
		{
			name:  "leaves a clean name untouched",
			track: matroska.EbmlTrack{Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "und", Name: "Director Commentary", Commentary: true}},
			want:  "Commentary by Director Commentary",
		},
		{
			name:  "preserves existing commentary prefix",
			track: matroska.EbmlTrack{Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "und", Name: "Commentary by Jane Doe", Commentary: true}},
			want:  "Commentary by Jane Doe",
		},
		{
			name:  "prefixes commentary name after language context",
			track: matroska.EbmlTrack{Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "eng", Name: "English / Jane Doe", Commentary: true}},
			want:  "Commentary by Jane Doe",
		},
		{
			name:  "leaves bare commentary name unprefixed instead of fabricating attribution",
			track: matroska.EbmlTrack{Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "und", Name: "Commentary", Commentary: true}},
			want:  "Commentary",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fixedTrackName(tt.track); got != tt.want {
				t.Errorf("fixedTrackName() = %q, want %q", got, tt.want)
			}
		})
	}
}

//nolint:paralleltest // depends on shared global config state
func TestComputeMatroskaNameFixesCommentaryPairing(t *testing.T) {
	config.InitDefaults()

	tracks := []matroska.EbmlTrack{
		{
			Type: "audio",
			Properties: matroska.EbmlTrackProperties{
				Number:     1,
				Language:   "eng",
				Name:       "Jane Doe",
				Commentary: true,
			},
		},
		{
			Type: "subtitles",
			Properties: matroska.EbmlTrackProperties{
				Number:     2,
				Language:   "eng",
				Name:       "Different Commentary",
				Commentary: true,
			},
		},
	}

	edits := ComputeMatroskaNameFixes(tracks)

	audioEdit, ok := findEdit(edits, 1)
	if !ok || !hasPropValueTest(audioEdit.Properties, "name", "Commentary by Jane Doe") {
		t.Fatalf("expected audio commentary prefix edit, got %+v", edits)
	}

	subEdit, ok := findEdit(edits, 2)
	if !ok || !hasPropValueTest(subEdit.Properties, "name", "Commentary by Jane Doe") {
		t.Fatalf("expected subtitle commentary pairing edit, got %+v", edits)
	}
}

//nolint:paralleltest // depends on shared global config state
func TestComputeMatroskaNameFixesSkipsAmbiguousCommentaryPairing(t *testing.T) {
	config.InitDefaults()

	tracks := []matroska.EbmlTrack{
		{Type: "audio", Properties: matroska.EbmlTrackProperties{Number: 1, Language: "eng", Name: "Commentary by Jane", Commentary: true}},
		{Type: "audio", Properties: matroska.EbmlTrackProperties{Number: 2, Language: "eng", Name: "Commentary by John", Commentary: true}},
		{Type: "subtitles", Properties: matroska.EbmlTrackProperties{Number: 3, Language: "eng", Name: "Different", Commentary: true}},
	}

	edits := ComputeMatroskaNameFixes(tracks)
	edit, ok := findEdit(edits, 3)

	if !ok || !hasPropValueTest(edit.Properties, "name", "Commentary by Different") {
		t.Fatalf("expected only commentary prefix fix when pairing is ambiguous, got %+v", edits)
	}
}

//nolint:paralleltest // depends on shared global config state
func TestComputeMatroskaFixesDefaultFlag(t *testing.T) {
	config.InitDefaults()

	tracks := []matroska.EbmlTrack{
		// First of two German audio tracks lacks the default flag -> should be set.
		{Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Number: 1}},
		{Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Number: 2}},
		// Specialized forced subtitle wrongly marked default -> should be cleared.
		{Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", Forced: true, Name: "Forced", Default: true, Number: 3}},
	}

	edits := ComputeMatroskaFlagFixes(tracks)

	first, ok := findEdit(edits, 1)
	if !ok || !hasPropValueTest(first.Properties, "flag-default", "1") {
		t.Errorf("expected track 1 to gain flag-default=1, got %+v", edits)
	}

	third, ok := findEdit(edits, 3)
	if !ok || !hasPropValueTest(third.Properties, "flag-default", "0") {
		t.Errorf("expected track 3 to lose default flag (flag-default=0), got %+v", edits)
	}
}

//nolint:paralleltest // depends on shared global config state
func TestComputeMatroskaFixesOriginalFlag(t *testing.T) {
	config.InitDefaults()

	tracks := []matroska.EbmlTrack{
		{Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "eng", OriginalLanguage: true, Default: true, Number: 1}},
		{Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "eng", Number: 2}},
	}

	edits := ComputeMatroskaFlagFixes(tracks)

	second, ok := findEdit(edits, 2)
	if !ok || !hasPropValueTest(second.Properties, "flag-original", "1") {
		t.Errorf("expected track 2 to gain flag-original=1, got %+v", edits)
	}
}

//nolint:paralleltest // depends on shared global config state
func TestComputeMatroskaFlagAndNameFixesAreIndependent(t *testing.T) {
	config.InitDefaults()

	// Track 1 needs both a flag fix (first of two German tracks, missing
	// default) and a name fix (redundant codec word), to verify the two
	// computations stay disjoint.
	tracks := []matroska.EbmlTrack{
		{Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Name: "AC3 5.1", Number: 1}},
		{Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Number: 2}},
	}

	flagEdits := ComputeMatroskaFlagFixes(tracks)
	if edit, ok := findEdit(flagEdits, 1); !ok || !hasPropValueTest(edit.Properties, "flag-default", "1") {
		t.Fatalf("expected ComputeMatroskaFlagFixes to set flag-default=1, got %+v", flagEdits)
	} else if hasPropKeyTest(edit.Properties, "name") {
		t.Errorf("ComputeMatroskaFlagFixes must not include name edits, got %+v", edit)
	}

	nameEdits := ComputeMatroskaNameFixes(tracks)
	if edit, ok := findEdit(nameEdits, 1); !ok || !hasPropValueTest(edit.Properties, "name", "5.1") {
		t.Fatalf("expected ComputeMatroskaNameFixes to clean the name to \"5.1\", got %+v", nameEdits)
	} else if hasPropKeyTest(edit.Properties, "flag-default") {
		t.Errorf("ComputeMatroskaNameFixes must not include flag edits, got %+v", edit)
	}
}

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

// aligned
// misaligned, nearest is 20s

// preferred language is "de"

// German (preferred) audio should sort ahead of English.

// Same-language audio bloat (several tracks of one language, e.g. a lossless
// track plus a lossy variant) is intentionally NOT auto-removed for now; only
// unwanted-language audio is pruned. See the limitation noted in docs/fix.md.
//

// preferred language is "de"

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

// preferred language is "de"

//nolint:paralleltest // depends on shared global config state
func TestReverseKeywordFlagFixes(t *testing.T) {
	config.InitDefaults()

	track := matroska.EbmlTrack{Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "eng", Name: "English SDH"}}

	fixes := ReverseKeywordFlagFixes(track)
	if len(fixes) != 1 || fixes[0].Property != "flag-hearing-impaired" {
		t.Errorf("expected flag-hearing-impaired fix for 'SDH' in name, got %+v", fixes)
	}

	// When the flag is already set, there is no mismatch.
	track.Properties.HearingImpaired = true
	if fixes := ReverseKeywordFlagFixes(track); len(fixes) != 0 {
		t.Errorf("expected no fixes when flag already set, got %+v", fixes)
	}
}
