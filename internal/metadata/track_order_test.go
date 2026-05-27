package metadata

import (
	"testing"
)

func TestVerifyTrackOrder(t *testing.T) {
	tests := []struct {
		name    string
		tracks  []EbmlTrack
		wantErr bool
	}{
		{
			name: "Valid German and English tracks",
			tracks: []EbmlTrack{
				{ID: 1, Type: "audio", Properties: EbmlTrackProperties{Language: "ger", Default: true}},
				{ID: 2, Type: "audio", Properties: EbmlTrackProperties{Language: "eng", Default: true}},
				{ID: 3, Type: "subtitles", Properties: EbmlTrackProperties{Language: "ger", Forced: true, Name: "Forced"}},
				{ID: 4, Type: "subtitles", Properties: EbmlTrackProperties{Language: "ger", Default: true}},
			},
			wantErr: false,
		},
		{
			name: "Out of order language (eng before ger)",
			tracks: []EbmlTrack{
				{ID: 1, Type: "audio", Properties: EbmlTrackProperties{Language: "eng", Default: true}},
				{ID: 2, Type: "audio", Properties: EbmlTrackProperties{Language: "ger", Default: true}},
			},
			wantErr: true,
		},
		{
			name: "Out of order subtitle properties (Default before Forced)",
			tracks: []EbmlTrack{
				{ID: 1, Type: "subtitles", Properties: EbmlTrackProperties{Language: "ger", Default: true}},
				{ID: 2, Type: "subtitles", Properties: EbmlTrackProperties{Language: "ger", Forced: true, Name: "Forced"}},
			},
			wantErr: true,
		},
		{
			name: "Missing Name for mul track",
			tracks: []EbmlTrack{
				{ID: 1, Type: "audio", Properties: EbmlTrackProperties{Language: "mul", Name: "", Default: true}},
			},
			wantErr: true,
		},
		{
			name: "Duplicate track detected",
			tracks: []EbmlTrack{
				{ID: 1, Type: "audio", Properties: EbmlTrackProperties{Language: "ger", Default: true}},
				{ID: 2, Type: "audio", Properties: EbmlTrackProperties{Language: "ger", Default: true}},
			},
			wantErr: true,
		},
		{
			name: "SDH missing keyword",
			tracks: []EbmlTrack{
				{ID: 1, Type: "audio", Properties: EbmlTrackProperties{Language: "ger", Default: true}},
				{ID: 2, Type: "subtitles", Properties: EbmlTrackProperties{Language: "ger", HearingImpaired: true}},
			},
			wantErr: true,
		},
		{
			name: "SDH with keyword",
			tracks: []EbmlTrack{
				{ID: 1, Type: "audio", Properties: EbmlTrackProperties{Language: "ger", Default: true}},
				{ID: 2, Type: "subtitles", Properties: EbmlTrackProperties{Language: "ger", HearingImpaired: true, Name: "SDH"}},
			},
			wantErr: false,
		},
		{
			name: "SDH keyword without flag",
			tracks: []EbmlTrack{
				{ID: 1, Type: "audio", Properties: EbmlTrackProperties{Language: "ger", Default: true}},
				{ID: 2, Type: "subtitles", Properties: EbmlTrackProperties{Language: "ger", Name: "SDH"}},
			},
			wantErr: true,
		},
		{
			name: "OriginalLanguage inconsistency",
			tracks: []EbmlTrack{
				{ID: 1, Type: "audio", Properties: EbmlTrackProperties{Language: "fre", OriginalLanguage: true, Default: true}},
				{ID: 2, Type: "subtitles", Properties: EbmlTrackProperties{Language: "fre", Default: true}},
			},
			wantErr: true,
		},
		{
			name: "Commentary missing keyword",
			tracks: []EbmlTrack{
				{ID: 1, Type: "audio", Properties: EbmlTrackProperties{Language: "ger", Commentary: true, Default: true}},
			},
			wantErr: true,
		},
		{
			name: "Commentary with keyword",
			tracks: []EbmlTrack{
				{ID: 1, Type: "audio", Properties: EbmlTrackProperties{Language: "ger", Default: true}},
				{ID: 2, Type: "audio", Properties: EbmlTrackProperties{Language: "ger", Commentary: true, Name: "Commentary"}},
			},
			wantErr: false,
		},
		{
			name: "Visual impaired missing keyword",
			tracks: []EbmlTrack{
				{ID: 1, Type: "audio", Properties: EbmlTrackProperties{Language: "ger", VisualImpaired: true, Default: true}},
			},
			wantErr: true,
		},
		{
			name: "Visual impaired with Descriptive",
			tracks: []EbmlTrack{
				{ID: 1, Type: "audio", Properties: EbmlTrackProperties{Language: "ger", Default: true}},
				{ID: 2, Type: "audio", Properties: EbmlTrackProperties{Language: "ger", VisualImpaired: true, Name: "Descriptive"}},
			},
			wantErr: false,
		},
		{
			name: "Visual impaired keyword without flag",
			tracks: []EbmlTrack{
				{ID: 1, Type: "audio", Properties: EbmlTrackProperties{Language: "ger", Default: true}},
				{ID: 2, Type: "audio", Properties: EbmlTrackProperties{Language: "ger", Name: "AD"}},
			},
			wantErr: true,
		},
		{
			name: "mul language with 1 language in name",
			tracks: []EbmlTrack{
				{ID: 1, Type: "audio", Properties: EbmlTrackProperties{Language: "mul", Name: "English", Default: true}},
			},
			wantErr: true,
		},
		{
			name: "mul language with 2 languages in name",
			tracks: []EbmlTrack{
				{ID: 1, Type: "audio", Properties: EbmlTrackProperties{Language: "mul", Name: "English / German", Default: true}},
			},
			wantErr: false,
		},
		{
			name: "Alphabetical language order",
			tracks: []EbmlTrack{
				{ID: 1, Type: "audio", Properties: EbmlTrackProperties{Language: "spa", Default: true}},
				{ID: 2, Type: "audio", Properties: EbmlTrackProperties{Language: "ita", Default: true}},
			},
			wantErr: false,
		},
		{
			name: "Alphabetical language order (wrong)",
			tracks: []EbmlTrack{
				{ID: 1, Type: "audio", Properties: EbmlTrackProperties{Language: "ita", Default: true}},
				{ID: 2, Type: "audio", Properties: EbmlTrackProperties{Language: "spa", Default: true}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := VerifyTrackOrder(tt.tracks); (err != nil) != tt.wantErr {
				t.Errorf("VerifyTrackOrder() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTrackNameChecks(t *testing.T) {
	t.Run("Quality (Junk)", func(t *testing.T) {
		tests := []struct {
			name    string
			track   EbmlTrack
			wantErr bool
		}{
			{"Junk keyword", EbmlTrack{Type: "audio", Properties: EbmlTrackProperties{Name: "German Stereo"}}, true},
			{"Valid name", EbmlTrack{Type: "audio", Properties: EbmlTrackProperties{Name: "Swissgerman / SDH"}}, false},
		}
		for _, tt := range tests {
			if err := checkTrackNameQuality(tt.track); (err != nil) != tt.wantErr {
				t.Errorf("%s: checkTrackNameQuality() error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		}
	})

	t.Run("Codecs", func(t *testing.T) {
		tests := []struct {
			name    string
			track   EbmlTrack
			wantErr bool
		}{
			{"Simple codec AC3", EbmlTrack{Type: "audio", Properties: EbmlTrackProperties{Name: "AC3"}}, true},
			{"Simple codec DTS", EbmlTrack{Type: "audio", Properties: EbmlTrackProperties{Name: "DTS 5.1"}}, true},
			{"Complex codec DTS-HD", EbmlTrack{Type: "audio", Properties: EbmlTrackProperties{Name: "DTS-HD Master Audio"}}, false},
		}
		for _, tt := range tests {
			if err := checkTrackNameCodecs(tt.track); (err != nil) != tt.wantErr {
				t.Errorf("%s: checkTrackNameCodecs() error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		}
	})

	t.Run("Redundant Lang", func(t *testing.T) {
		tests := []struct {
			name    string
			track   EbmlTrack
			wantErr bool
		}{
			{"Redundant German", EbmlTrack{Type: "audio", Properties: EbmlTrackProperties{Language: "ger", Name: "German"}}, true},
			{"Non-redundant English", EbmlTrack{Type: "audio", Properties: EbmlTrackProperties{Language: "ger", Name: "English"}}, false},
		}
		for _, tt := range tests {
			if err := checkTrackNameRedundantLang(tt.track); (err != nil) != tt.wantErr {
				t.Errorf("%s: checkTrackNameRedundantLang() error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		}
	})
}

func TestCheckDefaultFlags(t *testing.T) {
	tests := []struct {
		name    string
		tracks  []EbmlTrack
		wantErr bool
	}{
		{
			name: "Correct default flags",
			tracks: []EbmlTrack{
				{ID: 1, Type: "audio", Properties: EbmlTrackProperties{Language: "ger", Default: true}},
				{ID: 2, Type: "audio", Properties: EbmlTrackProperties{Language: "eng", Default: true}},
				{ID: 3, Type: "subtitles", Properties: EbmlTrackProperties{Language: "ger", Forced: true, Name: "Forced"}},
				{ID: 4, Type: "subtitles", Properties: EbmlTrackProperties{Language: "ger", Default: true}},
			},
			wantErr: false,
		},
		{
			name: "Missing default flags for first non-special subs track",
			tracks: []EbmlTrack{
				{ID: 1, Type: "audio", Properties: EbmlTrackProperties{Language: "ger", Default: true}},
				{ID: 2, Type: "audio", Properties: EbmlTrackProperties{Language: "eng", Default: true}},
				{ID: 3, Type: "subtitles", Properties: EbmlTrackProperties{Language: "ger", Forced: true, Name: "Forced"}},
				{ID: 4, Type: "subtitles", Properties: EbmlTrackProperties{Language: "ger", Default: false}},
			},
			wantErr: true,
		},
		{
			name: "Missing default flag for first audio",
			tracks: []EbmlTrack{
				{ID: 1, Type: "audio", Properties: EbmlTrackProperties{Language: "ger", Default: false}},
				{ID: 2, Type: "audio", Properties: EbmlTrackProperties{Language: "ger", Commentary: true, Name: "Commentary"}},
			},
			wantErr: true,
		},
		{
			name: "Relaxation: single track without default flag",
			tracks: []EbmlTrack{
				{ID: 1, Type: "audio", Properties: EbmlTrackProperties{Language: "ger", Default: false}},
			},
			wantErr: false,
		},
		{
			name: "Specialized track with Default flag (invalid)",
			tracks: []EbmlTrack{
				{ID: 1, Type: "audio", Properties: EbmlTrackProperties{Language: "ger", Default: true}},
				{ID: 2, Type: "audio", Properties: EbmlTrackProperties{Language: "ger", VisualImpaired: true, Default: true, Name: "AD"}},
			},
			wantErr: true,
		},
		{
			name: "Multiple default flags for same language",
			tracks: []EbmlTrack{
				{ID: 1, Type: "audio", Properties: EbmlTrackProperties{Language: "ger", Default: true}},
				{ID: 2, Type: "audio", Properties: EbmlTrackProperties{Language: "ger", Default: true}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CheckDefaultFlags(tt.tracks); (err != nil) != tt.wantErr {
				t.Errorf("CheckDefaultFlags() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckSubtitleFormat(t *testing.T) {
	tests := []struct {
		name    string
		tracks  []EbmlTrack
		wantErr bool
	}{
		{
			name: "All SRT subtitles",
			tracks: []EbmlTrack{
				{ID: 1, Type: "subtitles", Codec: "S_TEXT/SRT"},
				{ID: 2, Type: "subtitles", Codec: "SRT"},
			},
			wantErr: false,
		},
		{
			name: "Contains non-SRT subtitle",
			tracks: []EbmlTrack{
				{ID: 1, Type: "subtitles", Codec: "S_TEXT/SRT"},
				{ID: 2, Type: "subtitles", Codec: "S_TEXT/ASS"},
			},
			wantErr: true,
		},
		{
			name: "Audio tracks are ignored",
			tracks: []EbmlTrack{
				{ID: 1, Type: "audio", Codec: "A_AC3"},
				{ID: 2, Type: "subtitles", Codec: "SRT"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CheckSubtitleFormat(tt.tracks); (err != nil) != tt.wantErr {
				t.Errorf("CheckSubtitleFormat() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetTrackPriority(t *testing.T) {
	tests := []struct {
		name    string
		track   EbmlTrack
		wantMin int
		wantMax int
	}{
		{
			name:    "German Audio Default",
			track:   EbmlTrack{Type: "audio", Properties: EbmlTrackProperties{Language: "ger"}},
			wantMin: 1000,
			wantMax: 1000,
		},
		{
			name:    "German Audio AD",
			track:   EbmlTrack{Type: "audio", Properties: EbmlTrackProperties{Language: "ger", VisualImpaired: true}},
			wantMin: 1010,
			wantMax: 1010,
		},
		{
			name:    "German Sub Forced",
			track:   EbmlTrack{Type: "subtitles", Properties: EbmlTrackProperties{Language: "ger", Forced: true}},
			wantMin: 1000,
			wantMax: 1000,
		},
		{
			name:    "German Sub SDH",
			track:   EbmlTrack{Type: "subtitles", Properties: EbmlTrackProperties{Language: "ger", HearingImpaired: true}},
			wantMin: 1020,
			wantMax: 1020,
		},
		{
			name:    "Original Language",
			track:   EbmlTrack{Type: "audio", Properties: EbmlTrackProperties{Language: "fre", OriginalLanguage: true}},
			wantMin: 2000,
			wantMax: 2000,
		},
		{
			name:    "English Audio",
			track:   EbmlTrack{Type: "audio", Properties: EbmlTrackProperties{Language: "eng"}},
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
