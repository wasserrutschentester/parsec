package checks

import (
	"testing"

	"codeberg.org/n0ne/parsec/internal/config"
	"codeberg.org/n0ne/parsec/internal/metadata/matroska"
)

func TestVerifyTrackOrder(t *testing.T) {
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
			name: "Visual impaired missing keyword",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", VisualImpaired: true, Default: true, Number: 1}},
			},
			wantErr: true,
		},
		{
			name: "Visual impaired with Descriptive",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 1}},
				{ID: 2, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", VisualImpaired: true, Name: "Descriptive", Number: 2}},
			},
			wantErr: false,
		},
		{
			name: "Visual impaired keyword without flag",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 1}},
				{ID: 2, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Name: "AD", Number: 2}},
			},
			wantErr: true,
		},
		{
			name: "mul language with 1 language in name",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "mul", Name: "English", Default: true, Number: 1}},
			},
			wantErr: true,
		},
		{
			name: "mul language with 2 languages in name",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "mul", Name: "English / German", Default: true, Number: 1}},
			},
			wantErr: false,
		},
		{
			name: "Alphabetical language order",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "spa", Default: true, Number: 1}},
				{ID: 2, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ita", Default: true, Number: 2}},
			},
			wantErr: false,
		},
		{
			name: "Alphabetical language order (wrong)",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ita", Default: true, Number: 1}},
				{ID: 2, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "spa", Default: true, Number: 2}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := VerifyTrackOrder(tt.tracks)
			hasFailure := len(res) > 0
			if hasFailure != tt.wantErr {
				t.Errorf("VerifyTrackOrder() hasFailure = %v, wantErr %v", hasFailure, tt.wantErr)
			}
		})
	}
}

func TestTrackNameChecks(t *testing.T) {
	t.Run("Quality (Junk)", func(t *testing.T) {
		tests := []struct {
			name    string
			track   matroska.EbmlTrack
			wantErr bool
		}{
			{"Junk keyword", matroska.EbmlTrack{Type: "audio", Properties: matroska.EbmlTrackProperties{Name: "German Stereo"}}, true},
			{"Valid name", matroska.EbmlTrack{Type: "audio", Properties: matroska.EbmlTrackProperties{Name: "Swissgerman / SDH"}}, false},
		}
		for _, tt := range tests {
			res := checkTrackNameQuality(tt.track)
			hasFailure := res != nil
			if hasFailure != tt.wantErr {
				t.Errorf("%s: checkTrackNameQuality() hasFailure = %v, wantErr %v", tt.name, hasFailure, tt.wantErr)
			}
		}
	})

	t.Run("Codecs", func(t *testing.T) {
		tests := []struct {
			name    string
			track   matroska.EbmlTrack
			wantErr bool
		}{
			{"Simple codec AC3", matroska.EbmlTrack{Type: "audio", Properties: matroska.EbmlTrackProperties{Name: "AC3"}}, true},
			{"Simple codec DTS", matroska.EbmlTrack{Type: "audio", Properties: matroska.EbmlTrackProperties{Name: "DTS 5.1"}}, true},
			{"Complex codec DTS-HD", matroska.EbmlTrack{Type: "audio", Properties: matroska.EbmlTrackProperties{Name: "DTS-HD Master Audio"}}, false},
		}
		for _, tt := range tests {
			res := checkTrackNameCodecs(tt.track)
			hasFailure := res != nil
			if hasFailure != tt.wantErr {
				t.Errorf("%s: checkTrackNameCodecs() hasFailure = %v, wantErr %v", tt.name, hasFailure, tt.wantErr)
			}
		}
	})

	t.Run("Redundant Lang", func(t *testing.T) {
		tests := []struct {
			name    string
			track   matroska.EbmlTrack
			wantErr bool
		}{
			{"Redundant German", matroska.EbmlTrack{Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Name: "German"}}, true},
			{"Non-redundant English", matroska.EbmlTrack{Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Name: "English"}}, false},
		}
		for _, tt := range tests {
			res := checkTrackNameRedundantLang(tt.track)
			hasFailure := res != nil
			if hasFailure != tt.wantErr {
				t.Errorf("%s: checkTrackNameRedundantLang() hasFailure = %v, wantErr %v", tt.name, hasFailure, tt.wantErr)
			}
		}
	})
}

func TestCheckDefaultFlags(t *testing.T) {
	tests := []struct {
		name    string
		tracks  []matroska.EbmlTrack
		wantErr bool
	}{
		{
			name: "Correct default flags",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 1}},
				{ID: 2, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "eng", Default: true, Number: 2}},
				{ID: 3, Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", Forced: true, Name: "Forced", Number: 3}},
				{ID: 4, Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 4}},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := CheckDefaultFlags(tt.tracks)
			hasFailure := len(res) > 0
			if hasFailure != tt.wantErr {
				t.Errorf("CheckDefaultFlags() hasFailure = %v, wantErr %v", hasFailure, tt.wantErr)
			}
		})
	}
}

func TestCheckSubtitleFormat(t *testing.T) {
	tests := []struct {
		name    string
		tracks  []matroska.EbmlTrack
		wantErr bool
	}{
		{
			name: "All SRT subtitles",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "subtitles", Codec: "S_TEXT/SRT", Properties: matroska.EbmlTrackProperties{Number: 1}},
				{ID: 2, Type: "subtitles", Codec: "SRT", Properties: matroska.EbmlTrackProperties{Number: 2}},
			},
			wantErr: false,
		},
		{
			name: "Contains non-SRT subtitle",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "subtitles", Codec: "S_TEXT/SRT", Properties: matroska.EbmlTrackProperties{TextSubtitles: true, Number: 1}},
				{ID: 2, Type: "subtitles", Codec: "S_TEXT/ASS", Properties: matroska.EbmlTrackProperties{TextSubtitles: true, Number: 2}},
			},
			wantErr: true,
		},
		{
			name: "Audio tracks are ignored",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Codec: "A_AC3", Properties: matroska.EbmlTrackProperties{Number: 1}},
				{ID: 2, Type: "subtitles", Codec: "SRT", Properties: matroska.EbmlTrackProperties{Number: 2}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := CheckSubtitleFormat(tt.tracks)
			hasFailure := len(res) > 0
			if hasFailure != tt.wantErr {
				t.Errorf("CheckSubtitleFormat() hasFailure = %v, wantErr %v", hasFailure, tt.wantErr)
			}
		})
	}
}

func TestGetTrackPriority(t *testing.T) {
	tests := []struct {
		name    string
		track   matroska.EbmlTrack
		wantMin int
		wantMax int
	}{
		{
			name:    "German Audio Default",
			track:   matroska.EbmlTrack{Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger"}},
			wantMin: 1000,
			wantMax: 1000,
		},
		{
			name:    "German Audio AD",
			track:   matroska.EbmlTrack{Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", VisualImpaired: true}},
			wantMin: 1010,
			wantMax: 1010,
		},
		{
			name:    "German Sub Forced",
			track:   matroska.EbmlTrack{Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", Forced: true}},
			wantMin: 1000,
			wantMax: 1000,
		},
		{
			name:    "German Sub SDH",
			track:   matroska.EbmlTrack{Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", HearingImpaired: true}},
			wantMin: 1020,
			wantMax: 1020,
		},
		{
			name:    "Original Language",
			track:   matroska.EbmlTrack{Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "fre", OriginalLanguage: true}},
			wantMin: 2000,
			wantMax: 2000,
		},
		{
			name:    "English Audio",
			track:   matroska.EbmlTrack{Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "eng"}},
			wantMin: 4000,
			wantMax: 4000,
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
