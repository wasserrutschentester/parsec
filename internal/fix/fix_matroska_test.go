package fix

import (
	"os"
	"slices"
	"testing"

	"github.com/spf13/viper"

	"codeberg.org/upPollo/parsec/internal/checks"
	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
)

func findEdit(edits []matroska.TrackEdit, number int) (matroska.TrackEdit, bool) {
	for _, edit := range edits {
		if edit.Number == number {
			return edit, true
		}
	}

	return matroska.TrackEdit{}, false
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

//nolint:paralleltest // depends on shared global config state
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
			want:  "Director Commentary",
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
	if !ok || first.Props["flag-default"] != "1" {
		t.Errorf("expected track 1 to gain flag-default=1, got %+v", edits)
	}

	third, ok := findEdit(edits, 3)
	if !ok || third.Props["flag-default"] != "0" {
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
	if !ok || second.Props["flag-original"] != "1" {
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
	if edit, ok := findEdit(flagEdits, 1); !ok || edit.Props["flag-default"] != "1" {
		t.Fatalf("expected ComputeMatroskaFlagFixes to set flag-default=1, got %+v", flagEdits)
	} else if _, hasName := edit.Props["name"]; hasName {
		t.Errorf("ComputeMatroskaFlagFixes must not include name edits, got %+v", edit)
	}

	nameEdits := ComputeMatroskaNameFixes(tracks)
	if edit, ok := findEdit(nameEdits, 1); !ok || edit.Props["name"] != "5.1" {
		t.Fatalf("expected ComputeMatroskaNameFixes to clean the name to \"5.1\", got %+v", nameEdits)
	} else if _, hasFlag := edit.Props["flag-default"]; hasFlag {
		t.Errorf("ComputeMatroskaNameFixes must not include flag edits, got %+v", edit)
	}
}

//nolint:paralleltest // depends on shared global config state
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeContainerFixes(tt.ebml)
			if len(got) != len(tt.props) {
				t.Fatalf("ComputeContainerFixes() = %+v, want %+v", got, tt.props)
			}

			for k, v := range tt.props {
				if got[k] != v {
					t.Errorf("ComputeContainerFixes()[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

//nolint:paralleltest // depends on shared global config state
func TestComputeUnusedFontAttachmentsDisabled(t *testing.T) {
	config.InitDefaults()
	viper.Set("disabled_checks", []string{"matroska_unused_fonts"})

	ebml := &matroska.EbmlMetadata{
		Attachments: []matroska.EbmlAttachment{{ID: 1, FileName: "Arial.ttf", ContentType: "font/ttf"}},
	}

	if got := ComputeUnusedFontAttachments("", ebml); got != nil {
		t.Errorf("expected nil when matroska_unused_fonts is disabled, got %+v", got)
	}
}

func TestComputeFontRenames(t *testing.T) {
	t.Parallel()

	attachments := []matroska.EbmlAttachment{
		{ID: 1, FileName: "font1.ttf", ContentType: "font/ttf"},
		{ID: 2, FileName: "Calibri-Bold.ttf", ContentType: "font/ttf"},
		{ID: 3, FileName: "subs.ass"},
	}

	attachmentNames := map[int][]string{
		1: {"Open Sans"},
		2: {"Calibri Bold"},
	}

	got := computeFontRenames(attachments, attachmentNames)

	if len(got) != 1 {
		t.Fatalf("expected 1 rename, got %d: %+v", len(got), got)
	}

	want := FontRename{ID: 1, OldName: "font1.ttf", NewName: "Open Sans.ttf", InternalNames: []string{"Open Sans"}}
	if got[0].ID != want.ID || got[0].OldName != want.OldName || got[0].NewName != want.NewName || !slices.Equal(got[0].InternalNames, want.InternalNames) {
		t.Errorf("expected %+v, got %+v", want, got[0])
	}
}

func TestComputeFontRenamesDisambiguatesCollisions(t *testing.T) {
	t.Parallel()

	attachments := []matroska.EbmlAttachment{
		{ID: 1, FileName: "font1.ttf", ContentType: "font/ttf"},
		{ID: 2, FileName: "font2.ttf", ContentType: "font/ttf"},
		{ID: 3, FileName: "font3.ttf", ContentType: "font/ttf"},
	}

	// Three distinct attachments all embedding the same font name.
	attachmentNames := map[int][]string{
		1: {"Times New Roman"},
		2: {"Times New Roman"},
		3: {"Times New Roman"},
	}

	got := computeFontRenames(attachments, attachmentNames)

	if len(got) != 3 {
		t.Fatalf("expected 3 renames, got %d: %+v", len(got), got)
	}

	wantNames := []string{"Times New Roman.ttf", "Times New Roman (2).ttf", "Times New Roman (3).ttf"}
	for i, want := range wantNames {
		if got[i].NewName != want {
			t.Errorf("rename %d: NewName = %q, want %q", i, got[i].NewName, want)
		}
	}
}

func TestComputeFontRenamesExcludesUnusedAttachments(t *testing.T) {
	t.Parallel()

	attachments := []matroska.EbmlAttachment{
		{ID: 1, FileName: "font1.ttf", ContentType: "font/ttf"},
	}
	attachmentNames := map[int][]string{1: {"Open Sans"}}

	// font1.ttf isn't referenced by anything used, so it must be excluded
	// from rename candidates by ComputeFontRenames itself (via
	// excludeAttachments), matching matroska_unused_fonts' notion of unused.
	got := excludeAttachments(attachments, checks.UnusedFontAttachments(attachments, attachmentNames, map[string]bool{}))
	if len(got) != 0 {
		t.Errorf("expected unused attachment to be excluded, got %+v", got)
	}
}

//nolint:paralleltest // depends on shared global config state
func TestComputeFontRenamesDisabled(t *testing.T) {
	config.InitDefaults()
	viper.Set("disabled_checks", []string{"matroska_font_filename_compliance"})

	ebml := &matroska.EbmlMetadata{
		Attachments: []matroska.EbmlAttachment{{ID: 1, FileName: "font1.ttf", ContentType: "font/ttf"}},
	}

	if got := ComputeFontRenames("", ebml); got != nil {
		t.Errorf("expected nil when matroska_font_filename_compliance is disabled, got %+v", got)
	}
}

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

//nolint:paralleltest // depends on shared global config state
func TestComputeMatroskaRemuxTrackOrder(t *testing.T) {
	config.InitDefaults() // preferred language is "de"

	tracks := []matroska.EbmlTrack{
		{ID: 0, Type: "video"},
		{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "eng", Default: true}},
		{ID: 2, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true}},
	}

	plan := ComputeMatroskaRemux(tracks, "")

	// German (preferred) audio should sort ahead of English.
	want := []int{0, 2, 1}
	if !slices.Equal(plan.TrackOrder, want) {
		t.Errorf("expected track order %v, got %v", want, plan.TrackOrder)
	}
}

//nolint:paralleltest // depends on shared global config state
func TestComputeMatroskaRemuxCompressionAndDuplicates(t *testing.T) {
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

	if len(plan.RemovalCandidates) != 1 || plan.RemovalCandidates[0].TrackID != 2 {
		t.Errorf("expected track 2 flagged as duplicate, got %+v", plan.RemovalCandidates)
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
