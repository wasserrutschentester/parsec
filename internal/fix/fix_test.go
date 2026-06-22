package fix

import (
	"testing"

	"codeberg.org/upPollo/parsec/internal/checks"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/ui"
)

func TestMergeTrackEdits(t *testing.T) {
	t.Parallel()

	base := []matroska.TrackEdit{
		{Number: 1, Props: map[string]string{"flag-default": "1"}},
		{Number: 2, Props: map[string]string{"name": "Foo"}},
	}
	extra := []matroska.TrackEdit{
		{Number: 2, Props: map[string]string{"language": "ger"}},  // merged into existing track 2
		{Number: 3, Props: map[string]string{"flag-forced": "1"}}, // appended as a new track
	}

	got := mergeTrackEdits(base, extra)

	if len(got) != 3 {
		t.Fatalf("expected 3 edits, got %d: %+v", len(got), got)
	}

	// Order is base order followed by new tracks from extra.
	wantOrder := []int{1, 2, 3}
	for i, edit := range got {
		if edit.Number != wantOrder[i] {
			t.Fatalf("edit %d has number %d, want %d", i, edit.Number, wantOrder[i])
		}
	}

	// Track 2 must carry both its base and extra properties.
	track2 := got[1]
	if track2.Props["name"] != "Foo" || track2.Props["language"] != "ger" {
		t.Errorf("track 2 props = %+v, want name=Foo and language=ger", track2.Props)
	}

	if got[2].Props["flag-forced"] != "1" {
		t.Errorf("track 3 props = %+v, want flag-forced=1", got[2].Props)
	}
}

func TestMergeTrackEditsEmptyExtra(t *testing.T) {
	t.Parallel()

	base := []matroska.TrackEdit{{Number: 1, Props: map[string]string{"flag-default": "1"}}}

	got := mergeTrackEdits(base, nil)
	if len(got) != 1 || got[0].Number != 1 {
		t.Fatalf("expected base returned unchanged, got %+v", got)
	}
}

func testMismatches() []keywordMismatch {
	return []keywordMismatch{
		{
			track: matroska.EbmlTrack{Properties: matroska.EbmlTrackProperties{Number: 1, Name: "Commentary"}},
			fix:   checks.KeywordFlagFix{Property: "flag-commentary", Keyword: "Commentary"},
		},
		{
			track: matroska.EbmlTrack{Properties: matroska.EbmlTrackProperties{Number: 2, Name: "Forced SDH"}},
			fix:   checks.KeywordFlagFix{Property: "flag-forced", Keyword: "Forced"},
		},
		{
			// Same track as above, a second mismatch -> must merge into one edit.
			track: matroska.EbmlTrack{Properties: matroska.EbmlTrackProperties{Number: 2, Name: "Forced SDH"}},
			fix:   checks.KeywordFlagFix{Property: "flag-hearing-impaired", Keyword: "SDH"},
		},
	}
}

func TestKeywordMismatchEditsAll(t *testing.T) {
	t.Parallel()

	edits := keywordMismatchEdits(testMismatches(), nil)
	if len(edits) != 2 {
		t.Fatalf("expected 2 edits (one per track), got %d: %+v", len(edits), edits)
	}

	if edits[0].Number != 1 || edits[0].Props["flag-commentary"] != "1" {
		t.Errorf("track 1 edit = %+v, want flag-commentary=1", edits[0])
	}

	track2 := edits[1]
	if track2.Number != 2 || track2.Props["flag-forced"] != "1" || track2.Props["flag-hearing-impaired"] != "1" {
		t.Errorf("track 2 edit = %+v, want both flag-forced and flag-hearing-impaired set", track2)
	}
}

func TestKeywordMismatchEditsSelected(t *testing.T) {
	t.Parallel()

	// Only the first mismatch (track 1) is selected.
	edits := keywordMismatchEdits(testMismatches(), map[int]bool{0: true})
	if len(edits) != 1 || edits[0].Number != 1 {
		t.Fatalf("expected only track 1's edit, got %+v", edits)
	}
}

func TestKeywordMismatchEditsNoneSelected(t *testing.T) {
	t.Parallel()

	if edits := keywordMismatchEdits(testMismatches(), map[int]bool{}); len(edits) != 0 {
		t.Fatalf("expected no edits when nothing is selected, got %+v", edits)
	}
}

func TestFormatContainerChange(t *testing.T) {
	t.Parallel()

	got := formatContainerChange("title", "Movie [1080p]", "")
	want := `Title: "Movie [1080p]" ` + ui.Muted.Render("->") + " " + quoteOrNone("")

	if got != want {
		t.Errorf("formatContainerChange() = %q, want %q", got, want)
	}
}

func TestContainerKeyLabel(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"title":                 "Title",
		"writing-application":   "Writing Application",
		"some-unknown-property": "some-unknown-property",
	}

	for key, want := range tests {
		if got := containerKeyLabel(key); got != want {
			t.Errorf("containerKeyLabel(%q) = %q, want %q", key, got, want)
		}
	}
}
