package fix

import (
	"testing"

	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
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
