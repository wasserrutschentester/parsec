package fix

import (
	"strings"
	"testing"

	"codeberg.org/upPollo/parsec/internal/config"
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

//nolint:paralleltest // depends on shared global config state
func TestBuildKeywordFlagEdits(t *testing.T) {
	config.InitDefaults()

	ebml := &matroska.EbmlMetadata{
		Tracks: []matroska.EbmlTrack{
			{Type: "audio", Properties: matroska.EbmlTrackProperties{Number: 1, Name: "Commentary"}},
			// Track 2 has two mismatches (Forced + SDH) and must produce one consolidated edit.
			{Type: "audio", Properties: matroska.EbmlTrackProperties{Number: 2, Name: "Forced SDH"}},
		},
	}

	edits := buildKeywordFlagEdits(ebml)
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

func TestFormatContainerChange(t *testing.T) {
	t.Parallel()

	got := formatContainerChange("title", "Movie [1080p]", "")
	want := `Title: "Movie [1080p]" ` + ui.Muted.Render("->") + " " + quoteOrNone("")

	if got != want {
		t.Errorf("formatContainerChange() = %q, want %q", got, want)
	}
}

func TestLongestCommonSubsequence(t *testing.T) {
	t.Parallel()

	got := longestCommonSubsequence([]int{0, 1, 2, 3}, []int{0, 2, 1, 3})
	want := []int{0, 1, 3} // or {0, 2, 3}; both are valid LCS of equal length

	if len(got) != len(want) {
		t.Fatalf("longestCommonSubsequence() = %v, want length %d", got, len(want))
	}
}

func TestMisplacedTrackIDs(t *testing.T) {
	t.Parallel()

	// Track 2 (subtitle, out of place before video/audio) moves to the end;
	// tracks 0, 1 and 3 keep their relative order and just shift index.
	oldOrder := []int{2, 0, 1, 3}
	newOrder := []int{0, 1, 3, 2}

	got := misplacedTrackIDs(oldOrder, newOrder)

	if !got[2] {
		t.Errorf("expected track 2 to be flagged as misplaced, got %v", got)
	}

	for _, id := range []int{0, 1, 3} {
		if got[id] {
			t.Errorf("track %d should not be flagged as misplaced (it only shifted), got %v", id, got)
		}
	}
}

func TestTrackOrderTable(t *testing.T) {
	t.Parallel()

	ebml := &matroska.EbmlMetadata{
		Tracks: []matroska.EbmlTrack{
			{ID: 2, Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "eng"}},
			{ID: 0, Type: "video"},
			{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger"}},
		},
	}

	table := trackOrderTable(ebml, []int{0, 1, 2})

	for _, want := range []string{"video", "audio", "subtitles"} {
		if !strings.Contains(table, want) {
			t.Errorf("expected rendered table to mention %q, got:\n%s", want, table)
		}
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
