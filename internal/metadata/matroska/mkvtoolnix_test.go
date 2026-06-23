package matroska

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"testing"

	"codeberg.org/upPollo/parsec/internal/mdb"
)

func TestCreateTagsXML(t *testing.T) {
	t.Parallel()

	tags := mdb.MatroskaTags{
		Title: "Test Movie",
		Imdb:  "tt1234567",
		Tmdb:  "movie/123",
		Tvdb:  456,
		Tvdb2: "series/456",
	}

	xmlPath, err := createTagsXML(tags)
	if err != nil {
		t.Fatalf("createTagsXML failed: %v", err)
	}
	defer func() { _ = os.Remove(xmlPath) }()

	content, err := os.ReadFile(xmlPath)
	if err != nil {
		t.Fatalf("failed to read generated XML: %v", err)
	}

	var parsedTags mkvTags

	err = xml.Unmarshal(content, &parsedTags)
	if err != nil {
		t.Fatalf("failed to unmarshal generated XML: %v", err)
	}

	if len(parsedTags.Tags) != 1 {
		t.Fatalf("expected 1 Tag element, got %d", len(parsedTags.Tags))
	}

	tag := parsedTags.Tags[0]
	if tag.Targets.TargetTypeValue != 50 {
		t.Errorf("expected TargetTypeValue 50, got %d", tag.Targets.TargetTypeValue)
	}

	expectedSimples := map[string]string{
		"TITLE": "Test Movie",
		"IMDB":  "tt1234567",
		"TMDB":  "movie/123",
		"TVDB":  "456",
		"TVDB2": "series/456",
	}

	foundSimples := make(map[string]int)
	for _, s := range tag.Simple {
		foundSimples[s.Name]++
		if val, ok := expectedSimples[s.Name]; ok {
			if s.String != val {
				t.Errorf("expected %s to be %s, got %s", s.Name, val, s.String)
			}
		}
	}
}

func TestCountTypes(t *testing.T) {
	t.Parallel()

	metadata := &EbmlMetadata{
		Tracks: []EbmlTrack{
			{Type: "video"},
			{Type: "audio"},
			{Type: "audio"},
			{Type: "subtitles"},
			{Type: "subtitle"},
		},
	}

	metadata.countTypes()

	expected := []int{1, 1, 2, 1, 2}
	for i, track := range metadata.Tracks {
		if track.TypeOrder != expected[i] {
			t.Errorf("track %d (type %s) expected TypeNumber %d, got %d", i, track.Type, expected[i], track.TypeOrder)
		}
	}
}

func TestParseDimensions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		wantW int
		wantH int
	}{
		{"1920x1080", 1920, 1080},
		{"1280x720", 1280, 720},
		{"", 0, 0},
		{"invalid", 0, 0},
		{"1920", 0, 0},
		{"1920x", 0, 0},
		{"x1080", 0, 0},
		{"1920x1080x10", 0, 0},
	}

	for _, tt := range tests {
		w, h := ParseDimensions(tt.input)
		if w != tt.wantW || h != tt.wantH {
			t.Errorf("ParseDimensions(%q) = (%d, %d), want (%d, %d)", tt.input, w, h, tt.wantW, tt.wantH)
		}
	}
}

func TestUnmarshalRealJSON(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("../../../scratch_mkvmerge.json")
	if err != nil {
		t.Skip("skipping test; scratch_mkvmerge.json not found")
	}

	var metadata EbmlMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatalf("failed to unmarshal scratch_mkvmerge.json: %v", err)
	}

	// Verify that we successfully unmarshaled tracks
	if len(metadata.Tracks) == 0 {
		t.Fatalf("expected at least one track, got 0")
	}

	// The first track should be video (ID: 0)
	videoTrack := metadata.Tracks[0]
	if videoTrack.Type != "video" {
		t.Errorf("expected track 0 type to be 'video', got %q", videoTrack.Type)
	}

	// Verify pixel_dimensions parsing via helper
	w, h := ParseDimensions(videoTrack.Properties.PixelDimensions)
	if w != 1920 || h != 804 {
		t.Errorf("expected parsed pixel dimensions 1920x804, got %dx%d", w, h)
	}
}

func TestSplitCRLF(t *testing.T) {
	t.Parallel()

	input := []byte("Extracting...\nProgress: 10%\rProgress: 50%\rProgress: 100%\n")
	expected := []string{
		"Extracting...",
		"Progress: 10%",
		"Progress: 50%",
		"Progress: 100%",
	}

	var results []string

	data := input
	for len(data) > 0 {
		advance, token, err := splitCRLF(data, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if advance == 0 {
			break
		}

		results = append(results, string(token))
		data = data[advance:]
	}

	if len(results) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(results), results)
	}

	for i, got := range results {
		if got != expected[i] {
			t.Errorf("token %d: expected %q, got %q", i, expected[i], got)
		}
	}
}
