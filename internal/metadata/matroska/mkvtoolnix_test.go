package matroska

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"strings"
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

func TestFormatChapterTimestamp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ns   int64
		want string
	}{
		{0, "00:00:00.000000000"},
		{20_000_000_000, "00:00:20.000000000"},
		{3_661_500_000_000, "01:01:01.500000000"},
		{-1, "00:00:00.000000000"},
	}

	for _, tt := range tests {
		if got := formatChapterTimestamp(tt.ns); got != tt.want {
			t.Errorf("formatChapterTimestamp(%d) = %q, want %q", tt.ns, got, tt.want)
		}
	}
}

func TestReplaceChapterTimestamps(t *testing.T) {
	t.Parallel()

	xmlContent := []byte(`<?xml version="1.0"?>
<Chapters>
  <EditionEntry>
    <EditionUID>1</EditionUID>
    <ChapterAtom>
      <ChapterUID>10</ChapterUID>
      <ChapterTimeStart>00:00:00.000000000</ChapterTimeStart>
      <ChapterFlagHidden>0</ChapterFlagHidden>
      <ChapterDisplay>
        <ChapterString>Intro</ChapterString>
        <ChapterLanguage>eng</ChapterLanguage>
      </ChapterDisplay>
    </ChapterAtom>
    <ChapterAtom>
      <ChapterUID>11</ChapterUID>
      <ChapterTimeStart>00:00:16.000000000</ChapterTimeStart>
      <ChapterDisplay>
        <ChapterString>Scene 2</ChapterString>
        <ChapterLanguage>eng</ChapterLanguage>
      </ChapterDisplay>
    </ChapterAtom>
  </EditionEntry>
</Chapters>
`)

	got, err := replaceChapterTimestamps(xmlContent, []int64{0, 20_000_000_000})
	if err != nil {
		t.Fatalf("replaceChapterTimestamps() error = %v", err)
	}

	gotStr := string(got)

	if !strings.Contains(gotStr, "<ChapterTimeStart>00:00:20.000000000</ChapterTimeStart>") {
		t.Errorf("expected second chapter snapped to 20s, got:\n%s", gotStr)
	}

	if !strings.Contains(gotStr, "<ChapterUID>10</ChapterUID>") || !strings.Contains(gotStr, "<ChapterFlagHidden>0</ChapterFlagHidden>") {
		t.Errorf("expected unrelated XML content to be preserved untouched, got:\n%s", gotStr)
	}

	if !strings.Contains(gotStr, "<ChapterString>Scene 2</ChapterString>") {
		t.Errorf("expected display names to be preserved, got:\n%s", gotStr)
	}
}

func TestReplaceChapterTimestampsCountMismatch(t *testing.T) {
	t.Parallel()

	xmlContent := []byte("<ChapterTimeStart>00:00:00.000000000</ChapterTimeStart>")

	if _, err := replaceChapterTimestamps(xmlContent, []int64{0, 1}); err == nil {
		t.Error("expected an error when newTimes doesn't match the number of ChapterTimeStart elements")
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

func TestBuildRemuxArgsDoesNotStripCompressionWhenNotRequested(t *testing.T) {
	t.Parallel()

	tracks := []EbmlTrack{
		{ID: 0, Type: "video"},
		{ID: 1, Type: "audio"},
	}

	opts := RemuxOptions{TrackOrder: []int{0, 1}}

	got := buildRemuxArgs("out.mkv", "in.mkv", opts, tracks)

	for _, arg := range got {
		if strings.HasPrefix(arg, "--compression") || strings.HasSuffix(arg, ":none") {
			t.Fatalf("unexpected compression argument without confirmation: %v", got)
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
