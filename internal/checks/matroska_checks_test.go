package checks

import (
	"slices"
	"strings"
	"testing"

	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
)

func TestContainerCreationTimeNeedsFix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ebml *matroska.EbmlMetadata
		want bool
	}{
		{
			name: "no dates set",
			ebml: &matroska.EbmlMetadata{},
			want: false,
		},
		{
			name: "DateUtc set",
			ebml: &matroska.EbmlMetadata{Container: matroska.EbmlContainer{Properties: matroska.EbmlContainerProperties{DateUtc: "2024-01-01T00:00:00Z"}}},
			want: true,
		},
		{
			name: "DateLocal set",
			ebml: &matroska.EbmlMetadata{Container: matroska.EbmlContainer{Properties: matroska.EbmlContainerProperties{DateLocal: "2024-01-01T00:00:00+01:00"}}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := ContainerCreationTimeNeedsFix(tt.ebml); got != tt.want {
				t.Errorf("ContainerCreationTimeNeedsFix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStripCreationTimeTags(t *testing.T) {
	t.Parallel()

	input := []byte(`<?xml version="1.0"?>
<Tags>
  <Tag>
    <Targets />
    <Simple>
      <Name>TITLE</Name>
      <String>My Movie</String>
    </Simple>
    <Simple>
      <Name>ENCODED_DATE</Name>
      <String>2024-01-01 00:00:00</String>
    </Simple>
  </Tag>
  <Tag>
    <Targets>
      <TrackUID>111</TrackUID>
    </Targets>
    <Simple>
      <Name>_STATISTICS_WRITING_DATE_UTC</Name>
      <String>2024-01-01 00:00:00</String>
    </Simple>
  </Tag>
</Tags>`)

	got, removed := StripCreationTimeTags(input)

	wantRemoved := []string{"ENCODED_DATE", "_STATISTICS_WRITING_DATE_UTC"}
	if !slices.Equal(removed, wantRemoved) {
		t.Fatalf("removed = %v, want %v", removed, wantRemoved)
	}

	if strings.Contains(string(got), "ENCODED_DATE") || strings.Contains(string(got), "_STATISTICS_WRITING_DATE_UTC") {
		t.Errorf("expected date tags to be stripped, got:\n%s", got)
	}

	if !strings.Contains(string(got), "TITLE") {
		t.Errorf("expected unrelated Simple tag (TITLE) to survive, got:\n%s", got)
	}

	// The second Tag's only Simple entry was the stripped date tag, so the
	// now-empty Tag block (Targets with no Simple children) must be removed
	// too rather than left behind as dead structure.
	if strings.Contains(string(got), "TrackUID") {
		t.Errorf("expected the now-empty per-track Tag block to be removed, got:\n%s", got)
	}
}

func TestStripCreationTimeTagsNoMatch(t *testing.T) {
	t.Parallel()

	input := []byte(`<?xml version="1.0"?>
<Tags>
  <Tag>
    <Targets />
    <Simple>
      <Name>TITLE</Name>
      <String>My Movie</String>
    </Simple>
  </Tag>
</Tags>`)

	got, removed := StripCreationTimeTags(input)

	if removed != nil {
		t.Errorf("expected no removed tags, got %v", removed)
	}

	if string(got) != string(input) {
		t.Errorf("expected content unchanged when nothing matches, got:\n%s", got)
	}
}
