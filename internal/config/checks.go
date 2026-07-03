package config

// Check identifier constants for the checks internal/correct's fix
// computations gate on via IsCheckEnabled. Sharing these (rather than each
// call site spelling out its own string literal) means a typo in one place
// can't silently desync a fix from the check it's supposed to mirror; it
// would simply fail to compile instead. This only covers identifiers
// referenced outside the checks package itself; validCheckIdentifiers below
// remains the exhaustive list for config validation.
const (
	CheckMatroskaAppHygiene                = "matroska_app_hygiene"
	CheckMatroskaChaptersKeyframeAlignment = "matroska_chapters_keyframe_alignment"
	CheckMatroskaCommentaryPairing         = "matroska_commentary_pairing"
	CheckMatroskaCommentaryPrefix          = "matroska_commentary_prefix"
	CheckMatroskaCreationTimePrivacy       = "matroska_creation_time_privacy"
	CheckMatroskaDefaultFlags              = "matroska_default_flags"
	CheckMatroskaFontFilenameCompliance    = "matroska_font_filename_compliance"
	CheckMatroskaLanguageTag               = "matroska_language_tag"
	CheckMatroskaMultiLang                 = "matroska_multi_lang"
	CheckMatroskaNameCodecs                = "matroska_name_codecs"
	CheckMatroskaNameKeywords              = "matroska_name_keywords"
	CheckMatroskaNameQuality               = "matroska_name_quality"
	CheckMatroskaNameRedundantLang         = "matroska_name_redundant_lang"
	CheckMatroskaOriginalLanguage          = "matroska_original_language"
	CheckMatroskaTitleHygiene              = "matroska_title_hygiene"
	CheckMatroskaTrackOrder                = "matroska_track_order"
	CheckMatroskaUnusedFonts               = "matroska_unused_fonts"
	CheckMatroskaZlibCompression           = "matroska_zlib_compression"
	CheckMdbUnwantedAudioLang              = "mdb_unwanted_audio_lang"
	CheckMediainfoEmptyTracks              = "mediainfo_empty_tracks"
	CheckMediainfoMissingStatistics        = "mediainfo_missing_statistics"
)
