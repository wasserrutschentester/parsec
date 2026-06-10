package matroska

import (
	"encoding/xml"
	"os"
	"testing"

	"codeberg.org/upPollo/parsec/internal/mdb"
)

func TestCreateTagsXML(t *testing.T) {
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
