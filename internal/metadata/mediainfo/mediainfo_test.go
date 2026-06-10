package mediainfo

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata"
)

func TestSanitizeUTF8(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "Valid UTF-8",
			input:    []byte("Hello, World!"),
			expected: "Hello, World!",
		},
		{
			name:     "Valid UTF-8 with multi-byte",
			input:    []byte("Hellö, Wörld! ©"),
			expected: "Hellö, Wörld! ©",
		},
		{
			name:     "Windows-1252 invalid UTF-8 bytes",
			input:    []byte{0xA9, 0xAE, 0xBD, 0xE9}, // ©, ®, ½, é in Windows-1252
			expected: "©®½é",
		},
		{
			name:     "Mixed UTF-8 and Windows-1252",
			input:    append([]byte("Valid UTF-8: "), 0xA9),
			expected: "Valid UTF-8: ©",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := SanitizeUTF8(string(tt.input))
			if got != tt.expected {
				t.Errorf("SanitizeUTF8() = %q, want %q", got, tt.expected)
			}
		})
	}
}

//nolint:cyclop // complex nested JSON structure used for unmarshalling tests
func TestMediaInfo_UnmarshalFields(t *testing.T) {
	t.Parallel()

	jsonData := `{
		"media": {
			"track": [
				{
					"@type": "General",
					"VideoCount": "1",
					"AudioCount": "2",
					"FileSize": "12345678",
					"OverallBitRate": "5000"
				},
				{
					"@type": "Video",
					"Format_Profile": "High@L4.1",
					"BitDepth": "8",
					"ChromaSubsampling": "4:2:0",
					"StreamSize": "1000000",
					"FrameCount": "24000",
					"Encoded_Library": "x264"
				},
				{
					"@type": "Audio",
					"SamplingRate": "48000",
					"BitRate_Mode": "CBR"
				}
			]
		}
	}`

	var mi MediaInfo

	err := json.Unmarshal([]byte(jsonData), &mi)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	gen := mi.Media.Tracks[0]
	if gen.VideoCount != 1 || gen.AudioCount != 2 || gen.FileSize != 12345678 || gen.OverallBitRate != 5000 {
		t.Errorf("General track fields mismatch: %+v", gen)
	}

	video := mi.Media.Tracks[1]
	if video.FormatProfile != "High@L4.1" || video.BitDepth != 8 || video.ChromaSubsampling != "4:2:0" || video.StreamSize != 1000000 || video.FrameCount != 24000 || video.EncodedLibrary != "x264" {
		t.Errorf("Video track fields mismatch: %+v", video)
	}

	audio := mi.Media.Tracks[2]
	if audio.SamplingRate != 48000 || audio.BitRateMode != "CBR" {
		t.Errorf("Audio track fields mismatch: %+v", audio)
	}
}

//nolint:funlen // large embedded JSON string is needed for comprehensive unmarshalling tests
func TestMediaInfo_Unmarshal(t *testing.T) {
	t.Parallel()

	jsonData := `{
		"creatingLibrary": {
			"name": "MediaInfoLib",
			"version": "24.01",
			"url": "https://mediaarea.net"
		},
		"media": {
			"@ref": "test.mkv",
			"track": [
				{
					"@type": "General",
					"Duration": "123.456",
					"extra": {
						"IMDB": "tt1234567"
					}
				},
				{
					"@type": "Video",
					"Width": "1920",
					"Height": "1080",
					"FrameRate": "23.976",
					"Default": "Yes",
					"Forced": "No"
				},
				{
					"@type": "Audio",
					"Channels": "6",
					"Default": "No",
					"Forced": "No"
				}
			]
		}
	}`

	var mi MediaInfo

	err := json.Unmarshal([]byte(jsonData), &mi)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if len(mi.Media.Tracks) != 3 {
		t.Fatalf("Expected 3 tracks, got %d", len(mi.Media.Tracks))
	}

	video := mi.Media.Tracks[1]
	if video.Width != 1920 || video.Height != 1080 {
		t.Errorf("Video dimensions mismatch: %dx%d", video.Width, video.Height)
	}

	if video.FrameRate != 23.976 {
		t.Errorf("Video FrameRate mismatch: %f", video.FrameRate)
	}

	if !bool(video.Default) {
		t.Errorf("Video Default expected true, got false")
	}

	if bool(video.Forced) {
		t.Errorf("Video Forced expected false, got true")
	}

	audio := mi.Media.Tracks[2]
	if audio.Channels != 6 {
		t.Errorf("Audio channels mismatch: %d", audio.Channels)
	}

	if bool(audio.Default) {
		t.Errorf("Audio Default expected false, got true")
	}
}

func TestExtra_GetString(t *testing.T) {
	t.Parallel()

	e := Extra{
		"stringKey": "stringValue",
		"floatKey":  float64(123),
		"otherKey":  123,
	}

	tests := []struct {
		key  string
		want string
	}{
		{"stringKey", "stringValue"},
		{"floatKey", "123"},
		{"otherKey", ""},
		{"missingKey", ""},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			t.Parallel()

			if got := e.GetString(tt.key); got != tt.want {
				t.Errorf("GetString(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

//nolint:funlen // test cases cover various combinations of database IDs and types
func TestMediaInfo_GetMdbIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		extra    Extra
		wantImdb string
		wantTmdb int
		wantTvdb int
		wantIsTV bool
	}{
		{
			name: "IMDB only",
			extra: Extra{
				"IMDB": "tt1234567",
			},
			wantImdb: "tt1234567",
		},
		{
			name: "TMDB Movie",
			extra: Extra{
				"TMDB": "movie/123",
			},
			wantTmdb: 123,
		},
		{
			name: "TMDB TV",
			extra: Extra{
				"TMDB": "tv/456",
			},
			wantTmdb: 456,
			wantIsTV: true,
		},
		{
			name: "TVDB Series",
			extra: Extra{
				"TVDB": "series/789",
			},
			wantTvdb: 789,
			wantIsTV: true,
		},
		{
			name: "TVDB2 Series",
			extra: Extra{
				"TVDB2": "series/101",
			},
			wantTvdb: 101,
			wantIsTV: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mi := &MediaInfo{
				Media: Media{
					Tracks: []Track{
						{
							Type:  "General",
							Extra: tt.extra,
						},
					},
				},
			}

			imdb, tmdb, tvdb, isTV := mi.GetMdbIDs()
			if imdb != tt.wantImdb {
				t.Errorf("GetMdbIDs() imdb = %v, want %v", imdb, tt.wantImdb)
			}

			if tmdb != tt.wantTmdb {
				t.Errorf("GetMdbIDs() tmdb = %v, want %v", tmdb, tt.wantTmdb)
			}

			if tvdb != tt.wantTvdb {
				t.Errorf("GetMdbIDs() tvdb = %v, want %v", tvdb, tt.wantTvdb)
			}

			if isTV != tt.wantIsTV {
				t.Errorf("GetMdbIDs() isTV = %v, want %v", isTV, tt.wantIsTV)
			}
		})
	}
}

func TestMediaInfo_GetAudioLanguages(t *testing.T) {
	t.Parallel()

	mi := &MediaInfo{
		Media: Media{
			Tracks: []Track{
				{Type: "General"},
				{Type: "Video"},
				{Type: "Audio", Language: "de"},
				{Type: "Audio", Language: "en"},
				{Type: "Audio", Language: "de"}, // Duplicate
			},
		},
	}

	want := []string{"de", "en"}

	got := mi.GetAudioLanguages()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetAudioLanguages() = %v, want %v", got, want)
	}
}

func TestMediaInfo_GetSubtitleLanguages(t *testing.T) {
	t.Parallel()

	mi := &MediaInfo{
		Media: Media{
			Tracks: []Track{
				{Type: "Text", Language: "de"},
				{Type: "Text", Language: "en"},
			},
		},
	}

	want := []string{"de", "en"}

	got := mi.GetSubtitleLanguages()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetSubtitleLanguages() = %v, want %v", got, want)
	}
}

//nolint:funlen,paralleltest // test cases for language tagging involve many scenarios and track combinations; depends on shared global state (viper, config.InitDefaults)
func TestMediaInfo_GetLanguageTag(t *testing.T) {
	config.InitDefaults() // preferred_language = "de"

	tests := []struct {
		name          string
		tracks        []Track
		subbedTagging bool
		want          string
	}{
		{
			name: "Single language (German)",
			tracks: []Track{
				{Type: "Audio", Language: "de"},
			},
			subbedTagging: true,
			want:          "GERMAN",
		},
		{
			name: "Dual language (German/English)",
			tracks: []Track{
				{Type: "Audio", Language: "de"},
				{Type: "Audio", Language: "de"},
				{Type: "Audio", Language: "en"},
			},
			subbedTagging: true,
			want:          "GERMAN.DL",
		},
		{
			name: "Multi language (3+)",
			tracks: []Track{
				{Type: "Audio", Language: "de"},
				{Type: "Audio", Language: "en"},
				{Type: "Audio", Language: "fr"},
			},
			subbedTagging: true,
			want:          "GERMAN.ML",
		},
		{
			name: "Subbed (English audio, German subs)",
			tracks: []Track{
				{Type: "Audio", Language: "en"},
				{Type: "Text", Language: "de"},
			},
			subbedTagging: true,
			want:          "GERMAN.SUBBED",
		},
		{
			name: "Subbed override disabled (English audio, German subs)",
			tracks: []Track{
				{Type: "Audio", Language: "en"},
				{Type: "Text", Language: "de"},
			},
			subbedTagging: false,
			want:          "ENGLISH",
		},
		{
			name: "Not subbed (English audio, French subs, preferred is de)",
			tracks: []Track{
				{Type: "Audio", Language: "en"},
				{Type: "Text", Language: "fr"},
			},
			subbedTagging: true,
			want:          "ENGLISH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set("subbed_tagging", tt.subbedTagging)
			mi := &MediaInfo{
				Media: Media{
					Tracks: tt.tracks,
				},
			}
			meta := &metadata.Metadata{}
			mi.SetLanguageTag(meta)

			if got := strings.Trim(meta.Language+"."+meta.LanguageExt, "."); got != tt.want {
				t.Errorf("GetLanguageTag() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMediaInfo_GetMetadata(t *testing.T) {
	t.Parallel()

	mi := &MediaInfo{
		Media: Media{
			Tracks: []Track{
				{
					Type:      "Video",
					Height:    1080,
					Format:    "AVC",
					HDRFormat: "Dolby Vision / HDR10",
					BitDepth:  10,
				},
				{
					Type:                     "Audio",
					Format:                   "E-AC-3",
					FormatAdditionalFeatures: "JOC", // Atmos
					Title:                    "Atmos",
					Channels:                 6,
					Language:                 "de",
				},
			},
		},
	}

	got := mi.GetMetadata()
	want := &metadata.Metadata{
		Resolution:    "1080p",
		VideoCodec:    "H.264",
		AudioCodec:    "DDP",
		AudioChannels: "5.1",
		Language:      "GERMAN",
		HDR:           "DV.HDR",
		AudioMeta:     "Atmos",
		BitDepth:      10,
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetMetadata() mismatch. got: %+v, want: %+v", got, want)
	}
}

func TestTrack_detectHDR(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		track    Track
		expected string
	}{
		{
			name: "Dolby Vision and HDR10",
			track: Track{
				HDRFormat:              "Dolby Vision",
				HDRFormatCompatibility: "HDR10",
			},
			expected: "DV.HDR",
		},
		{
			name: "HDR10+",
			track: Track{
				HDRFormat: "HDR10+",
			},
			expected: "HDR10Plus",
		},
		{
			name: "HLG",
			track: Track{
				TransferCharacteristics: "HLG",
			},
			expected: "HLG",
		},
		{
			name: "PQ10",
			track: Track{
				TransferCharacteristics: "PQ",
			},
			expected: "PQ10",
		},
		{
			name: "No HDR",
			track: Track{
				HDRFormat: "",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.track.detectHDR(); got != tt.expected {
				t.Errorf("detectHDR() = %v, want %v", got, tt.expected)
			}
		})
	}
}
