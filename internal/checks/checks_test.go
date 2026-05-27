package checks

import (
	"testing"

	"codeberg.org/n0ne/parsec/internal/mdb"
	"codeberg.org/n0ne/parsec/internal/metadata"
	"codeberg.org/n0ne/parsec/internal/metadata/mediainfo"
)

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
			got := CheckRedundantAudio(mi)
			if tt.wantWarn && len(got) == 0 {
				t.Errorf("CheckRedundantAudio() expected warnings, got none")
			}
			if !tt.wantWarn && len(got) > 0 {
				t.Errorf("CheckRedundantAudio() expected no warnings, got %v", got)
			}
		})
	}
}

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
			got := CheckResolution(track)
			if tt.wantWarn && len(got) == 0 {
				t.Errorf("CheckResolution() expected warnings, got none")
			}
			if !tt.wantWarn && len(got) > 0 {
				t.Errorf("CheckResolution() expected no warnings, got %v", got)
			}
		})
	}
}

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
			got := CheckFrameRate(track)
			if tt.wantWarn && len(got) == 0 {
				t.Errorf("CheckFrameRate(%f) expected warning, got none", tt.fps)
			}
			if !tt.wantWarn && len(got) > 0 {
				t.Errorf("CheckFrameRate(%f) expected no warning, got %v", tt.fps, got)
			}
		})
	}
}

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
			got := CheckBitRate(track)
			if tt.wantWarn && len(got) == 0 {
				t.Errorf("CheckBitRate(%d, %d) expected warning, got none", tt.height, tt.bitrate)
			}
			if !tt.wantWarn && len(got) > 0 {
				t.Errorf("CheckBitRate(%d, %d) expected no warning, got %v", tt.height, tt.bitrate, got)
			}
		})
	}
}

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
				{Type: "Video", Duration: 100.0},
				{Type: "Audio", Duration: 100.0, TypeOrder: &order1, ID: "1"},
			},
			wantWarn: false,
		},
		{
			name: "Audio slightly longer",
			tracks: []mediainfo.Track{
				{Type: "Video", Duration: 100.0},
				{Type: "Audio", Duration: 106.0, TypeOrder: &order1, ID: "1"},
			},
			wantWarn: true,
		},
		{
			name: "Audio slightly shorter",
			tracks: []mediainfo.Track{
				{Type: "Video", Duration: 100.0},
				{Type: "Audio", Duration: 70.0, TypeOrder: &order1, ID: "1"},
			},
			wantWarn: true,
		},
		{
			name: "Video missing",
			tracks: []mediainfo.Track{
				{Type: "Audio", Duration: 100.0, TypeOrder: &order1, ID: "1"},
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

			got := checkDurations(mi)
			if tt.wantWarn && len(got) == 0 {
				t.Errorf("checkDurations() expected warnings, got none")
			}
			if !tt.wantWarn && len(got) > 0 {
				t.Errorf("checkDurations() expected no warnings, got %v", got)
			}
		})
	}
}

func TestCheckYear(t *testing.T) {
	tests := []struct {
		name     string
		year     int
		season   int
		isTV     bool
		wantWarn bool
	}{
		{"Movie with year", 2023, 0, false, false},
		{"Movie without year", 0, 0, false, true},
		{"TV Show", 0, 1, true, false},
		{"Redundant Year", 2023, 2023, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := &metadata.Metadata{Year: tt.year, Season: tt.season, IsTV: tt.isTV}
			err := CheckYear(meta)
			if tt.wantWarn && err == nil {
				t.Errorf("CheckYear() expected error, got nil")
			}
			if !tt.wantWarn && err != nil {
				t.Errorf("CheckYear() expected no error, got %v", err)
			}
		})
	}
}

func TestCheckStreaming(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		service  string
		wantWarn bool
	}{
		{"WEB with service", "WEB-DL", "AMZN", false},
		{"WEB missing service", "WEB-DL", "", true},
		{"BluRay with service", "BluRay", "AMZN", true},
		{"BluRay without service", "BluRay", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := &metadata.Metadata{Source: tt.source, Service: tt.service}
			err := CheckStreaming(meta)
			if tt.wantWarn && err == nil {
				t.Errorf("CheckStreaming() expected error, got nil")
			}
			if !tt.wantWarn && err != nil {
				t.Errorf("CheckStreaming() expected no error, got %v", err)
			}
		})
	}
}

func TestCheckTvSpecial(t *testing.T) {
	tests := []struct {
		name         string
		isTV         bool
		season       int
		date         string
		episodeTitle string
		wantWarn     bool
	}{
		{"Regular TV", true, 1, "", "", false},
		{"Special with date/title", true, 0, "2023-01-01", "New Year Special", false},
		{"Special missing date", true, 0, "", "New Year Special", true},
		{"Special missing title", true, 0, "2023-01-01", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := &metadata.Metadata{IsTV: tt.isTV, Season: tt.season, Date: tt.date, EpisodeTitle: tt.episodeTitle}
			err := CheckTvSpecial(meta)
			if tt.wantWarn && err == nil {
				t.Errorf("CheckTvSpecial() expected error, got nil")
			}
			if !tt.wantWarn && err != nil {
				t.Errorf("CheckTvSpecial() expected no error, got %v", err)
			}
		})
	}
}

func TestNormalizeForComparison(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Movie.Title.2023", "movie title 2023"},
		{"Movie-Title-2023", "movie title 2023"},
		{"Movie Title (2023)", "movie title 2023"},
		{"Movie   Title", "movie title"},
	}

	for _, tt := range tests {
		got := NormalizeForComparison(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeForComparison(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCheckTitle(t *testing.T) {
	tests := []struct {
		name      string
		metaTitle string
		resTitle  string
		wantWarn  bool
	}{
		{"Match", "Movie Title", "Movie Title", false},
		{"Mismatch", "Movie Title", "Different Title", true},
		{"Normalization Match", "Movie.Title", "Movie Title", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := &metadata.Metadata{Title: tt.metaTitle}
			res := &mdb.SearchResult{Title: tt.resTitle}
			got := CheckTitle(meta, res)
			if tt.wantWarn && len(got) == 0 {
				t.Errorf("CheckTitle() expected warnings, got none")
			}
			if !tt.wantWarn && len(got) > 0 {
				t.Errorf("CheckTitle() expected no warnings, got %v", got)
			}
		})
	}
}

func TestCheckMovieYear(t *testing.T) {
	tests := []struct {
		name     string
		metaYear int
		resYear  int
		wantWarn bool
	}{
		{"Match", 2023, 2023, false},
		{"Mismatch", 2023, 2022, true},
		{"Meta Zero", 0, 2023, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := &metadata.Metadata{Year: tt.metaYear, IsTV: false}
			res := &mdb.SearchResult{Year: tt.resYear}
			got := CheckMovieYear(meta, res)
			if tt.wantWarn && len(got) == 0 {
				t.Errorf("CheckMovieYear() expected warnings, got none")
			}
			if !tt.wantWarn && len(got) > 0 {
				t.Errorf("CheckMovieYear() expected no warnings, got %v", got)
			}
		})
	}
}
