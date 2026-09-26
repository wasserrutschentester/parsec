package correct

import (
	"testing"

	"github.com/spf13/viper"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/metadata/mediainfo"
	"codeberg.org/upPollo/parsec/internal/metadata/resolve"
)

//nolint:paralleltest,funlen // mutates global state
func TestPlanFile(t *testing.T) {
	config.InitDefaults()
	// Force the check to be enabled so we guarantee a title edit is proposed
	viper.Set("disabled_checks", []string{})

	// Mock resolveMetadata
	originalResolve := resolveMetadata

	resolveMetadata = func(_ resolve.Options) (*resolve.Result, error) {
		return &resolve.Result{
			Meta: &metadata.Metadata{
				Title: "Test Title",
			},
			MediaInfo: &mediainfo.MediaInfo{},
		}, nil
	}
	defer func() { resolveMetadata = originalResolve }()

	// Mock getEbmlMetadata
	originalGetEbml := getEbmlMetadata

	getEbmlMetadata = func(_ string) (*matroska.EbmlMetadata, error) {
		return &matroska.EbmlMetadata{
			Container: matroska.EbmlContainer{
				Properties: matroska.EbmlContainerProperties{
					Title: "Ugly Old Title (WEB-DL) [1080p]",
				},
			},
			Tracks: []matroska.EbmlTrack{
				{
					ID:   1,
					Type: "video",
					Properties: matroska.EbmlTrackProperties{
						Name:    "Junk HD Video", // Should trigger NameEdits
						Default: false,
					},
				},
				{
					ID:   2,
					Type: "audio",
					Properties: matroska.EbmlTrackProperties{
						Name:    "Surround 5.1",
						Default: true,
					},
				},
			},
		}, nil
	}
	defer func() { getEbmlMetadata = originalGetEbml }()

	// Mock extractTagsXML
	originalExtractTags := extractTagsXML

	extractTagsXML = func(_ string) ([]byte, error) {
		return []byte("<Tags></Tags>"), nil
	}
	defer func() { extractTagsXML = originalExtractTags }()

	opts := Options{}

	plan, err := PlanFile("dummy.mkv", opts)
	if err != nil {
		t.Fatalf("PlanFile failed: %v", err)
	}

	if plan == nil {
		t.Fatalf("expected non-nil FixPlan")
	}

	// Verify that the title fix got populated
	var foundTitle bool

	for _, prop := range plan.Metadata.Container.Properties {
		if prop.Key == "title" {
			foundTitle = true

			if prop.NewValue != "Test Title" {
				t.Errorf("expected container title to be set to 'Test Title', got '%s'", prop.NewValue)
			}
		}
	}

	if !foundTitle {
		t.Errorf("expected a container property edit for 'title'")
	}
}

func TestFixPlanIsNotEmptyWhenChapterLanguageNeedsPrompt(t *testing.T) {
	t.Parallel()

	plan := NewFixPlan()

	plan.Metadata.Chapters.LanguageFix.NeedsPrompt = true
	if plan.IsEmpty() {
		t.Fatal("chapter language input requirement must not be reported as a clean file")
	}
}

//nolint:paralleltest // mutates global state
func TestPlanFileStopAfterVideoEnds(t *testing.T) {
	config.InitDefaults()
	viper.Set("disabled_checks", []string{})

	dur100 := 100.0
	dur110 := 110.0

	originalResolve := resolveMetadata
	defer func() { resolveMetadata = originalResolve }()

	resolveMetadata = func(_ resolve.Options) (*resolve.Result, error) {
		return &resolve.Result{
			Meta: &metadata.Metadata{},
			MediaInfo: &mediainfo.MediaInfo{
				Media: mediainfo.Media{
					Tracks: []mediainfo.Track{
						{Type: "Video", Duration: &dur100},
						{Type: "Audio", Duration: &dur110},
					},
				},
			},
		}, nil
	}

	originalGetEbml := getEbmlMetadata
	defer func() { getEbmlMetadata = originalGetEbml }()

	getEbmlMetadata = func(_ string) (*matroska.EbmlMetadata, error) {
		return &matroska.EbmlMetadata{
			Tracks: []matroska.EbmlTrack{
				{ID: 0, Type: "video"},
				{ID: 1, Type: "audio"},
			},
		}, nil
	}

	originalExtractTags := extractTagsXML
	defer func() { extractTagsXML = originalExtractTags }()

	extractTagsXML = func(_ string) ([]byte, error) {
		return []byte("<Tags></Tags>"), nil
	}

	plan, err := PlanFile("dummy.mkv", Options{})
	if err != nil {
		t.Fatalf("PlanFile failed: %v", err)
	}

	if len(plan.Remux.TruncatedTracks) != 1 {
		t.Fatalf("expected 1 truncated track, got %d", len(plan.Remux.TruncatedTracks))
	}

	if plan.Remux.TruncatedTracks[0].TrackID != 1 {
		t.Errorf("expected TruncatedTracks[0].TrackID 1, got %d", plan.Remux.TruncatedTracks[0].TrackID)
	}

	if !plan.Remux.Required {
		t.Errorf("expected plan.Remux.Required to be true")
	}
}
