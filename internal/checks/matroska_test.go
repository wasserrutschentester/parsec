package checks

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
)

//nolint:funlen,paralleltest // comprehensive test cases for diverse matroska track configurations; depends on shared global state
func TestRunTrackChecks(t *testing.T) {
	config.InitDefaults()
	viper.Set("disabled_checks", []string{
		"matroska_subtitle_inline_fonts",
		"matroska_ass_events",
		"matroska_video_cropping",
		"matroska_title_hygiene",
		"matroska_app_hygiene",
		"matroska_track_delay",
	})

	tests := []struct {
		name      string
		tracks    []matroska.EbmlTrack
		chapters  []matroska.EbmlChapters
		container matroska.EbmlContainer
		wantErr   bool
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
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Name: "Full", Number: 1}},
				{ID: 2, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Name: "Main", Number: 2}},
			},
			wantErr: false,
		},
		{
			name: "Second standard track is only default",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: false, Name: "Full", Number: 1}},
				{ID: 2, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Name: "Main", Number: 2}},
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
			name: "Contains SRT and ASS subtitles",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "subtitles", Codec: "SubRip/SRT", Properties: matroska.EbmlTrackProperties{TextSubtitles: true, Language: "ger", Number: 1}},
				{ID: 2, Type: "subtitles", Codec: "SubStationAlpha", Properties: matroska.EbmlTrackProperties{TextSubtitles: true, Language: "eng", Number: 2}},
			},
			wantErr: false,
		},
		{
			name: "Contains SSA subtitle",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "subtitles", Codec: "SubStationAlpha", Properties: matroska.EbmlTrackProperties{TextSubtitles: true, Language: "ger", Number: 1}},
			},
			wantErr: false,
		},
		{
			name: "Contains unsupported text subtitle (WebVTT)",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "subtitles", Codec: "S_TEXT/WEBVTT", Properties: matroska.EbmlTrackProperties{TextSubtitles: true, Language: "ger", Number: 1}},
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
			name: "ASS Script Info missing headers",
			tracks: []matroska.EbmlTrack{
				{
					ID:    1,
					Type:  "subtitles",
					Codec: "S_TEXT/ASS",
					Properties: matroska.EbmlTrackProperties{
						Language:     "ger",
						Number:       1,
						CodecPrivate: "5b53637269707420496e666f5d0a536372697074547970653a2076342e30302b0a", // [Script Info]\nScriptType: v4.00+\n
					},
				},
			},
			wantErr: true,
		},
		{
			name: "ASS Style validation failure (invalid Fontsize)",
			tracks: []matroska.EbmlTrack{
				{
					ID:    1,
					Type:  "subtitles",
					Codec: "S_TEXT/ASS",
					Properties: matroska.EbmlTrackProperties{
						Language: "ger",
						Number:   1,
						// [V4+ Styles]\nFormat: Name, Fontname, Fontsize, ...\nStyle: Default, Arial, 600, ...
						CodecPrivate: "5b56342b205374796c65735d0a466f726d61743a204e616d652c20466f6e746e616d652c20466f6e7473697a650a5374796c653a2044656661756c742c20417269616c2c203630300a",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "ASS Script Info resolution mismatch",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "video", Properties: matroska.EbmlTrackProperties{PixelDimensions: "1920x1080"}},
				{
					ID:    2,
					Type:  "subtitles",
					Codec: "S_TEXT/ASS",
					Properties: matroska.EbmlTrackProperties{
						Language: "ger",
						Number:   2,
						// PlayResX: 640, PlayResY: 360 (Mismatch with 1920x1080)
						CodecPrivate: "5b53637269707420496e666f5d0a536372697074547970653a2076342e30302b0a5363616c6564426f72646572416e64536861646f773a207965730a5943624372204d61747269783a204e6f6e650a506c6179526573583a203634300a506c6179526573593a203336300a4c61796f7574526573583a20313932300a4c61796f7574526573593a20313038300a",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Valid TrueHD with AC3 compatibility track",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Codec: "A_TRUEHD", Properties: matroska.EbmlTrackProperties{Language: "eng", Default: true, Number: 1}},
				{ID: 2, Type: "audio", Codec: "A_AC3", Properties: matroska.EbmlTrackProperties{Language: "eng", Number: 2}},
			},
			wantErr: false,
		},
		{
			name: "Valid TrueHD with EAC3 compatibility track",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Codec: "A_TRUEHD", Properties: matroska.EbmlTrackProperties{Language: "eng", Default: true, Number: 1}},
				{ID: 2, Type: "audio", Codec: "A_EAC3", Properties: matroska.EbmlTrackProperties{Language: "eng", Number: 2}},
			},
			wantErr: false,
		},
		{
			name: "Invalid TrueHD - last track in file",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Codec: "A_TRUEHD", Properties: matroska.EbmlTrackProperties{Language: "eng", Number: 1}},
			},
			wantErr: true,
		},
		{
			name: "Invalid TrueHD - followed by different language",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Codec: "A_TRUEHD", Properties: matroska.EbmlTrackProperties{Language: "eng", Number: 1}},
				{ID: 2, Type: "audio", Codec: "A_AC3", Properties: matroska.EbmlTrackProperties{Language: "ger", Number: 2}},
			},
			wantErr: true,
		},
		{
			name: "Invalid TrueHD - followed by incompatible codec",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Codec: "A_TRUEHD", Properties: matroska.EbmlTrackProperties{Language: "eng", Number: 1}},
				{ID: 2, Type: "audio", Codec: "A_AAC", Properties: matroska.EbmlTrackProperties{Language: "eng", Number: 2}},
			},
			wantErr: true,
		},
		{
			name: "Invalid TrueHD - followed by non-audio track",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Codec: "A_TRUEHD", Properties: matroska.EbmlTrackProperties{Language: "eng", Number: 1}},
				{ID: 2, Type: "subtitles", Codec: "S_TEXT/UTF8", Properties: matroska.EbmlTrackProperties{Language: "eng", Number: 2}},
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
		{
			name: "Valid chapters",
			chapters: []matroska.EbmlChapters{
				{
					Editions: []matroska.EbmlEdition{
						{
							Chapters: []matroska.EbmlChapterAtom{
								{TimeStart: 0, Display: []matroska.EbmlDisplay{{String: "Intro", Language: "eng"}}},
								{TimeStart: 15000000000, Display: []matroska.EbmlDisplay{{String: "The Journey", Language: "eng"}}},
								{TimeStart: 120000000000, Display: []matroska.EbmlDisplay{{String: "Credits", Language: "eng"}}},
							},
						},
					},
				},
			},
			container: matroska.EbmlContainer{
				Properties: matroska.EbmlContainerProperties{Duration: 300000000000},
			},
			wantErr: false,
		},
		{
			name: "Invalid chapters - first start non-zero",
			chapters: []matroska.EbmlChapters{
				{
					Editions: []matroska.EbmlEdition{
						{
							Chapters: []matroska.EbmlChapterAtom{
								{TimeStart: 5000000000, Display: []matroska.EbmlDisplay{{String: "Intro", Language: "eng"}}},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid chapters - non-monotonic",
			chapters: []matroska.EbmlChapters{
				{
					Editions: []matroska.EbmlEdition{
						{
							Chapters: []matroska.EbmlChapterAtom{
								{TimeStart: 0, Display: []matroska.EbmlDisplay{{String: "Intro", Language: "eng"}}},
								{TimeStart: 120000000000, Display: []matroska.EbmlDisplay{{String: "The End", Language: "eng"}}},
								{TimeStart: 30000000000, Display: []matroska.EbmlDisplay{{String: "The Middle", Language: "eng"}}},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid chapters - duplicate timestamps",
			chapters: []matroska.EbmlChapters{
				{
					Editions: []matroska.EbmlEdition{
						{
							Chapters: []matroska.EbmlChapterAtom{
								{TimeStart: 0, Display: []matroska.EbmlDisplay{{String: "Intro", Language: "eng"}}},
								{TimeStart: 60000000000, Display: []matroska.EbmlDisplay{{String: "Part 2", Language: "eng"}}},
								{TimeStart: 60000000000, Display: []matroska.EbmlDisplay{{String: "Part 3", Language: "eng"}}},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid chapters - interval too close",
			chapters: []matroska.EbmlChapters{
				{
					Editions: []matroska.EbmlEdition{
						{
							Chapters: []matroska.EbmlChapterAtom{
								{TimeStart: 0, Display: []matroska.EbmlDisplay{{String: "Intro", Language: "eng"}}},
								{TimeStart: 5000000000, Display: []matroska.EbmlDisplay{{String: "Part 2", Language: "eng"}}},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid chapters - exceed duration",
			chapters: []matroska.EbmlChapters{
				{
					Editions: []matroska.EbmlEdition{
						{
							Chapters: []matroska.EbmlChapterAtom{
								{TimeStart: 0, Display: []matroska.EbmlDisplay{{String: "Intro", Language: "eng"}}},
								{TimeStart: 350000000000, Display: []matroska.EbmlDisplay{{String: "Outro", Language: "eng"}}},
							},
						},
					},
				},
			},
			container: matroska.EbmlContainer{
				Properties: matroska.EbmlContainerProperties{Duration: 300000000000},
			},
			wantErr: true,
		},
		{
			name: "Invalid chapters - empty display name",
			chapters: []matroska.EbmlChapters{
				{
					Editions: []matroska.EbmlEdition{
						{
							Chapters: []matroska.EbmlChapterAtom{
								{TimeStart: 0, Display: []matroska.EbmlDisplay{{String: "   ", Language: "eng"}}},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid chapters - duplicate consecutive display names",
			chapters: []matroska.EbmlChapters{
				{
					Editions: []matroska.EbmlEdition{
						{
							Chapters: []matroska.EbmlChapterAtom{
								{TimeStart: 0, Display: []matroska.EbmlDisplay{{String: "Intro", Language: "eng"}}},
								{TimeStart: 15000000000, Display: []matroska.EbmlDisplay{{String: "Intro", Language: "eng"}}},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid chapters - missing display language",
			chapters: []matroska.EbmlChapters{
				{
					Editions: []matroska.EbmlEdition{
						{
							Chapters: []matroska.EbmlChapterAtom{
								{TimeStart: 0, Display: []matroska.EbmlDisplay{{String: "Intro", Language: "und"}}},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid chapters - inconsistent display languages",
			chapters: []matroska.EbmlChapters{
				{
					Editions: []matroska.EbmlEdition{
						{
							Chapters: []matroska.EbmlChapterAtom{
								{TimeStart: 0, Display: []matroska.EbmlDisplay{{String: "Intro", Language: "eng"}}},
								{TimeStart: 15000000000, Display: []matroska.EbmlDisplay{{String: "Part 2", Language: "fre"}}},
							},
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := runTrackChecks("", &matroska.EbmlMetadata{
				Tracks:    tt.tracks,
				Chapters:  tt.chapters,
				Container: tt.container,
			}, nil, nil, nil)

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

//nolint:paralleltest // depends on shared global state
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

//nolint:paralleltest // depends on shared global state
func TestRunTrackChecksDuplicateTracks(t *testing.T) {
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

//nolint:paralleltest // depends on shared global state
func TestRunTrackChecksUnusedFonts(t *testing.T) {
	config.InitDefaults()

	ebml := &matroska.EbmlMetadata{
		Tracks: []matroska.EbmlTrack{
			{
				ID:    1,
				Type:  "subtitles",
				Codec: "S_TEXT/ASS",
				Properties: matroska.EbmlTrackProperties{
					Language: "ger",
					Number:   1,
					// [V4+ Styles]\nFormat: Name, Fontname\nStyle: Default, Arial\n
					CodecPrivate: "5b56342b205374796c65735d0a466f726d61743a204e616d652c20466f6e746e616d650a5374796c653a2044656661756c742c20417269616c0a",
				},
			},
		},
		Attachments: []matroska.EbmlAttachment{
			{ID: 1, FileName: "Arial.ttf", ContentType: "font/ttf"},
			{ID: 2, FileName: "UnusedFont.ttf", ContentType: "font/ttf"},
		},
	}

	fontMap := map[string]string{
		"arial": "Arial",
	}
	attachmentNames := map[int][]string{
		1: {"Arial"},
		2: {"UnusedFont"},
	}

	res := runTrackChecks("", ebml, fontMap, attachmentNames, nil)
	found := false

	for _, r := range res {
		if r.Identifier == "matroska_unused_fonts" {
			found = true

			if !strings.Contains(r.Warning, "UnusedFont.ttf") {
				t.Errorf("Expected warning to contain UnusedFont.ttf, got '%s'", r.Warning)
			}

			if strings.Contains(r.Warning, "Arial.ttf") {
				t.Errorf("Warning should not contain Arial.ttf, got '%s'", r.Warning)
			}
		}
	}

	if !found {
		t.Error("Did not find unused fonts check result")
	}
}

//nolint:paralleltest // depends on shared global state
func TestRunTrackChecksFontFilenameCompliance(t *testing.T) {
	config.InitDefaults()
	viper.Set("enabled_checks", []string{"all"})

	ebml := &matroska.EbmlMetadata{
		Tracks: []matroska.EbmlTrack{},
		Attachments: []matroska.EbmlAttachment{
			{ID: 1, FileName: "Arial.ttf", ContentType: "font/ttf"},
			{ID: 2, FileName: "WrongName.ttf", ContentType: "font/ttf"},
		},
	}

	attachmentNames := map[int][]string{
		1: {"Arial"},
		2: {"CorrectName"},
	}

	res := runTrackChecks("", ebml, nil, attachmentNames, nil)
	found := false

	for _, r := range res {
		if r.Identifier == "matroska_font_filename_compliance" {
			found = true

			if r.Severity != "info" {
				t.Errorf("Expected severity to be info, got '%s'", r.Severity)
			}

			if !strings.Contains(r.Warning, "WrongName.ttf") {
				t.Errorf("Expected warning to contain WrongName.ttf, got '%s'", r.Warning)
			}

			if strings.Contains(r.Warning, "Arial.ttf") {
				t.Errorf("Warning should not contain Arial.ttf, got '%s'", r.Warning)
			}
		}
	}

	if !found {
		t.Error("Did not find font filename compliance check result")
	}
}

func runHygieneTest(t *testing.T, name string, ebml *matroska.EbmlMetadata, meta *metadata.Metadata, identifier string, wantErr bool) {
	t.Helper()

	t.Run(name, func(t *testing.T) {
		res := runTrackChecks("", ebml, nil, nil, meta)
		found := false

		for _, r := range res {
			if r.Identifier == identifier {
				found = true

				if !r.Passed != wantErr {
					t.Errorf("check %s failed, got %v, want error %v", identifier, !r.Passed, wantErr)
				}
			}
		}

		if !found && wantErr {
			t.Errorf("check %s not found but wanted error", identifier)
		}
	})
}

//nolint:paralleltest // depends on shared global state
func TestRunTrackChecksContainerHygiene(t *testing.T) {
	config.InitDefaults()
	viper.Set("enabled_checks", []string{"matroska_title_hygiene", "matroska_app_hygiene"})

	tests := []struct {
		name       string
		ebml       *matroska.EbmlMetadata
		meta       *metadata.Metadata
		identifier string
		wantErr    bool
	}{
		{
			name: "Clean title",
			ebml: &matroska.EbmlMetadata{
				Container: matroska.EbmlContainer{Properties: matroska.EbmlContainerProperties{Title: "Frieren"}},
			},
			meta:       &metadata.Metadata{Title: "Frieren"},
			identifier: "matroska_title_hygiene",
			wantErr:    false,
		},
		{
			name: "Dirty title with technical info",
			ebml: &matroska.EbmlMetadata{
				Container: matroska.EbmlContainer{Properties: matroska.EbmlContainerProperties{Title: "Frieren [1080p]"}},
			},
			meta:       &metadata.Metadata{Title: "Frieren"},
			identifier: "matroska_title_hygiene",
			wantErr:    true,
		},
		{
			name: "Clean WritingApplication",
			ebml: &matroska.EbmlMetadata{
				Container: matroska.EbmlContainer{Properties: matroska.EbmlContainerProperties{WritingApplication: "mkvmerge v85.0"}},
			},
			identifier: "matroska_app_hygiene",
			wantErr:    false,
		},
		{
			name: "Dirty WritingApplication with path",
			ebml: &matroska.EbmlMetadata{
				Container: matroska.EbmlContainer{Properties: matroska.EbmlContainerProperties{WritingApplication: "mkvmerge /home/user/test.mkv"}},
			},
			identifier: "matroska_app_hygiene",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		runHygieneTest(t, tt.name, tt.ebml, tt.meta, tt.identifier, tt.wantErr)
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

func createMockCuesFile(tb testing.TB, cueTimes []uint64) string {
	tb.Helper()

	seekIDData := encodeTestElement(0x53AB, []byte{0x1C, 0x53, 0xBB, 0x6B})
	seekPosData := encodeTestElement(0x53AC, []byte{40})
	seekData := encodeTestElement(0x4DBB, append(seekIDData, seekPosData...))
	seekHeadData := encodeTestElement(0x114D9B74, seekData)

	padding := append([]byte{0xEC, 0x93}, make([]byte, 19)...)

	cueTrack := encodeTestElement(0xF7, []byte{1})
	cueTrackPos := encodeTestElement(0xB7, cueTrack)

	var cuesInner []byte

	for _, ct := range cueTimes {
		var ctBytes []byte
		if ct > 0xFF {
			ctBytes = []byte{byte(ct >> 8), byte(ct)}
		} else {
			ctBytes = []byte{byte(ct)}
		}

		cueTime := encodeTestElement(0xB3, ctBytes)
		cuePoint := encodeTestElement(0xBB, append(cueTime, cueTrackPos...))
		cuesInner = append(cuesInner, cuePoint...)
	}

	cuesData := encodeTestElement(0x1C53BB6B, cuesInner)

	segmentPayload := make([]byte, 0, len(seekHeadData)+len(padding)+len(cuesData))
	segmentPayload = append(segmentPayload, seekHeadData...)
	segmentPayload = append(segmentPayload, padding...)
	segmentPayload = append(segmentPayload, cuesData...)

	segmentData := encodeTestElement(0x18538067, segmentPayload)
	ebmlHeader := encodeTestElement(0x1A45DFA3, nil)

	fileData := make([]byte, 0, len(ebmlHeader)+len(segmentData))
	fileData = append(fileData, ebmlHeader...)
	fileData = append(fileData, segmentData...)

	tmpFile, err := os.CreateTemp("", "test-check-ebml-*.mkv")
	if err != nil {
		tb.Fatalf("failed to create temp file: %v", err)
	}

	defer func() {
		_ = tmpFile.Close()
	}()

	if _, err := tmpFile.Write(fileData); err != nil {
		tb.Fatalf("failed to write temp file: %v", err)
	}

	return tmpFile.Name()
}

//nolint:paralleltest // Test mutates global viper config and cannot run in parallel
func TestCheckChaptersKeyframeAlignmentAligned(t *testing.T) {
	config.InitDefaults()
	viper.Set("enabled_checks", []string{"all"})

	filePath := createMockCuesFile(t, []uint64{0, 10000, 20000})

	defer func() {
		_ = os.Remove(filePath)
	}()

	ebml := &matroska.EbmlMetadata{
		Tracks: []matroska.EbmlTrack{
			{ID: 1, Type: "video", Properties: matroska.EbmlTrackProperties{Number: 1}},
		},
		Chapters: []matroska.EbmlChapters{
			{
				Editions: []matroska.EbmlEdition{
					{
						Chapters: []matroska.EbmlChapterAtom{
							{TimeStart: 0},
							{TimeStart: 10000000000},
						},
					},
				},
			},
		},
	}

	res := runTrackChecks(filePath, ebml, nil, nil, nil)

	for _, r := range res {
		if r.Identifier == "matroska_chapters_keyframe_alignment" {
			if !r.Passed {
				t.Errorf("Expected alignment check to pass, got warning: %s", r.Warning)
			}
		}
	}
}

//nolint:paralleltest // Test mutates global viper config and cannot run in parallel
func TestCheckChaptersKeyframeAlignmentNonAligned(t *testing.T) {
	config.InitDefaults()
	viper.Set("enabled_checks", []string{"all"})

	filePath := createMockCuesFile(t, []uint64{0, 10000, 20000})

	defer func() {
		_ = os.Remove(filePath)
	}()

	ebml := &matroska.EbmlMetadata{
		Tracks: []matroska.EbmlTrack{
			{ID: 1, Type: "video", Properties: matroska.EbmlTrackProperties{Number: 1}},
		},
		Chapters: []matroska.EbmlChapters{
			{
				Editions: []matroska.EbmlEdition{
					{
						Chapters: []matroska.EbmlChapterAtom{
							{TimeStart: 0},
							{TimeStart: 15000000000},
						},
					},
				},
			},
		},
	}

	res := runTrackChecks(filePath, ebml, nil, nil, nil)
	found := false

	for _, r := range res {
		if r.Identifier == "matroska_chapters_keyframe_alignment" {
			found = true

			if r.Passed {
				t.Error("Expected alignment check to fail for non-aligned chapters")
			}

			if !strings.Contains(r.Actual, "off by 5.000s") {
				t.Errorf("Expected mismatch actual details, got: %q", r.Actual)
			}
		}
	}

	if !found {
		t.Error("Did not find keyframe alignment check result")
	}
}

//nolint:paralleltest // Test mutates global viper config and cannot run in parallel
func TestCheckChaptersKeyframeAlignmentAsymmetric(t *testing.T) {
	config.InitDefaults()
	viper.Set("enabled_checks", []string{"all"})

	tests := []struct {
		name       string
		timeStarts []int64
		wantPassed bool
	}{
		{
			name:       "aligned slightly after (5ms)",
			timeStarts: []int64{0, 10005000000},
			wantPassed: true,
		},
		{
			name:       "aligned slightly before within rounding tolerance (0.5ms)",
			timeStarts: []int64{0, 9999500000},
			wantPassed: true,
		},
		{
			name:       "non-aligned before rounding tolerance (2ms)",
			timeStarts: []int64{0, 9998000000},
			wantPassed: false,
		},
		{
			name:       "non-aligned after tolerance (15ms)",
			timeStarts: []int64{0, 10015000000},
			wantPassed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := createMockCuesFile(t, []uint64{0, 10000, 20000})

			defer func() {
				_ = os.Remove(filePath)
			}()

			res := runAsymmetricCheck(filePath, tt.timeStarts)
			foundResult := findAlignmentResult(res)

			if tt.wantPassed {
				if foundResult != nil {
					t.Errorf("Expected check to pass (not be present in issues), but got failure: %+v", foundResult)
				}
			} else {
				if foundResult == nil {
					t.Error("Expected check to fail, but found no issue result")
				} else if foundResult.Passed {
					t.Error("Expected check result to have Passed=false, but got Passed=true")
				}
			}
		})
	}
}

func runAsymmetricCheck(filePath string, timeStarts []int64) []CheckResult {
	ebml := &matroska.EbmlMetadata{
		Tracks: []matroska.EbmlTrack{
			{ID: 1, Type: "video", Properties: matroska.EbmlTrackProperties{Number: 1}},
		},
		Chapters: []matroska.EbmlChapters{
			{
				Editions: []matroska.EbmlEdition{
					{
						Chapters: []matroska.EbmlChapterAtom{
							{TimeStart: timeStarts[0]},
							{TimeStart: timeStarts[1]},
						},
					},
				},
			},
		},
	}

	return runTrackChecks(filePath, ebml, nil, nil, nil)
}

func findAlignmentResult(res []CheckResult) *CheckResult {
	for i := range res {
		if res[i].Identifier == "matroska_chapters_keyframe_alignment" {
			return &res[i]
		}
	}

	return nil
}

func encodeTestVINT(val uint64) []byte {
	if val < 0x80-1 {
		return []byte{byte(val | 0x80)}
	}

	if val < 0x4000-1 {
		return []byte{byte((val >> 8) | 0x40), byte(val)}
	}

	panic("too large for test VINT")
}

func encodeTestElement(id uint64, data []byte) []byte {
	idBytes := make([]byte, 0, 4)

	switch {
	case id > 0xFFFFFF:
		idBytes = append(idBytes, byte(id>>24), byte(id>>16), byte(id>>8), byte(id))
	case id > 0xFFFF:
		idBytes = append(idBytes, byte(id>>16), byte(id>>8), byte(id))
	case id > 0xFF:
		idBytes = append(idBytes, byte(id>>8), byte(id))
	default:
		idBytes = append(idBytes, byte(id))
	}

	sizeBytes := encodeTestVINT(uint64(len(data)))

	res := make([]byte, 0, len(idBytes)+len(sizeBytes)+len(data))
	res = append(res, idBytes...)
	res = append(res, sizeBytes...)
	res = append(res, data...)

	return res
}
