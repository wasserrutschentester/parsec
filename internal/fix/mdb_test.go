package fix

import (
	"slices"
	"testing"

	"codeberg.org/upPollo/parsec/internal/checks"
	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
)

//nolint:paralleltest // depends on shared global config state
func TestNeedsOriginalLanguageForUnwantedAudio(t *testing.T) {
	config.InitDefaults()

	knownSafe := []matroska.EbmlTrack{
		{Type: "video"},
		{Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger"}},
		{Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "und"}},
		{Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "mul"}},
	}
	if needsOriginalLanguageForUnwantedAudio(knownSafe) {
		t.Fatal("expected no MDB lookup when all audio languages are known safe without original language")
	}

	withPotentialBloat := slices.Concat(knownSafe, []matroska.EbmlTrack{{
		Type:       "audio",
		Properties: matroska.EbmlTrackProperties{Language: "eng"},
	}})
	if !needsOriginalLanguageForUnwantedAudio(withPotentialBloat) {
		t.Fatal("expected MDB lookup for non-preferred audio language")
	}
}

func TestRemovalLanguages(t *testing.T) {
	t.Parallel()

	candidates := []checks.RemovalCandidate{
		{Track: matroska.EbmlTrack{Properties: matroska.EbmlTrackProperties{Language: "spa"}}},
		{Track: matroska.EbmlTrack{Properties: matroska.EbmlTrackProperties{Language: "fre"}}},
		{Track: matroska.EbmlTrack{Properties: matroska.EbmlTrackProperties{Language: "spa"}}},
		{Track: matroska.EbmlTrack{}},
	}

	want := []string{"fre", "spa", "und"}
	if got := removalLanguages(candidates); !slices.Equal(got, want) {
		t.Fatalf("removalLanguages() = %v, want %v", got, want)
	}
}

func TestNormalizeOriginalLanguageCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "empty", value: "", want: ""},
		{name: "two letter", value: "de", want: "de"},
		{name: "three letter legacy", value: "ger", want: "de"},
		{name: "uppercase three letter", value: "JPN", want: "ja"},
		{name: "too short", value: "d", wantErr: true},
		{name: "too long", value: "de-DE", wantErr: true},
		{name: "non letter", value: "d3", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeOriginalLanguageCode(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("normalizeOriginalLanguageCode(%q) expected error", tt.value)
				}

				return
			}

			if err != nil {
				t.Fatalf("normalizeOriginalLanguageCode(%q) unexpected error: %v", tt.value, err)
			}

			if got != tt.want {
				t.Fatalf("normalizeOriginalLanguageCode(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}
