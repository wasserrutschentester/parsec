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
				{ID: 2, Type: "subtitles", Properties: EbmlTrackProperties{Language: "ger", HearingImpaired: true, Name: "German SDH"}},
			},
			wantErr: false,
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
				{ID: 2, Type: "audio", Properties: EbmlTrackProperties{Language: "ger", Commentary: true, Name: "Director's Commentary"}},
			},
			wantErr: false,
		},
		{
			name: "Alphabetical language order",
			tracks: []EbmlTrack{
				{ID: 1, Type: "audio", Properties: EbmlTrackProperties{Language: "ita", Default: true}},
				{ID: 2, Type: "audio", Properties: EbmlTrackProperties{Language: "spa", Default: true}},
			},
			wantErr: false,
		},
		{
			name: "Alphabetical language order (wrong)",
			tracks: []EbmlTrack{
				{ID: 1, Type: "audio", Properties: EbmlTrackProperties{Language: "spa", Default: true}},
				{ID: 2, Type: "audio", Properties: EbmlTrackProperties{Language: "ita", Default: true}},
			},
			wantErr: true,
		},
		{
			name: "Specialized track with Default flag (invalid)",
			tracks: []EbmlTrack{
				{ID: 1, Type: "audio", Properties: EbmlTrackProperties{Language: "ger", Default: true}},
				{ID: 2, Type: "audio", Properties: EbmlTrackProperties{Language: "ger", VisualImpaired: true, Default: true, Name: "AD"}},
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
