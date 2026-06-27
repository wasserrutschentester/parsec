package checks

import (
	"strings"
	"testing"

	"github.com/spf13/viper"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
)

func TestGetTrackPriority(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			got := getTrackPriority(tt.track)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("getTrackPriority() = %v, want range [%v, %v]", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

//nolint:paralleltest // depends on shared global state
func TestRunTrackChecksDuplicateTracks(t *testing.T) {
	viper.Reset()
	config.InitDefaults()

	tracks := []matroska.EbmlTrack{
		{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 1}},
		{ID: 2, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 2}},
	}

	res := runTrackChecks("", &matroska.EbmlMetadata{Tracks: tracks}, nil, nil, nil)
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
}

//nolint:paralleltest,funlen // depends on shared global state; comprehensive metrics tests
func TestRunTrackChecksTrackMetrics(t *testing.T) {
	config.InitDefaults()
	viper.Set("enabled_checks", []string{"matroska_track_delay", "matroska_video_cropping"})

	t.Run("Track Delays", func(t *testing.T) {
		tests := []struct {
			name       string
			ebml       *matroska.EbmlMetadata
			identifier string
			wantErr    bool
		}{
			{
				name: "Track with reasonable delay",
				ebml: &matroska.EbmlMetadata{
					Tracks: []matroska.EbmlTrack{
						{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{CodecDelay: 5000000, Language: "ger", Number: 1}},
					},
				},
				identifier: "matroska_track_delay",
				wantErr:    false,
			},
			{
				name: "Track with excessive delay",
				ebml: &matroska.EbmlMetadata{
					Tracks: []matroska.EbmlTrack{
						{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{CodecDelay: 2000000000, Language: "ger", Number: 1}},
					},
				},
				identifier: "matroska_track_delay",
				wantErr:    true,
			},
		}

		for _, tt := range tests {
			runHygieneTest(t, tt.name, tt.ebml, nil, tt.identifier, tt.wantErr)
		}
	})

	t.Run("Video Cropping", func(t *testing.T) {
		tests := []struct {
			name       string
			ebml       *matroska.EbmlMetadata
			identifier string
			wantErr    bool
		}{
			{
				name: "Video with proper cropping",
				ebml: &matroska.EbmlMetadata{
					Tracks: []matroska.EbmlTrack{
						{ID: 1, Type: "video", Properties: matroska.EbmlTrackProperties{PixelDimensions: "1920x1080", DisplayDimensions: "1920x1080"}},
					},
				},
				identifier: "matroska_video_cropping",
				wantErr:    false,
			},
			{
				name: "Video with resolution-based black bars but no MKV crop",
				ebml: &matroska.EbmlMetadata{
					Tracks: []matroska.EbmlTrack{
						{ID: 1, Type: "video", Properties: matroska.EbmlTrackProperties{PixelDimensions: "1920x1080", DisplayDimensions: "1920x800"}},
					},
				},
				identifier: "matroska_video_cropping",
				wantErr:    true,
			},
		}

		for _, tt := range tests {
			runHygieneTest(t, tt.name, tt.ebml, nil, tt.identifier, tt.wantErr)
		}
	})
}
