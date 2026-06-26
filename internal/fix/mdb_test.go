package fix

import (
	"slices"
	"testing"

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

	candidates := []RemovalCandidate{
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
