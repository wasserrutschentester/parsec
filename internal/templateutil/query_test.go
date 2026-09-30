package templateutil

import (
	"reflect"
	"testing"
	"time"
)

type sampleTrack struct {
	Type     string
	Codec    string
	Bitrate  int
	Language string
	Props    sampleProps
}

type sampleProps struct {
	Original bool
}

func TestPluck(t *testing.T) {
	t.Parallel()

	tracks := []sampleTrack{
		{Type: "video", Codec: "AVC"},
		{Type: "audio", Codec: "DDP"},
		{Type: "audio", Codec: "AAC"},
	}

	codecs := Pluck("Codec", tracks)

	expected := []any{"AVC", "DDP", "AAC"}
	if !reflect.DeepEqual(codecs, expected) {
		t.Errorf("Pluck(Codec) = %v, want %v", codecs, expected)
	}

	types := Pluck("Type", &tracks)

	expectedTypes := []any{"video", "audio", "audio"}
	if !reflect.DeepEqual(types, expectedTypes) {
		t.Errorf("Pluck(Type) = %v, want %v", types, expectedTypes)
	}

	if got := Pluck("NonExistent", tracks); len(got) != 0 {
		t.Errorf("Pluck(NonExistent) = %v, want empty", got)
	}

	if got := Pluck("Codec", "not-a-slice"); got != nil {
		t.Errorf("Pluck on non-slice = %v, want nil", got)
	}
}

func TestUniq(t *testing.T) {
	t.Parallel()

	input := []any{"a", "b", "a", "c", "b"}
	got := Uniq(input)

	want := []any{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Uniq() = %v, want %v", got, want)
	}

	if gotNil := Uniq(nil); gotNil != nil {
		t.Errorf("Uniq(nil) = %v, want nil", gotNil)
	}
}

func TestFirstAndLast(t *testing.T) {
	t.Parallel()

	items := []int{10, 20, 30, 40, 50}

	first2, err := First(2, items)
	if err != nil || !reflect.DeepEqual(first2, []any{10, 20}) {
		t.Errorf("First(2) = %v (err: %v), want [10 20]", first2, err)
	}

	last2, err := Last(2, items)
	if err != nil || !reflect.DeepEqual(last2, []any{40, 50}) {
		t.Errorf("Last(2) = %v (err: %v), want [40 50]", last2, err)
	}

	firstAll, _ := First(10, items)
	if !reflect.DeepEqual(firstAll, []any{10, 20, 30, 40, 50}) {
		t.Errorf("First(10) = %v, want all items", firstAll)
	}

	if _, err := First(-1, items); err == nil {
		t.Errorf("First(-1) expected error, got nil")
	}

	if _, err := Last(-1, items); err == nil {
		t.Errorf("Last(-1) expected error, got nil")
	}
}

func TestIndexOrEmpty(t *testing.T) {
	t.Parallel()

	items := []string{"zero", "one", "two"}
	if got := IndexOrEmpty(1, items); got != "one" {
		t.Errorf("IndexOrEmpty(1) = %v, want 'one'", got)
	}

	if got := IndexOrEmpty(5, items); got != "" {
		t.Errorf("IndexOrEmpty(5) = %v, want ''", got)
	}

	if got := IndexOrEmpty(-1, items); got != "" {
		t.Errorf("IndexOrEmpty(-1) = %v, want ''", got)
	}

	if got := IndexOrEmpty(0, nil); got != "" {
		t.Errorf("IndexOrEmpty on nil = %v, want ''", got)
	}
}

func TestDefault(t *testing.T) {
	t.Parallel()

	if got := Default("fallback", ""); got != "fallback" {
		t.Errorf("Default(fallback, '') = %v, want 'fallback'", got)
	}

	if got := Default("fallback", 0); got != "fallback" {
		t.Errorf("Default(fallback, 0) = %v, want 'fallback'", got)
	}

	if got := Default("fallback", nil); got != "fallback" {
		t.Errorf("Default(fallback, nil) = %v, want 'fallback'", got)
	}

	if got := Default("fallback", "value"); got != "value" {
		t.Errorf("Default(fallback, 'value') = %v, want 'value'", got)
	}

	if got := Default(42, 100); got != 100 {
		t.Errorf("Default(42, 100) = %v, want 100", got)
	}
}

func TestParseDate(t *testing.T) {
	t.Parallel()

	// 1 arg -> time.Time
	resTime := ParseDate("2024-05-18")
	if tm, ok := resTime.(time.Time); !ok || tm.Year() != 2024 || tm.Month() != 5 || tm.Day() != 18 {
		t.Errorf("ParseDate('2024-05-18') = %v, want 2024-05-18 time.Time", resTime)
	}

	// 2 args -> formatted string
	resStr := ParseDate("2006.01.02", "2024-05-18")
	if resStr != "2024.05.18" {
		t.Errorf("ParseDate(2006.01.02, 2024-05-18) = %v, want '2024.05.18'", resStr)
	}

	// Invalid / empty date
	if got := ParseDate("2006.01.02", ""); got != "" {
		t.Errorf("ParseDate with empty date = %v, want ''", got)
	}
}

//nolint:cyclop,funlen // tests multiple query operator branches
func TestWhere(t *testing.T) {
	t.Parallel()

	tracks := []sampleTrack{
		{Type: "video", Codec: "HEVC", Bitrate: 15000, Language: "en", Props: sampleProps{Original: true}},
		{Type: "audio", Codec: "DDP", Bitrate: 640, Language: "de", Props: sampleProps{Original: false}},
		{Type: "audio", Codec: "AAC", Bitrate: 192, Language: "en", Props: sampleProps{Original: true}},
		{Type: "sub", Codec: "SubRip", Bitrate: 0, Language: "de", Props: sampleProps{Original: false}},
	}

	t.Run("Equality", func(t *testing.T) {
		t.Parallel()

		audioTracks, err := Where("Type", "audio", tracks)
		if err != nil {
			t.Fatalf("Where(Type, audio) err: %v", err)
		}

		if res, ok := audioTracks.([]any); !ok || len(res) != 2 {
			t.Fatalf("Where(Type, audio) len = %d, want 2", len(res))
		}
	})

	t.Run("GreaterThan", func(t *testing.T) {
		t.Parallel()

		highBitrate, err := Where("Bitrate", ">", 500, tracks)
		if err != nil {
			t.Fatalf("Where(Bitrate > 500) err: %v", err)
		}

		if res, ok := highBitrate.([]any); !ok || len(res) != 2 {
			t.Fatalf("Where(Bitrate > 500) len = %d, want 2", len(res))
		}
	})

	t.Run("NestedPath", func(t *testing.T) {
		t.Parallel()

		origTracks, err := Where("Props.Original", true, tracks)
		if err != nil {
			t.Fatalf("Where(Props.Original, true) err: %v", err)
		}

		if res, ok := origTracks.([]any); !ok || len(res) != 2 {
			t.Fatalf("Where(Props.Original, true) len = %d, want 2", len(res))
		}
	})

	t.Run("InOperator", func(t *testing.T) {
		t.Parallel()

		enTracks, err := Where("Language", "in", []string{"en"}, tracks)
		if err != nil {
			t.Fatalf("Where(Language in ['en']) err: %v", err)
		}

		if res, ok := enTracks.([]any); !ok || len(res) != 2 {
			t.Fatalf("Where(Language in ['en']) len = %d, want 2", len(res))
		}
	})

	t.Run("ContainsOperator", func(t *testing.T) {
		t.Parallel()

		resTracks, err := Where("Codec", "contains", "D", tracks)
		if err != nil {
			t.Fatalf("Where(Codec contains 'D') err: %v", err)
		}

		if res, ok := resTracks.([]any); !ok || len(res) != 1 {
			t.Fatalf("Where(Codec contains 'D') len = %d, want 1", len(res))
		}
	})

	t.Run("NotContainsOperator", func(t *testing.T) {
		t.Parallel()

		resTracks, err := Where("Codec", "not contains", "D", tracks)
		if err != nil {
			t.Fatalf("Where(Codec not contains 'D') err: %v", err)
		}

		if res, ok := resTracks.([]any); !ok || len(res) != 3 {
			t.Fatalf("Where(Codec not contains 'D') len = %d, want 3", len(res))
		}
	})
}
