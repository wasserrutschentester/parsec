package correct

import (
	"reflect"
	"testing"

	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
)

//nolint:gocognit,paralleltest,cyclop,funlen // mutates package variables
func TestExecutePlan(t *testing.T) {
	// Table driven tests for each execution step
	t.Run("executeContainerProperties", func(t *testing.T) {
		var (
			capturedPath  string
			capturedProps map[string]string
		)

		execSetContainerProperties = func(filePath string, props map[string]string) error {
			capturedPath = filePath
			capturedProps = props

			return nil
		}

		plan := &FixPlan{
			Metadata: MetadataPlan{
				Container: ContainerPlan{
					Properties: []ContainerPropertyEdit{
						{Key: "title", NewValue: "Test Title"},
					},
				},
			},
		}

		err := executeContainerProperties("test.mkv", plan)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if capturedPath != "test.mkv" {
			t.Errorf("expected path test.mkv, got %v", capturedPath)
		}

		expectedProps := map[string]string{"title": "Test Title"}
		if !reflect.DeepEqual(capturedProps, expectedProps) {
			t.Errorf("expected props %v, got %v", expectedProps, capturedProps)
		}
	})

	t.Run("executeAttachments", func(t *testing.T) {
		var (
			capturedRenamePath string
			capturedRenames    map[int]string
		)

		execRenameAttachments = func(filePath string, renames map[int]string) error {
			capturedRenamePath = filePath
			capturedRenames = renames

			return nil
		}

		var (
			capturedAddPath string
			capturedAdds    []matroska.AttachmentAdd
		)

		execAddAttachments = func(filePath string, adds []matroska.AttachmentAdd) error {
			capturedAddPath = filePath
			capturedAdds = adds

			return nil
		}

		var (
			capturedDelPath string
			capturedRemoves []int
		)

		execDeleteAttachments = func(filePath string, removes []int) error {
			capturedDelPath = filePath
			capturedRemoves = removes

			return nil
		}

		plan := &FixPlan{
			Metadata: MetadataPlan{
				Attachments: AttachmentPlan{
					Renames: []AttachmentRename{{ID: 1, NewName: "font.ttf"}},
					ToAdd: []MissingFontAttachment{{
						Path:           "/tmp/font.ttf",
						AttachmentName: "font2.ttf",
						MIMEType:       "application/x-truetype-font",
					}},
					ToRemove: []AttachmentRemove{{ID: 2}},
				},
			},
		}

		err := executeAttachments("test.mkv", plan)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if capturedRenamePath != "test.mkv" || capturedAddPath != "test.mkv" || capturedDelPath != "test.mkv" {
			t.Errorf("expected paths to be test.mkv")
		}

		expectedRenames := map[int]string{1: "font.ttf"}
		if !reflect.DeepEqual(capturedRenames, expectedRenames) {
			t.Errorf("expected renames %v, got %v", expectedRenames, capturedRenames)
		}

		expectedAdds := []matroska.AttachmentAdd{{Path: "/tmp/font.ttf", Name: "font2.ttf", MIMEType: "application/x-truetype-font"}}
		if !reflect.DeepEqual(capturedAdds, expectedAdds) {
			t.Errorf("expected adds %v, got %v", expectedAdds, capturedAdds)
		}

		expectedRemoves := []int{2}
		if !reflect.DeepEqual(capturedRemoves, expectedRemoves) {
			t.Errorf("expected removes %v, got %v", expectedRemoves, capturedRemoves)
		}
	})

	t.Run("executeChapters", func(t *testing.T) {
		var (
			capturedPath  string
			capturedTimes []int64
		)

		execRewriteChapterTimes = func(filePath string, times []int64) error {
			capturedPath = filePath
			capturedTimes = times

			return nil
		}

		plan := &FixPlan{
			Metadata: MetadataPlan{
				Chapters: ChapterPlan{
					KeyframeSnaps: ChapterAlignmentFix{
						Changed: 1,
						Times:   []int64{100, 200},
					},
				},
			},
		}

		err := executeChapters("test.mkv", plan)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if capturedPath != "test.mkv" {
			t.Errorf("expected path test.mkv, got %v", capturedPath)
		}

		expectedTimes := []int64{100, 200}
		if !reflect.DeepEqual(capturedTimes, expectedTimes) {
			t.Errorf("expected times %v, got %v", expectedTimes, capturedTimes)
		}
	})

	t.Run("executeTracksAndTags", func(t *testing.T) {
		var addStatPath string

		execAddTrackStatistics = func(filePath string) error {
			addStatPath = filePath

			return nil
		}

		var extractTagsPath string

		execExtractTagsXML = func(filePath string) ([]byte, error) {
			extractTagsPath = filePath

			return []byte("<Tags></Tags>"), nil // mock XML
		}

		var setTagsPath string

		execSetTagsXML = func(filePath string, _ []byte) error {
			setTagsPath = filePath

			return nil
		}

		var (
			setTracksPath  string
			setTracksEdits []matroska.TrackEdit
		)

		execSetTrackProperties = func(filePath string, edits []matroska.TrackEdit) error {
			setTracksPath = filePath
			setTracksEdits = edits

			return nil
		}

		plan := &FixPlan{
			Metadata: MetadataPlan{
				Container: ContainerPlan{
					WriteStatistics:   true,
					ClearCreationTime: true,
				},
				Tracks: []matroska.TrackEdit{{Number: 1}, {Number: 2}, {Number: 3}},
			},
		}

		err := executeTracksAndTags("test.mkv", plan)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if addStatPath != "test.mkv" {
			t.Errorf("expected addStatPath test.mkv, got %v", addStatPath)
		}

		if extractTagsPath != "test.mkv" {
			t.Errorf("expected extractTagsPath test.mkv, got %v", extractTagsPath)
		}

		// Since our mock XML doesn't contain creation time tags, StripCreationTimeTags will return removed=nil
		// Thus SetTagsXML shouldn't be called.
		if setTagsPath != "" {
			t.Errorf("expected setTagsPath empty, got %v", setTagsPath)
		}

		if setTracksPath != "test.mkv" {
			t.Errorf("expected setTracksPath test.mkv, got %v", setTracksPath)
		}

		expectedEdits := []matroska.TrackEdit{{Number: 1}, {Number: 2}, {Number: 3}}
		if !reflect.DeepEqual(setTracksEdits, expectedEdits) {
			t.Errorf("expected track edits %v, got %v", expectedEdits, setTracksEdits)
		}
	})

	t.Run("executeRemux", func(t *testing.T) {
		var (
			capturedPath string
			capturedOpts matroska.RemuxOptions
		)

		execRemuxTracks = func(filePath string, opts matroska.RemuxOptions) error {
			capturedPath = filePath
			capturedOpts = opts

			return nil
		}

		plan := &FixPlan{
			Remux: RemuxPlan{
				Required:         true,
				TrackOrder:       []int{2, 1, 3},
				StripCompression: []int{2},
				RemoveTracks:     []RemovalCandidate{{TrackID: 3}},
			},
		}

		err := executeRemux("test.mkv", plan)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if capturedPath != "test.mkv" {
			t.Errorf("expected path test.mkv, got %v", capturedPath)
		}

		expectedOpts := matroska.RemuxOptions{
			TrackOrder:          []int{2, 1, 3},
			StripCompressionIDs: []int{2},
			RemoveTrackIDs:      []int{3},
		}

		if !reflect.DeepEqual(capturedOpts, expectedOpts) {
			t.Errorf("expected opts %v, got %v", expectedOpts, capturedOpts)
		}
	})

	t.Run("ExecutePlan", func(t *testing.T) {
		// Just tests that no error is returned on empty plan
		err := ExecutePlan("test.mkv", &FixPlan{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
