package checks

import (
	"testing"

	"github.com/spf13/viper"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/metadata/mediainfo"
)

//nolint:funlen,paralleltest // large table-driven test that modifies global config state, preventing parallelism
func TestRunTrackChecks(t *testing.T) {
	config.InitDefaults()
	viper.Set("disabled_checks", []string{
		config.CheckMatroskaSubtitleInlineFonts,
		config.CheckMatroskaAssEvents,
		config.CheckMatroskaSrtValidation,
		config.CheckMatroskaVideoCropping,
		config.CheckMatroskaTitleHygiene,
		config.CheckMatroskaAppHygiene,
		config.CheckMatroskaTrackDelay,
		config.CheckMatroskaCommentaryChannels,
		config.CheckMatroskaCommentaryBitrate,
		config.CheckMatroskaCommentaryPrefix,
		config.CheckMatroskaCommentaryPairing,
	})
	t.Logf("viper disabled_checks inside test = %v", viper.GetStringSlice("disabled_checks"))

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
			name: "Valid audio commentary ordering (strictly by notability, ignoring language)",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 1}},
				{ID: 2, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "eng", Default: true, Number: 2}},
				{ID: 3, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "eng", Commentary: true, Name: "Commentary by Director", Number: 3}},
				{ID: 4, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Commentary: true, Name: "Commentary by Actor", Number: 4}},
			},
			wantErr: false,
		},
		{
			name: "Invalid audio commentary ordering (Actor before Director)",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 1}},
				{ID: 2, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Commentary: true, Name: "Commentary by Actor", Number: 2}},
				{ID: 3, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Commentary: true, Name: "Commentary by Director", Number: 3}},
			},
			wantErr: true,
		},
		{
			name: "Valid subtitle commentary ordering (end group, grouped by language, then notability)",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 1}},
				{ID: 2, Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 2}},
				{ID: 3, Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "eng", Default: true, Number: 3}},
				{ID: 4, Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", Commentary: true, Name: "Commentary by Director", Number: 4}},
				{ID: 5, Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", Commentary: true, Name: "Commentary by Actor", Number: 5}},
				{ID: 6, Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "eng", Commentary: true, Name: "Commentary by Director", Number: 6}},
			},
			wantErr: false,
		},
		{
			name: "Invalid subtitle commentary ordering (Commentary before Standard)",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 1}},
				{ID: 2, Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", Commentary: true, Name: "Commentary by Director", Number: 2}},
				{ID: 3, Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 3}},
			},
			wantErr: true,
		},
		{
			name: "Valid subtitle commentary ordering (Standard Commentary before SDH Commentary)",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 1}},
				{ID: 2, Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 2}},
				{ID: 3, Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", Commentary: true, Name: "Commentary by Director", Number: 3}},
				{ID: 4, Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", Commentary: true, HearingImpaired: true, Name: "SDH / Commentary by Director", Number: 4}},
			},
			wantErr: false,
		},
		{
			name: "Invalid subtitle commentary ordering (SDH Commentary before Standard Commentary)",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 1}},
				{ID: 2, Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 2}},
				{ID: 3, Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", Commentary: true, HearingImpaired: true, Name: "SDH / Commentary by Director", Number: 3}},
				{ID: 4, Type: "subtitles", Properties: matroska.EbmlTrackProperties{Language: "ger", Commentary: true, Name: "Commentary by Director", Number: 4}},
			},
			wantErr: true,
		},
		{
			name: "Valid audio descriptive ordering (Standard Audio before Descriptive Audio)",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 1}},
				{ID: 2, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", VisualImpaired: true, Name: "AD", Number: 2}},
			},
			wantErr: false,
		},
		{
			name: "Invalid audio descriptive ordering (Descriptive Audio before Standard Audio)",
			tracks: []matroska.EbmlTrack{
				{ID: 1, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", VisualImpaired: true, Name: "AD", Number: 1}},
				{ID: 2, Type: "audio", Properties: matroska.EbmlTrackProperties{Language: "ger", Default: true, Number: 2}},
			},
			wantErr: true,
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
			var xmlChs *matroska.Chapters
			if len(tt.chapters) > 0 && len(tt.chapters[0].Editions) > 0 {
				xmlChs = &matroska.Chapters{Atoms: tt.chapters[0].Editions[0].Chapters}
			}

			res := runTrackChecks("", &matroska.EbmlMetadata{
				Tracks:    tt.tracks,
				Chapters:  tt.chapters,
				Container: tt.container,
			}, xmlChs, nil, nil)

			hasFailure := false

			for _, r := range res {
				if !r.Passed {
					hasFailure = true

					break
				}
			}

			if hasFailure != tt.wantErr {
				for _, r := range res {
					if !r.Passed {
						t.Errorf("  Failed check: %s: %s", r.Identifier, r.Warning)
					}
				}

				t.Errorf("runTrackChecks() hasFailure = %v, wantErr %v", hasFailure, tt.wantErr)
			}
		})
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
	viper.Set("enabled_checks", []string{config.CheckMatroskaTitleHygiene, config.CheckMatroskaAppHygiene})

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
			identifier: config.CheckMatroskaTitleHygiene,
			wantErr:    false,
		},
		{
			name: "Dirty title with technical info",
			ebml: &matroska.EbmlMetadata{
				Container: matroska.EbmlContainer{Properties: matroska.EbmlContainerProperties{Title: "Frieren [1080p]"}},
			},
			meta:       &metadata.Metadata{Title: "Frieren"},
			identifier: config.CheckMatroskaTitleHygiene,
			wantErr:    true,
		},
		{
			name: "Clean WritingApplication",
			ebml: &matroska.EbmlMetadata{
				Container: matroska.EbmlContainer{Properties: matroska.EbmlContainerProperties{WritingApplication: "mkvmerge v85.0"}},
			},
			identifier: config.CheckMatroskaAppHygiene,
			wantErr:    false,
		},
		{
			name: "Dirty WritingApplication with path",
			ebml: &matroska.EbmlMetadata{
				Container: matroska.EbmlContainer{Properties: matroska.EbmlContainerProperties{WritingApplication: "mkvmerge /home/user/test.mkv"}},
			},
			identifier: config.CheckMatroskaAppHygiene,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		runHygieneTest(t, tt.name, tt.ebml, tt.meta, tt.identifier, tt.wantErr)
	}
}

//nolint:funlen,paralleltest // table-driven test cases mutating config and stubs
func TestCheckCommentaryChannels(t *testing.T) {
	config.InitDefaults()

	tests := []struct {
		name     string
		tracks   []matroska.EbmlTrack
		expected bool // true if passed, false if failed
	}{
		{
			name: "stereo commentary passes",
			tracks: []matroska.EbmlTrack{
				{
					Type: "audio",
					Properties: matroska.EbmlTrackProperties{
						Number:        1,
						Commentary:    true,
						AudioChannels: 2,
					},
				},
			},
			expected: true,
		},
		{
			name: "mono commentary passes",
			tracks: []matroska.EbmlTrack{
				{
					Type: "audio",
					Properties: matroska.EbmlTrackProperties{
						Number:        1,
						Commentary:    true,
						AudioChannels: 1,
					},
				},
			},
			expected: true,
		},
		{
			name: "5.1 commentary fails",
			tracks: []matroska.EbmlTrack{
				{
					Type: "audio",
					Properties: matroska.EbmlTrackProperties{
						Number:        1,
						Commentary:    true,
						AudioChannels: 6,
					},
				},
			},
			expected: false,
		},
		{
			name: "5.1 non-commentary passes",
			tracks: []matroska.EbmlTrack{
				{
					Type: "audio",
					Properties: matroska.EbmlTrackProperties{
						Number:        1,
						Commentary:    false,
						AudioChannels: 6,
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := checkCommentaryChannels(tt.tracks)
			if tt.expected && res != nil {
				t.Errorf("Expected pass, got warning: %v", res.Warning)
			}

			if !tt.expected && res == nil {
				t.Error("Expected warning, got pass")
			}
		})
	}
}

//nolint:funlen,paralleltest // table-driven test cases mutating config and stubs
func TestCheckCommentaryBitrate(t *testing.T) {
	config.InitDefaults()

	// Stub getMediaInfo
	oldGetMediaInfo := getMediaInfo
	defer func() { getMediaInfo = oldGetMediaInfo }()

	var mockTracks []mediainfo.Track

	getMediaInfo = func(_ string) (*mediainfo.MediaInfo, error) {
		return &mediainfo.MediaInfo{
			Media: mediainfo.Media{
				Tracks: mockTracks,
			},
		}, nil
	}

	tests := []struct {
		name     string
		filePath string
		tracks   []matroska.EbmlTrack
		miTracks []mediainfo.Track
		expected bool // true if passed, false if failed
	}{
		{
			name:     "encode with low bitrate commentary passes",
			filePath: "Movie.2024.1080p.x264-GRP.mkv",
			tracks: []matroska.EbmlTrack{
				{
					Type: "audio",
					Properties: matroska.EbmlTrackProperties{
						Number:     1,
						Commentary: true,
					},
				},
			},
			miTracks: []mediainfo.Track{
				{
					ID:      "1",
					Type:    "Audio",
					Format:  "AAC",
					BitRate: 96000,
				},
			},
			expected: true,
		},
		{
			name:     "encode with high bitrate commentary fails",
			filePath: "Movie.2024.1080p.x264-GRP.mkv",
			tracks: []matroska.EbmlTrack{
				{
					Type: "audio",
					Properties: matroska.EbmlTrackProperties{
						Number:     1,
						Commentary: true,
					},
				},
			},
			miTracks: []mediainfo.Track{
				{
					ID:      "1",
					Type:    "Audio",
					Format:  "AAC",
					BitRate: 192000,
				},
			},
			expected: false,
		},
		{
			name:     "remux with high bitrate commentary passes",
			filePath: "Movie.2024.1080p.REMUX.mkv",
			tracks: []matroska.EbmlTrack{
				{
					Type: "audio",
					Properties: matroska.EbmlTrackProperties{
						Number:     1,
						Commentary: true,
					},
				},
			},
			miTracks: []mediainfo.Track{
				{
					ID:      "1",
					Type:    "Audio",
					Format:  "AC-3",
					BitRate: 192000,
				},
			},
			expected: true,
		},
		{
			name:     "lossless commentary on encode passes",
			filePath: "Movie.2024.1080p.x264-GRP.mkv",
			tracks: []matroska.EbmlTrack{
				{
					Type: "audio",
					Properties: matroska.EbmlTrackProperties{
						Number:     1,
						Commentary: true,
					},
				},
			},
			miTracks: []mediainfo.Track{
				{
					ID:            "1",
					Type:          "Audio",
					Format:        "FLAC",
					FormatProfile: "",
					BitRate:       320000,
				},
			},
			expected: true,
		},
		{
			name:     "non-commentary high bitrate passes",
			filePath: "Movie.2024.1080p.x264-GRP.mkv",
			tracks: []matroska.EbmlTrack{
				{
					Type: "audio",
					Properties: matroska.EbmlTrackProperties{
						Number:     1,
						Commentary: false,
					},
				},
			},
			miTracks: []mediainfo.Track{
				{
					ID:      "1",
					Type:    "Audio",
					Format:  "AAC",
					BitRate: 640000,
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTracks = tt.miTracks

			res := checkCommentaryBitrate(tt.filePath, tt.tracks)
			if tt.expected && res != nil {
				t.Errorf("Expected pass, got warning: %v", res.Warning)
			}

			if !tt.expected && res == nil {
				t.Error("Expected warning, got pass")
			}
		})
	}
}

//nolint:funlen,paralleltest // table-driven test cases mutating config and stubs
func TestCheckCommentaryPrefix(t *testing.T) {
	config.InitDefaults()

	tests := []struct {
		name     string
		tracks   []matroska.EbmlTrack
		expected bool // true if passed, false if failed
	}{
		{
			name: "standard commentary prefix passes",
			tracks: []matroska.EbmlTrack{
				{
					Type: "audio",
					Properties: matroska.EbmlTrackProperties{
						Number:     1,
						Commentary: true,
						Name:       "Commentary by director John Carpenter",
					},
				},
			},
			expected: true,
		},
		{
			name: "dialect prefixed commentary passes",
			tracks: []matroska.EbmlTrack{
				{
					Type: "audio",
					Properties: matroska.EbmlTrackProperties{
						Number:     1,
						Commentary: true,
						Name:       "French / Commentary by director John Carpenter",
					},
				},
			},
			expected: true,
		},
		{
			name: "multi-slash dialect and SDH prefixed commentary passes",
			tracks: []matroska.EbmlTrack{
				{
					Type: "subtitles",
					Properties: matroska.EbmlTrackProperties{
						Number:     1,
						Commentary: true,
						Name:       "English / SDH / Commentary by director John Carpenter",
					},
				},
			},
			expected: true,
		},
		{
			name: "isolated score prefix passes",
			tracks: []matroska.EbmlTrack{
				{
					Type: "audio",
					Properties: matroska.EbmlTrackProperties{
						Number:     1,
						Commentary: true,
						Name:       "Isolated score with commentary by composer Mark Isham",
					},
				},
			},
			expected: true,
		},
		{
			name: "The Hysteria Continues prefix passes",
			tracks: []matroska.EbmlTrack{
				{
					Type: "audio",
					Properties: matroska.EbmlTrackProperties{
						Number:     1,
						Commentary: true,
						Name:       "Commentary by The Hysteria Continues",
					},
				},
			},
			expected: true,
		},
		{
			name: "non-standard prefix fails",
			tracks: []matroska.EbmlTrack{
				{
					Type: "audio",
					Properties: matroska.EbmlTrackProperties{
						Number:     1,
						Commentary: true,
						Name:       "Audio commentary by John Carpenter",
					},
				},
			},
			expected: false,
		},
		{
			name: "empty name fails",
			tracks: []matroska.EbmlTrack{
				{
					Type: "audio",
					Properties: matroska.EbmlTrackProperties{
						Number:     1,
						Commentary: true,
						Name:       "",
					},
				},
			},
			expected: false,
		},
		{
			name: "non-commentary passes with any name",
			tracks: []matroska.EbmlTrack{
				{
					Type: "audio",
					Properties: matroska.EbmlTrackProperties{
						Number:     1,
						Commentary: false,
						Name:       "Audio commentary by John Carpenter",
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := checkCommentaryPrefix(tt.tracks)
			if tt.expected && res != nil {
				t.Errorf("Expected pass, got warning: %v", res.Warning)
			}

			if !tt.expected && res == nil {
				t.Error("Expected warning, got pass")
			}
		})
	}
}

//nolint:funlen,paralleltest // table-driven test cases mutating config and stubs
func TestCheckCommentaryPairing(t *testing.T) {
	config.InitDefaults()

	tests := []struct {
		name     string
		tracks   []matroska.EbmlTrack
		expected bool // true if passed, false if failed
	}{
		{
			name: "perfect pairing passes",
			tracks: []matroska.EbmlTrack{
				{
					Type: "audio",
					Properties: matroska.EbmlTrackProperties{
						Number:     1,
						Commentary: true,
						Language:   "eng",
						Name:       "Commentary by director John Carpenter",
					},
				},
				{
					Type: "subtitles",
					Properties: matroska.EbmlTrackProperties{
						Number:     2,
						Commentary: true,
						Language:   "eng",
						Name:       "Commentary by director John Carpenter",
					},
				},
			},
			expected: true,
		},
		{
			name: "pairing ignoring dialect and SDH passes",
			tracks: []matroska.EbmlTrack{
				{
					Type: "audio",
					Properties: matroska.EbmlTrackProperties{
						Number:     1,
						Commentary: true,
						Language:   "fre",
						Name:       "Commentary by director John Carpenter",
					},
				},
				{
					Type: "subtitles",
					Properties: matroska.EbmlTrackProperties{
						Number:     2,
						Commentary: true,
						Language:   "fre",
						Name:       "French / Commentary by director John Carpenter (SDH)",
					},
				},
			},
			expected: true,
		},
		{
			name: "pairing with multiple slashes and SDH prefix passes",
			tracks: []matroska.EbmlTrack{
				{
					Type: "audio",
					Properties: matroska.EbmlTrackProperties{
						Number:     1,
						Commentary: true,
						Language:   "eng",
						Name:       "English / Commentary by director John Carpenter",
					},
				},
				{
					Type: "subtitles",
					Properties: matroska.EbmlTrackProperties{
						Number:     2,
						Commentary: true,
						Language:   "eng",
						Name:       "English / SDH / Commentary by director John Carpenter",
					},
				},
			},
			expected: true,
		},
		{
			name: "missing audio commentary pairing fails",
			tracks: []matroska.EbmlTrack{
				{
					Type: "subtitles",
					Properties: matroska.EbmlTrackProperties{
						Number:     1,
						Commentary: true,
						Language:   "eng",
						Name:       "Commentary by director John Carpenter",
					},
				},
			},
			expected: false,
		},
		{
			name: "mismatched language passes if names match",
			tracks: []matroska.EbmlTrack{
				{
					Type: "audio",
					Properties: matroska.EbmlTrackProperties{
						Number:     1,
						Commentary: true,
						Language:   "eng",
						Name:       "Commentary by director John Carpenter",
					},
				},
				{
					Type: "subtitles",
					Properties: matroska.EbmlTrackProperties{
						Number:     2,
						Commentary: true,
						Language:   "fre",
						Name:       "Commentary by director John Carpenter",
					},
				},
			},
			expected: true,
		},
		{
			name: "non-commentary subtitle does not require pairing",
			tracks: []matroska.EbmlTrack{
				{
					Type: "subtitles",
					Properties: matroska.EbmlTrackProperties{
						Number:     1,
						Commentary: false,
						Language:   "eng",
						Name:       "English",
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := checkCommentaryPairing(tt.tracks)
			if tt.expected && res != nil {
				t.Errorf("Expected pass, got warning: %v", res.Warning)
			}

			if !tt.expected && res == nil {
				t.Error("Expected warning, got pass")
			}
		})
	}
}
