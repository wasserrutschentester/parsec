package correct

import (
	"slices"
	"testing"

	"github.com/spf13/viper"

	"codeberg.org/upPollo/parsec/internal/checks"
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
func TestComputeUnusedFontAttachmentsDisabled(t *testing.T) {
	config.InitDefaults()
	viper.Set("disabled_checks", []string{"matroska_unused_fonts"})

	ebml := &matroska.EbmlMetadata{
		Attachments: []matroska.EbmlAttachment{{ID: 1, FileName: "Arial.ttf", ContentType: "font/ttf"}},
	}

	if got := ComputeUnusedFontAttachments(ebml, nil, nil); got != nil {
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

	attachmentFonts := []matroska.AttachmentFontInfo{
		{AttachmentID: 1, FamilyName: "Open Sans"},
		{AttachmentID: 2, FamilyName: "Calibri Bold"},
	}

	got := computeFontRenames(attachments, attachmentNames, attachmentFonts)

	if len(got) != 1 {
		t.Fatalf("expected 1 rename, got %d: %+v", len(got), got)
	}

	// NewName matches checks.ProposedFontFilename exactly (family-only
	// fallback strips spaces via cleanFallbackFontName), not the raw
	// internal name, so correct's rename target can never drift from what
	// matroska_font_filename_compliance shows as its "Proposed Name".
	want := FontRename{ID: 1, OldName: "font1.ttf", NewName: "OpenSans.ttf", InternalNames: []string{"Open Sans"}}
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

	attachmentFonts := []matroska.AttachmentFontInfo{
		{AttachmentID: 1, FamilyName: "Times New Roman"},
		{AttachmentID: 2, FamilyName: "Times New Roman"},
		{AttachmentID: 3, FamilyName: "Times New Roman"},
	}

	got := computeFontRenames(attachments, attachmentNames, attachmentFonts)

	if len(got) != 3 {
		t.Fatalf("expected 3 renames, got %d: %+v", len(got), got)
	}

	wantNames := []string{"TimesNewRoman.ttf", "TimesNewRoman (2).ttf", "TimesNewRoman (3).ttf"}
	for i, want := range wantNames {
		if got[i].NewName != want {
			t.Errorf("rename %d: NewName = %q, want %q", i, got[i].NewName, want)
		}
	}
}

// Regression test: when a font has a PostScript name, the rename target
// must use it (matching checks.ProposedFontFilename's priority), not the
// family name. Before this alignment, correct picked whichever name
// GetFontMapping listed first for the attachment, which put the family name
// ahead of the PostScript name - drifting from what the check itself
// proposed in its "Proposed Name" column.
func TestComputeFontRenamesPrefersPostScriptName(t *testing.T) {
	t.Parallel()

	attachments := []matroska.EbmlAttachment{
		{ID: 1, FileName: "font1.ttf", ContentType: "font/ttf"},
	}

	attachmentNames := map[int][]string{
		1: {"Open Sans", "OpenSans-Bold"},
	}

	attachmentFonts := []matroska.AttachmentFontInfo{
		{AttachmentID: 1, FamilyName: "Open Sans", PostScriptName: "OpenSans-Bold"},
	}

	got := computeFontRenames(attachments, attachmentNames, attachmentFonts)

	if len(got) != 1 || got[0].NewName != "OpenSans-Bold.ttf" {
		t.Errorf("expected rename to use the PostScript name, got %+v", got)
	}
}

func TestComputeFontRenamesExcludesUnusedAttachments(t *testing.T) {
	t.Parallel()

	attachments := []matroska.EbmlAttachment{
		{ID: 1, FileName: "font1.ttf", ContentType: "font/ttf"},
	}
	attachmentFonts := []matroska.AttachmentFontInfo{{AttachmentID: 1, FamilyName: "Open Sans"}}

	// font1.ttf isn't referenced by anything used, so it must be excluded
	// from rename candidates by ComputeFontRenames itself (via
	// excludeAttachments), matching matroska_unused_fonts' notion of unused.
	got := excludeAttachments(attachments, checks.UnusedFontAttachments(attachments, attachmentFonts, map[checks.FontStyle]bool{}))
	if len(got) != 0 {
		t.Errorf("expected unused attachment to be excluded, got %+v", got)
	}
}

// Regression test: the exported UnusedFontAttachments must match by
// family+weight+italic like the matroska_unused_fonts check does, not by
// family alone. A prior version of the fix-side export collapsed to
// family-only matching, which meant a Bold-only attachment was treated as
// "used" merely because some Regular-weight text referenced the same family,
// so correct silently disagreed with what check flagged as unused.
func TestUnusedFontAttachmentsMatchesByWeightAndItalic(t *testing.T) {
	t.Parallel()

	attachments := []matroska.EbmlAttachment{
		{ID: 1, FileName: "OpenSans-Bold.ttf", ContentType: "font/ttf"},
	}
	attachmentFonts := []matroska.AttachmentFontInfo{
		{AttachmentID: 1, FamilyName: "OpenSans", Weight: 700, Italic: false},
	}

	// Only the Regular (400) weight is actually used by any subtitle.
	usedFonts := map[checks.FontStyle]bool{
		{Family: "OpenSans", Weight: 400, Italic: false}: true,
	}

	got := checks.UnusedFontAttachments(attachments, attachmentFonts, usedFonts)
	if len(got) != 1 || got[0].ID != 1 {
		t.Errorf("expected the Bold attachment to be flagged unused despite matching family, got %+v", got)
	}
}

//nolint:paralleltest // depends on shared global config state
func TestComputeFontRenamesDisabled(t *testing.T) {
	config.InitDefaults()
	viper.Set("disabled_checks", []string{"matroska_font_filename_compliance"})

	ebml := &matroska.EbmlMetadata{
		Attachments: []matroska.EbmlAttachment{{ID: 1, FileName: "font1.ttf", ContentType: "font/ttf"}},
	}

	if got := ComputeFontRenames(ebml, nil, nil, nil); got != nil {
		t.Errorf("expected nil when matroska_font_filename_compliance is disabled, got %+v", got)
	}
}

type fakeFontResolver map[string]ResolvedFont

func (f fakeFontResolver) Resolve(fontName string, _ bool) (ResolvedFont, bool) {
	resolved, ok := f[fontName]

	return resolved, ok
}

func TestComputeMissingFontAttachmentPlan(t *testing.T) {
	t.Parallel()

	resolver := fakeFontResolver{
		"Open Sans": {
			Path:          "/fonts/OpenSans-Regular.ttf",
			Source:        fontSourceSystem,
			InternalNames: []string{"Open Sans", "Open Sans Regular"},
		},
	}

	plan := computeMissingFontAttachmentPlan(
		[]string{"Open Sans", "Unknown Font"},
		[]matroska.EbmlAttachment{{FileName: "Open Sans.ttf"}},
		resolver,
		true,
	)

	if len(plan.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %+v", plan.Attachments)
	}

	att := plan.Attachments[0]
	if att.FontName != "Open Sans" || att.AttachmentName != "Open Sans (2).ttf" || att.MIMEType != "font/ttf" || att.Source != fontSourceSystem {
		t.Errorf("unexpected attachment plan: %+v", att)
	}

	if !slices.Equal(plan.Unresolved, []string{"Unknown Font"}) {
		t.Errorf("unresolved = %+v, want Unknown Font", plan.Unresolved)
	}
}

func TestGoogleFontDirName(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"Open Sans":     "opensans",
		"Noto Sans JP":  "notosansjp",
		"Roboto Serif!": "robotoserif",
	}

	for input, want := range tests {
		if got := googleFontDirName(input); got != want {
			t.Errorf("googleFontDirName(%q) = %q, want %q", input, got, want)
		}
	}
}
