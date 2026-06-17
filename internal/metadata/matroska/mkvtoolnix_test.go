package matroska

import (
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

func TestBuildPropeditArgs(t *testing.T) {
	t.Parallel()

	edits := []TrackEdit{
		{Number: 2, Props: map[string]string{"name": "German", "flag-default": "1"}},
		{Number: 3, Props: map[string]string{"name": ""}},
	}

	got := buildPropeditArgs("movie.mkv", edits)

	want := []string{
		"movie.mkv",
		"--edit", "track:@2", "--set", "flag-default=1", "--set", "name=German",
		"--edit", "track:@3", "--delete", "name",
	}

	if len(got) != len(want) {
		t.Fatalf("expected %d args, got %d: %v", len(want), len(got), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("arg %d: expected %q, got %q", i, want[i], got[i])
		}
	}
}

func TestBuildRemuxArgs(t *testing.T) {
	t.Parallel()

	tracks := []EbmlTrack{
		{ID: 0, Type: "video"},
		{ID: 1, Type: "audio"},
		{ID: 2, Type: "audio"},
	}

	opts := RemuxOptions{
		TrackOrder:              []int{0, 2, 1},
		RemoveTrackIDs:          []int{1},
		DisableTrackCompression: true,
	}

	got := buildRemuxArgs("out.mkv", "in.mkv", opts, tracks)

	want := []string{
		"-o", "out.mkv",
		"--audio-tracks", "!1",
		"--compression", "0:none",
		"--compression", "2:none",
		"--track-order", "0:0,0:2",
		"in.mkv",
	}

	if len(got) != len(want) {
		t.Fatalf("expected %d args, got %d: %v", len(want), len(got), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("arg %d: expected %q, got %q", i, want[i], got[i])
		}
	}
}
