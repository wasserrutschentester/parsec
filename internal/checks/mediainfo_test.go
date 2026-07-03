package checks

import (
	"testing"

	"codeberg.org/upPollo/parsec/internal/metadata/mediainfo"
)

func ptr(i int) *int {
	return &i
}

func ptrFloat(f float64) *float64 {
	return &f
}

//nolint:paralleltest // depends on shared global state
func TestCheckRedundantAudio(t *testing.T) {
	tests := []struct {
		name     string
		tracks   []mediainfo.Track
		wantWarn bool
	}{
		{
			name: "single audio",
			tracks: []mediainfo.Track{
				{Type: "Audio", Language: "en"},
			},
			wantWarn: false,
		},
		{
			name: "duplicate audio",
			tracks: []mediainfo.Track{
				{Type: "Audio", Language: "en"},
				{Type: "Audio", Language: "en"},
			},
			wantWarn: true,
		},
		{
			name: "audio and commentary",
			tracks: []mediainfo.Track{
				{Type: "Audio", Language: "en"},
				{Type: "Audio", Language: "en", Title: "Commentary by Director"},
			},
			wantWarn: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mi := &mediainfo.MediaInfo{
				Media: mediainfo.Media{
					Tracks: tt.tracks,
				},
			}
			results := checkRedundantAudio(mi)
			hasFailure := false

			for _, r := range results {
				if !r.Passed {
					hasFailure = true

					break
				}
			}

			if tt.wantWarn && !hasFailure {
				t.Errorf("checkRedundantAudio() expected warnings, got none")
			}

			if !tt.wantWarn && hasFailure {
				t.Errorf("checkRedundantAudio() expected no warnings, got failure")
			}
		})
	}
}

//nolint:paralleltest // depends on shared global state
func TestCheckResolution(t *testing.T) {
	tests := []struct {
		name     string
		width    int
		height   int
		wantWarn bool
	}{
		{
			name:     "Standard 1080p",
			width:    1920,
			height:   1080,
			wantWarn: false,
		},
		{
			name:     "Non-mod2",
			width:    1919,
			height:   800,
			wantWarn: true,
		},
		{
			name:     "Non-standard width",
			width:    1900,
			height:   800,
			wantWarn: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			track := &mediainfo.Track{
				Type:   "Video",
				Width:  tt.width,
				Height: tt.height,
			}
			results := checkResolution(track)
			hasFailure := false

			for _, r := range results {
				if !r.Passed {
					hasFailure = true

					break
				}
			}

			if tt.wantWarn && !hasFailure {
				t.Errorf("checkResolution() expected warnings, got none")
			}

			if !tt.wantWarn && hasFailure {
				t.Errorf("checkResolution() expected no warnings, got failure")
			}
		})
	}
}

//nolint:paralleltest // depends on shared global state
func TestCheckFrameRate(t *testing.T) {
	tests := []struct {
		name     string
		fps      float64
		wantWarn bool
	}{
		{"23.976", 23.976, false},
		{"24", 24.0, false},
		{"25", 25.0, false},
		{"29.97", 29.97, false},
		{"30", 30.0, false},
		{"50", 50.0, false},
		{"59.94", 59.94, false},
		{"60", 60.0, false},
		{"23.0", 23.0, true},
		{"0", 0.0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			track := &mediainfo.Track{FrameRate: tt.fps}
			results := checkFrameRate(track)
			hasFailure := false

			for _, r := range results {
				if !r.Passed {
					hasFailure = true

					break
				}
			}

			if tt.wantWarn && !hasFailure {
				t.Errorf("checkFrameRate(%f) expected warning, got none", tt.fps)
			}

			if !tt.wantWarn && hasFailure {
				t.Errorf("checkFrameRate(%f) expected no warning, got failure", tt.fps)
			}
		})
	}
}

//nolint:paralleltest // depends on shared global state
func TestCheckBitRate(t *testing.T) {
	tests := []struct {
		name     string
		height   int
		bitrate  int
		wantWarn bool
	}{
		{"1080p High", 1080, 5000000, false},
		{"1080p Low", 1080, 1500000, true},
		{"720p High", 720, 2000000, false},
		{"720p Low", 720, 800000, true},
		{"480p", 480, 400000, false},
		{"0 bitrate", 1080, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			track := &mediainfo.Track{Height: tt.height, BitRate: tt.bitrate}
			results := checkBitRate(track)
			hasFailure := false

			for _, r := range results {
				if !r.Passed {
					hasFailure = true

					break
				}
			}

			if tt.wantWarn && !hasFailure {
				t.Errorf("checkBitRate(%d, %d) expected warning, got none", tt.height, tt.bitrate)
			}

			if !tt.wantWarn && hasFailure {
				t.Errorf("checkBitRate(%d, %d) expected no warning, got failure", tt.height, tt.bitrate)
			}
		})
	}
}

//nolint:funlen,cyclop,paralleltest // test cases cover many edge cases in duration checks; depends on shared global state
func TestCheckDurations(t *testing.T) {
	order0 := 0
	order1 := 1

	tests := []struct {
		name     string
		tracks   []mediainfo.Track
		wantWarn bool
	}{
		{
			name: "Perfect match",
			tracks: []mediainfo.Track{
				{Type: "Video", Duration: ptrFloat(100.0)},
				{Type: "Audio", Duration: ptrFloat(100.0), TypeOrder: &order1, ID: "1"},
			},
			wantWarn: false,
		},
		{
			name: "Audio slightly longer",
			tracks: []mediainfo.Track{
				{Type: "Video", Duration: ptrFloat(100.0)},
				{Type: "Audio", Duration: ptrFloat(106.0), TypeOrder: &order1, ID: "1"},
			},
			wantWarn: true,
		},
		{
			name: "Audio slightly shorter",
			tracks: []mediainfo.Track{
				{Type: "Video", Duration: ptrFloat(100.0)},
				{Type: "Audio", Duration: ptrFloat(70.0), TypeOrder: &order1, ID: "1"},
			},
			wantWarn: true,
		},
		{
			name: "Video missing",
			tracks: []mediainfo.Track{
				{Type: "Audio", Duration: ptrFloat(100.0), TypeOrder: &order1, ID: "1"},
			},
			wantWarn: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mi := &mediainfo.MediaInfo{
				Media: mediainfo.Media{
					Tracks: tt.tracks,
				},
			}
			// Initialize TypeOrder for Video if not set
			for i := range mi.Media.Tracks {
				if mi.Media.Tracks[i].Type == "Video" && mi.Media.Tracks[i].TypeOrder == nil {
					mi.Media.Tracks[i].TypeOrder = &order0
				}
			}

			results := checkDurations(mi)
			hasFailure := false

			for _, r := range results {
				if !r.Passed {
					hasFailure = true

					break
				}
			}

			if tt.wantWarn && !hasFailure {
				t.Errorf("checkDurations() expected warnings, got none")
			}

			if !tt.wantWarn && hasFailure {
				t.Errorf("checkDurations() expected no warnings, got failure")
			}
		})
	}
}

//nolint:funlen,paralleltest // numerous test cases are needed to cover many codec and normalization combinations; depends on shared global state
func TestCheckDialogueNormalization(t *testing.T) {
	tests := []struct {
		name     string
		track    mediainfo.Track
		wantWarn bool
	}{
		{
			name: "TrueHD with Dialog_Normalization",
			track: mediainfo.Track{
				Type:                "Audio",
				Format:              "MLP FBA",
				DialogNormalization: "-27 dB",
			},
			wantWarn: true,
		},
		{
			name: "TrueHD with dialnorm in Extra",
			track: mediainfo.Track{
				Type:   "Audio",
				Format: "MLP FBA",
				Extra:  mediainfo.Extra{"dialnorm": "-27"},
			},
			wantWarn: true,
		},
		{
			name: "TrueHD without DialNorm",
			track: mediainfo.Track{
				Type:   "Audio",
				Format: "MLP FBA",
			},
			wantWarn: false,
		},
		{
			name: "DTS-HD MA with DialNorm",
			track: mediainfo.Track{
				Type:                "Audio",
				Format:              "DTS",
				FormatProfile:       "MA / Core",
				DialogNormalization: "-27 dB",
			},
			wantWarn: true,
		},
		{
			name: "AC-3 with DialNorm (Allowed)",
			track: mediainfo.Track{
				Type:                "Audio",
				Format:              "AC-3",
				DialogNormalization: "-27 dB",
			},
			wantWarn: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mi := &mediainfo.MediaInfo{
				Media: mediainfo.Media{
					Tracks: []mediainfo.Track{tt.track},
				},
			}
			results := checkDialogueNormalization(mi)
			hasFailure := false

			for _, r := range results {
				if !r.Passed {
					hasFailure = true

					break
				}
			}

			if tt.wantWarn && !hasFailure {
				t.Errorf("checkDialogueNormalization() expected warning, got none")
			}

			if !tt.wantWarn && hasFailure {
				t.Errorf("checkDialogueNormalization() expected no warning, got failure")
			}
		})
	}
}

//nolint:funlen,paralleltest // numerous test cases are needed to cover many codec and channel combinations; depends on shared global state
func TestCheckStereoLossless(t *testing.T) {
	tests := []struct {
		name     string
		track    mediainfo.Track
		wantWarn bool
	}{
		{
			name: "Stereo FLAC (Allowed)",
			track: mediainfo.Track{
				Type:     "Audio",
				Format:   "FLAC",
				Channels: 2,
			},
			wantWarn: false,
		},
		{
			name: "Stereo TrueHD (Warning)",
			track: mediainfo.Track{
				Type:     "Audio",
				Format:   "MLP FBA",
				Channels: 2,
			},
			wantWarn: true,
		},
		{
			name: "Mono TrueHD (Warning)",
			track: mediainfo.Track{
				Type:     "Audio",
				Format:   "MLP FBA",
				Channels: 1,
			},
			wantWarn: true,
		},
		{
			name: "5.1 TrueHD (Allowed)",
			track: mediainfo.Track{
				Type:     "Audio",
				Format:   "MLP FBA",
				Channels: 6,
			},
			wantWarn: false,
		},
		{
			name: "Stereo DTS-HD MA (Warning)",
			track: mediainfo.Track{
				Type:          "Audio",
				Format:        "DTS",
				FormatProfile: "MA / Core",
				Channels:      2,
			},
			wantWarn: true,
		},
		{
			name: "Stereo AAC (Allowed - Lossy)",
			track: mediainfo.Track{
				Type:     "Audio",
				Format:   "AAC",
				Channels: 2,
			},
			wantWarn: false,
		},
		{
			name: "Stereo PCM (Warning)",
			track: mediainfo.Track{
				Type:     "Audio",
				Format:   "PCM",
				Channels: 2,
			},
			wantWarn: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mi := &mediainfo.MediaInfo{
				Media: mediainfo.Media{
					Tracks: []mediainfo.Track{tt.track},
				},
			}
			results := checkStereoLossless(mi)
			hasFailure := false

			for _, r := range results {
				if !r.Passed {
					hasFailure = true

					break
				}
			}

			if tt.wantWarn && !hasFailure {
				t.Errorf("checkStereoLossless() expected warning, got none")
			}

			if !tt.wantWarn && hasFailure {
				t.Errorf("checkStereoLossless() expected no warning, got failure")
			}
		})
	}
}

//nolint:funlen,paralleltest // numerous test cases are needed to cover zero channels and zero elements check; depends on shared global state
func TestCheckZeroChannelsElements(t *testing.T) {
	tests := []struct {
		name     string
		track    mediainfo.Track
		wantWarn bool
	}{
		{
			name: "Audio track with channels (Allowed)",
			track: mediainfo.Track{
				Type:     "Audio",
				Format:   "AC-3",
				Channels: 6,
			},
			wantWarn: false,
		},
		{
			name: "Audio track with zero channels (Error)",
			track: mediainfo.Track{
				Type:     "Audio",
				Format:   "AC-3",
				Channels: 0,
			},
			wantWarn: true,
		},
		{
			name: "Subtitle track with elements (Allowed)",
			track: mediainfo.Track{
				Type:         "Text",
				Format:       "SRT",
				ElementCount: ptr(150),
			},
			wantWarn: false,
		},
		{
			name: "Subtitle track with zero elements (Error)",
			track: mediainfo.Track{
				Type:         "Text",
				Format:       "SRT",
				ElementCount: ptr(0),
			},
			wantWarn: true,
		},
		{
			name: "Subtitle track with zero elements in Extra (Error)",
			track: mediainfo.Track{
				Type:   "Text",
				Format: "SRT",
				Extra:  mediainfo.Extra{"ElementCount": "0"},
			},
			wantWarn: true,
		},
		{
			name: "Subtitle track with zero elements in Extra underscores (Error)",
			track: mediainfo.Track{
				Type:   "Text",
				Format: "SRT",
				Extra:  mediainfo.Extra{"Element_Count": "0"},
			},
			wantWarn: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mi := &mediainfo.MediaInfo{
				Media: mediainfo.Media{
					Tracks: []mediainfo.Track{tt.track},
				},
			}
			results := checkEmptyTracks(mi)
			hasFailure := false

			for _, r := range results {
				if !r.Passed {
					hasFailure = true

					break
				}
			}

			if tt.wantWarn && !hasFailure {
				t.Errorf("checkEmptyTracks() expected warning/error, got none")
			}

			if !tt.wantWarn && hasFailure {
				t.Errorf("checkEmptyTracks() expected no warning/error, got failure")
			}
		})
	}
}
