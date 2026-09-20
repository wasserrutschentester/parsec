package correct

import (
	"testing"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
)

func TestMergeTrackEdits(t *testing.T) {
	t.Parallel()

	base := []matroska.TrackEdit{
		{Number: 1, Properties: []matroska.TrackPropertyEdit{{Key: "flag-default", Value: "1"}}},
		{Number: 2, Properties: []matroska.TrackPropertyEdit{{Key: "name", Value: "Foo"}}},
	}
	extra := []matroska.TrackEdit{
		{Number: 2, Properties: []matroska.TrackPropertyEdit{{Key: "language", Value: "ger"}}},
		{Number: 3, Properties: []matroska.TrackPropertyEdit{{Key: "flag-forced", Value: "1"}}},
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
	if !hasPropValue(track2.Properties, "name", "Foo") || !hasPropValue(track2.Properties, "language", "ger") {
		t.Errorf("track 2 props = %+v, want name=Foo and language=ger", track2.Properties)
	}

	if !hasPropValue(got[2].Properties, "flag-forced", "1") {
		t.Errorf("track 3 props = %+v, want flag-forced=1", got[2].Properties)
	}
}

func hasPropValue(props []matroska.TrackPropertyEdit, key, value string) bool {
	for _, p := range props {
		if p.Key == key && p.Value == value {
			return true
		}
	}

	return false
}

func TestMergeTrackEditsEmptyExtra(t *testing.T) {
	t.Parallel()

	base := []matroska.TrackEdit{{Number: 1, Properties: []matroska.TrackPropertyEdit{{Key: "flag-default", Value: "1"}}}}

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

	if edits[0].Number != 1 || !hasPropValue(edits[0].Properties, "flag-commentary", "1") {
		t.Errorf("track 1 edit = %+v, want flag-commentary=1", edits[0])
	}

	track2 := edits[1]
	if track2.Number != 2 || !hasPropValue(track2.Properties, "flag-forced", "1") || !hasPropValue(track2.Properties, "flag-hearing-impaired", "1") {
		t.Errorf("track 2 edit = %+v, want both flag-forced and flag-hearing-impaired set", track2)
	}
}
