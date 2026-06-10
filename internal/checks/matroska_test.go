package checks

import (
	"strings"
	"testing"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
)

//nolint:funlen
func TestRunTrackChecks(t *testing.T) {
	config.InitDefaults()

	tests := []struct {
		name    string
		tracks  []matroska.EbmlTrack
		wantErr bool
	}{
		{
			name: "Valid German and English tracks",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 1}},
				{ID: 2, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "eng", Default: true, Number: 2}},
				{ID: 3, Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", Forced: true, Name: "Forced", Number: 3}},
				{ID: 4, Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 4}},
			},
			wantErr: false,
		},
		{
			name: "Out of order language (eng before ger)",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "eng", Default: true, Number: 1}},
				{ID: 2, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 2}},
			},
			wantErr: true,
		},
		{
			name: "Out of order subtitle properties (Default before Forced)",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 1}},
				{ID: 2, Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", Forced: true, Name: "Forced", Number: 2}},
			},
			wantErr: true,
		},
		{
			name: "Missing Name for mul track",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "mul", Name: "", Default: true, Number: 1}},
			},
			wantErr: true,
		},
		{
			name: "Duplicate track detected",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 1}},
				{ID: 2, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 2}},
			},
			wantErr: true,
		},
		{
			name: "SDH missing keyword",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 1}},
				{ID: 2, Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", HearingImpaired: true, Number: 2}},
			},
			wantErr: true,
		},
		{
			name: "SDH with keyword",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 1}},
				{ID: 2, Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", HearingImpaired: true, Name: "SDH", Number: 2}},
			},
			wantErr: false,
		},
		{
			name: "SDH keyword without flag",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 1}},
				{ID: 2, Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", Name: "SDH", Number: 2}},
			},
			wantErr: true,
		},
		{
			name: "OriginalLanguage inconsistency",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "fre", OriginalLanguage: true, Default: true, Number: 1}},
				{ID: 2, Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "fre", Default: true, Number: 2}},
			},
			wantErr: true,
		},
		{
			name: "Commentary missing keyword",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Commentary: true, Default: true, Number: 1}},
			},
			wantErr: true,
		},
		{
			name: "Commentary with keyword",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 1}},
				{ID: 2, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Commentary: true, Name: "Commentary", Number: 2}},
			},
			wantErr: false,
		},
		{
			name: "Missing default flags for first non-special subs track",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 1}},
				{ID: 2, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "eng", Default: true, Number: 2}},
				{ID: 3, Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", Forced: true, Name: "Forced", Number: 3}},
				{ID: 4, Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: false, Number: 4}},
			},
			wantErr: true,
		},
		{
			name: "Missing default flag for first audio",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: false, Number: 1}},
				{ID: 2, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Commentary: true, Name: "Commentary", Number: 2}},
			},
			wantErr: true,
		},
		{
			name: "Relaxation: single track without default flag",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: false, Number: 1}},
			},
			wantErr: false,
		},
		{
			name: "Specialized track with Default flag (invalid)",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 1}},
				{ID: 2, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", VisualImpaired: true, Default: true, Name: "AD", Number: 2}},
			},
			wantErr: true,
		},
		{
			name: "Multiple default flags for same language",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 1}},
				{ID: 2, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 2}},
			},
			wantErr: true,
		},
		{
			name: "All SRT subtitles",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "subtitles", Codec: "S_TEXT/SRT", Properties: matroska.EbmlTrackProperties{Language: "ger", Number: 1}},
				{ID: 2, Type: "subtitles", Codec: "SRT", Properties: matroska.EbmlTrackProperties{Language: "eng", Number: 2}},
			},
			wantErr: false,
		},
		{
			name: "Contains non-SRT subtitle",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "subtitles", Codec: "S_TEXT/SRT", Properties: matroska.EbmlTrackProperties{TextSubtitles: true, Language: "ger", Number: 1}},
				{ID: 2, Type: "subtitles", Codec: "S_TEXT/ASS", Properties: matroska.EbmlTrackProperties{TextSubtitles: true, Language: "eng", Number: 2}},
			},
			wantErr: true,
		},
		{
			name: "Redundant language name 'German'",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Name: "German", Default: true, Number: 1}},
			},
			wantErr: true,
		},
		{
			name: "Dialect 'Castilian' on Spanish track is allowed",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "spa", Name: "Castilian", Default: true, Number: 1}},
			},
			wantErr: false,
		},
		{
			name: "Dialect 'Latino' on single Spanish track is allowed",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "spa", Name: "Latino", Default: true, Number: 1}},
			},
			wantErr: false,
		},
		{
			name: "Chinese dialects always allowed (Traditional)",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "chi", Name: "Traditional", Default: true, Number: 1}},
			},
			wantErr: false,
		},
		{
			name: "Chinese dialects: 'Chinese' is still redundant",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "chi", Name: "Chinese Traditional", Default: true, Number: 1}},
			},
			wantErr: true,
		},
		{
			name: "Zlib compression disabled",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Number: 1, ContentEncodingAlgorithms: ""}},
			},
			wantErr: false,
		},
		{
			name: "Zlib compression enabled",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", Number: 1, ContentEncodingAlgorithms: "0"}},
			},
			wantErr: true,
		},
		{
			name: "Zlib compression enabled (multiple)",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", Number: 1, ContentEncodingAlgorithms: "1,0"}},
			},
			wantErr: true,
		},
		{
			name: "Audio tracks are ignored",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Codec: "A_AC3", Properties: matroska.EbmlTrackProperties{Language: "ger", Number: 1}},
				{ID: 2, Type: "subtitles", Codec: "SRT", Properties: matroska.EbmlTrackProperties{Language: "eng", Number: 2}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := runTrackChecks(tt.tracks)

			hasFailure := false

			for _, r := range res {
				if !r.Passed {
					hasFailure = true
					break
				}
			}

			if hasFailure != tt.wantErr {
				t.Errorf("runTrackChecks() hasFailure = %v, wantErr %v", hasFailure, tt.wantErr)
			}
		})
	}
}

func TestGetTrackPriority(t *testing.T) {
	tests := []struct {
		name    string
		track   matroska.EbmlTrack
		wantMin int64
		wantMax int64
	}{
		{
			name:    "German Audio Default",
			track:   matroska.EbmlTrack{Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger"}},
			wantMin: priorityPreferred,
			wantMax: priorityPreferred + (int64(1) << 60) - 1,
		},
		{
			name:    "German Audio AD",
			track:   matroska.EbmlTrack{Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", VisualImpaired: true}},
			wantMin: priorityPreferred,
			wantMax: priorityPreferred + (int64(1) << 60) - 1,
		},
		{
			name:    "German Sub Forced",
			track:   matroska.EbmlTrack{Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", Forced: true}},
			wantMin: priorityPreferred,
			wantMax: priorityPreferred + (int64(1) << 60) - 1,
		},
		{
			name:    "German Sub SDH",
			track:   matroska.EbmlTrack{Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", HearingImpaired: true}},
			wantMin: priorityPreferred,
			wantMax: priorityPreferred + (int64(1) << 60) - 1,
		},
		{
			name:    "Original Language",
			track:   matroska.EbmlTrack{Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "fre", OriginalLanguage: true}},
			wantMin: priorityOriginal,
			wantMax: priorityOriginal + (int64(1) << 60) - 1,
		},
		{
			name:    "English Audio",
			track:   matroska.EbmlTrack{Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "eng"}},
			wantMin: priorityOther,
			wantMax: priorityOther + (int64(1) << 60) - 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getTrackPriority(tt.track)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("getTrackPriority() = %v, want range [%v, %v]", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestRunTrackChecksMultiTrack(t *testing.T) {
	config.InitDefaults()

	t.Run("Duplicate tracks returns both original and duplicate", func(t *testing.T) {
		tracks := []matroska.EbmlTrack{
			{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 1}},
			{ID: 2, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 2}},
		}

		res := runTrackChecks(tracks)
		found := false

		for _, r := range res {
			if r.Identifier == "matroska_duplicate_tracks" {
				found = true

				if len(r.Tracks) != 2 {
					t.Errorf("Expected 2 tracks for duplicate check, got %d", len(r.Tracks))
				}

				if r.Tracks[0].Warning != "original track" {
					t.Errorf("Expected first track warning to be 'original track', got '%s'", r.Tracks[0].Warning)
				}

				if !strings.Contains(r.Tracks[1].Warning, "duplicate track") {
					t.Errorf("Expected second track warning to contain 'duplicate track', got '%s'", r.Tracks[1].Warning)
				}
			}
		}

		if !found {
			t.Error("Did not find duplicate tracks check result")
		}
	})
}
