package correct

import (
	"testing"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
)

// First of two German audio tracks lacks the default flag -> should be set.

// Specialized forced subtitle wrongly marked default -> should be cleared.

// Track 1 needs both a flag fix (first of two German tracks, missing
// default) and a name fix (redundant codec word), to verify the two
// computations stay disjoint.

//nolint:funlen,paralleltest // comprehensive test
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
