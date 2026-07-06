package config

//nolint:revive // groups are for readability
const (
	// Filename checks
	CheckFilenameGenerationMismatch = "filename_generation_mismatch"
	CheckFilenameCharacters         = "filename_characters"
	CheckFilenameSequences          = "filename_sequences"
	CheckFilenameYearMissing        = "filename_year_missing"
	CheckFilenameYearRedundant      = "filename_year_redundant"
	CheckFilenameStreaming          = "filename_streaming"
	CheckFilenameTVSpecial          = "filename_tv_special"

	// MDB checks
	CheckMdbTitle             = "mdb_title"
	CheckMdbMovieYear         = "mdb_movie_year"
	CheckMdbSeriesYear        = "mdb_series_year"
	CheckMdbTrackLanguages    = "mdb_track_languages"
	CheckMdbUnknownOrigLang   = "mdb_unknown_original_lang"
	CheckMdbUnwantedAudioLang = "mdb_unwanted_audio_lang"
	CheckMdbEpisodeExistence  = "mdb_episode_existence"
	CheckMdbEpisodeTitle      = "mdb_episode_title"
	CheckMdbEpisodeDate       = "mdb_episode_date"

	// MediaInfo checks
	CheckMediainfoInterlacedWeb     = "mediainfo_interlaced_web"
	CheckMediainfoFramerate         = "mediainfo_framerate"
	CheckMediainfoBitrate           = "mediainfo_bitrate"
	CheckMediainfoDurations         = "mediainfo_durations"
	CheckMediainfoRedundantAudio    = "mediainfo_redundant_audio"
	CheckMediainfoResolution        = "mediainfo_resolution"
	CheckMediainfoDialogueNorm      = "mediainfo_dialogue_normalization"
	CheckMediainfoStereoLossless    = "mediainfo_stereo_lossless"
	CheckMediainfoEmptyTracks       = "mediainfo_empty_tracks"
	CheckMediainfoMissingStatistics = "mediainfo_missing_statistics"

	// Matroska: Track Basics
	CheckMatroskaLanguageTag      = "matroska_language_tag"
	CheckMatroskaMultiLang        = "matroska_multi_lang"
	CheckMatroskaOriginalLanguage = "matroska_original_language"
	CheckMatroskaDuplicateTracks  = "matroska_duplicate_tracks"
	CheckMatroskaDefaultFlags     = "matroska_default_flags"
	CheckMatroskaTrackOrder       = "matroska_track_order"
	CheckMatroskaTrackDelay       = "matroska_track_delay"
	CheckMatroskaVideoCropping    = "matroska_video_cropping"

	// Matroska: Track Naming
	CheckMatroskaNameQuality       = "matroska_name_quality"
	CheckMatroskaNameCodecs        = "matroska_name_codecs"
	CheckMatroskaNameRedundantLang = "matroska_name_redundant_lang"
	CheckMatroskaNameKeywords      = "matroska_name_keywords"

	// Matroska: Subtitles
	CheckMatroskaSubtitleFormat      = "matroska_subtitle_format"
	CheckMatroskaSubtitleFonts       = "matroska_subtitle_fonts"
	CheckMatroskaSubtitleInlineFonts = "matroska_subtitle_inline_fonts"
	CheckMatroskaSrtValidation       = "matroska_srt_validation"
	CheckMatroskaAssScriptInfo       = "matroska_ass_script_info"
	CheckMatroskaAssStyles           = "matroska_ass_styles"
	CheckMatroskaAssEvents           = "matroska_ass_events"
	CheckMatroskaZlibCompression     = "matroska_zlib_compression"

	// Matroska: Container & Attachments
	CheckMatroskaTitleHygiene           = "matroska_title_hygiene"
	CheckMatroskaAppHygiene             = "matroska_app_hygiene"
	CheckMatroskaCreationTimePrivacy    = "matroska_creation_time_privacy"
	CheckMatroskaTruehdCompatibility    = "matroska_truehd_compatibility"
	CheckMatroskaUnusedFonts            = "matroska_unused_fonts"
	CheckMatroskaFontFilenameCompliance = "matroska_font_filename_compliance"

	// Matroska: Commentary
	CheckMatroskaCommentaryChannels = "matroska_commentary_channels"
	CheckMatroskaCommentaryBitrate  = "matroska_commentary_bitrate"
	CheckMatroskaCommentaryPrefix   = "matroska_commentary_prefix"
	CheckMatroskaCommentaryPairing  = "matroska_commentary_pairing"

	// Matroska: Chapters
	CheckMatroskaChaptersStartNonZero      = "matroska_chapters_start_non_zero"
	CheckMatroskaChaptersNonMonotonic      = "matroska_chapters_non_monotonic"
	CheckMatroskaChaptersDuplicate         = "matroska_chapters_duplicate"
	CheckMatroskaChaptersTooClose          = "matroska_chapters_too_close"
	CheckMatroskaChaptersExceedDuration    = "matroska_chapters_exceed_duration"
	CheckMatroskaChaptersNameHygiene       = "matroska_chapters_name_hygiene"
	CheckMatroskaChaptersLanguageHygiene   = "matroska_chapters_language_hygiene"
	CheckMatroskaChaptersKeyframeAlignment = "matroska_chapters_keyframe_alignment"
)

// AllChecks contains every check identifier available in the system.
// This serves as the single source of truth for validation and iteration.
var AllChecks = []string{
	CheckFilenameGenerationMismatch,
	CheckFilenameCharacters,
	CheckFilenameSequences,
	CheckFilenameYearMissing,
	CheckFilenameYearRedundant,
	CheckFilenameStreaming,
	CheckFilenameTVSpecial,
	CheckMdbTitle,
	CheckMdbMovieYear,
	CheckMdbSeriesYear,
	CheckMdbTrackLanguages,
	CheckMdbUnknownOrigLang,
	CheckMdbUnwantedAudioLang,
	CheckMdbEpisodeExistence,
	CheckMdbEpisodeTitle,
	CheckMdbEpisodeDate,
	CheckMediainfoInterlacedWeb,
	CheckMediainfoFramerate,
	CheckMediainfoBitrate,
	CheckMediainfoDurations,
	CheckMediainfoRedundantAudio,
	CheckMediainfoResolution,
	CheckMediainfoDialogueNorm,
	CheckMediainfoStereoLossless,
	CheckMediainfoEmptyTracks,
	CheckMediainfoMissingStatistics,
	CheckMatroskaLanguageTag,
	CheckMatroskaMultiLang,
	CheckMatroskaNameQuality,
	CheckMatroskaNameCodecs,
	CheckMatroskaNameRedundantLang,
	CheckMatroskaOriginalLanguage,
	CheckMatroskaDuplicateTracks,
	CheckMatroskaNameKeywords,
	CheckMatroskaDefaultFlags,
	CheckMatroskaSubtitleFormat,
	CheckMatroskaSubtitleFonts,
	CheckMatroskaSubtitleInlineFonts,
	CheckMatroskaSrtValidation,
	CheckMatroskaAssScriptInfo,
	CheckMatroskaAssStyles,
	CheckMatroskaAssEvents,
	CheckMatroskaZlibCompression,
	CheckMatroskaTrackOrder,
	CheckMatroskaUnusedFonts,
	CheckMatroskaFontFilenameCompliance,
	CheckMatroskaTrackDelay,
	CheckMatroskaVideoCropping,
	CheckMatroskaTitleHygiene,
	CheckMatroskaAppHygiene,
	CheckMatroskaCreationTimePrivacy,
	CheckMatroskaTruehdCompatibility,
	CheckMatroskaCommentaryChannels,
	CheckMatroskaCommentaryBitrate,
	CheckMatroskaCommentaryPrefix,
	CheckMatroskaCommentaryPairing,
	CheckMatroskaChaptersStartNonZero,
	CheckMatroskaChaptersNonMonotonic,
	CheckMatroskaChaptersDuplicate,
	CheckMatroskaChaptersTooClose,
	CheckMatroskaChaptersExceedDuration,
	CheckMatroskaChaptersNameHygiene,
	CheckMatroskaChaptersLanguageHygiene,
	CheckMatroskaChaptersKeyframeAlignment,
}
