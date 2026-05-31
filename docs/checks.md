# Parsec Validation Checks

This document lists all individual checks performed by the `parsec check` command to ensure media files adhere to the project specification.

Most of these can be disabled in the [Config](config.md) if you don't want to use them

## Filename Checks

| Check | Function | Identifier | Configurable | Description |
|-------|----------|------------|--------------|-------------|
| Allowed Characters | `checkAllowedCharacters` | `filename_characters` | Yes | Ensures filename only contains `[a-zA-Z0-9\-\.]`. |
| Disallowed Sequences | `checkCharacterSequences` | `filename_sequences` | Yes | Detects invalid patterns like `.-.`, `..`, or multiple dots/dashes. |
| Missing Year | `checkYearMissing` | `filename_year_missing` | Yes | Ensures movies have a release year in the filename. |
| Redundant Year | `checkYearRedundant` | `filename_year_redundant` | Yes | Checks if a year tag is redundant (e.g., when using S20YY style naming for episodes). |
| Streaming Service | `checkStreamingService` | `filename_streaming` | Yes | Ensures a streaming service tag is present for WEB sources. |
| TV Specials | `checkTvSpecial` | `filename_tv_special` | Yes | Requires air date and episode title for TV specials (Season 00). |
| Name Mismatch | `checkNameMismatch` | `filename_generation_mismatch` | Yes | Verifies that the filename matches the name generated from its metadata. |

## Technical Quality Checks (MediaInfo)

| Check | Function | Identifier | Configurable | Description |
|-------|----------|------------|--------------|-------------|
| Video Track Presence | `mediainfo.Get` | N/A | No | Verifies that the file contains at least one video track. Mandatory for processing. |
| Audio Track Presence | `mediainfo.Get` | N/A | No | Verifies that the file contains at least one audio track. Mandatory for processing. |
| Interlaced WEB | `checkInterlacedWeb` | `mediainfo_interlaced_web` | Yes | Issues a warning if a WEB source is detected as interlaced. |
| Frame Rate | `checkFrameRate` | `mediainfo_framerate` | Yes | Validates against standard framerates (23.976, 24, 25, 29.97, 30, 50, 59.94, 60). |
| Low Bitrate | `checkBitRate` | `mediainfo_bitrate` | Yes | Checks for minimum bitrate thresholds based on resolution (e.g., 2 Mbps for 1080p). |
| Track Durations | `checkDurations` | `mediainfo_durations` | Yes | Detects significant timing discrepancies between video, audio, and subtitle tracks. |
| Redundant Audio | `checkRedundantAudio` | `mediainfo_redundant_audio` | Yes | Identifies multiple standard audio tracks for the same language. |
| Resolution | `checkResolution` | `mediainfo_resolution` | Yes | Checks for odd resolution, standard widths, and sane aspect ratios. |

## Matroska / EBML Checks

| Check | Function | Identifier | Configurable | Description |
|-------|----------|------------|--------------|-------------|
| Matroska Format | `checkMatroskaFormat` | `matroska_ebml_error` | No | Verifies that the file is a valid Matroska (MKV) container. |
| Track Order | `checkTrackOrder` | `matroska_track_order` | Yes | Ensures audio and subtitle tracks are sorted by language priority (preferred_language, Original, English, and then alphabetical by ISO tag) and type (Forced, Standard/Default, SDH/Descriptive, Commentary). |
| Language Tags | `validateTrackBasics` | `matroska_language_tag` | Yes | Verifies that all tracks have a valid ISO language tag. |
| 'mul' Track Name | `validateTrackBasics` | `matroska_multi_lang` | Yes | Ensures that tracks with language 'mul' (Multiple) have a Name field listing at least two full language names. |
| Original Language Consistency | `checkOriginalLanguageConsistency` | `matroska_original_language` | Yes | Verifies that the `OriginalLanguage` flag is applied consistently. |
| Name Quality | `checkTrackNameQuality` | `matroska_name_quality` | Yes | Detects "junk" keywords (STEREO, ENCODED, SURROUND, etc.) in track names. |
| Simple Codecs | `checkTrackNameCodecs` | `matroska_name_codecs` | Yes | Detects simple codecs (AC3, AAC, DTS) in track names that are easily identified from technical metadata. |
| Redundant Language | `checkTrackNameRedundantLang` | `matroska_name_redundant_lang` | Yes | Flags full language names (e.g., "German") in the track title that match the track's language tag. |
| Flag Keywords | `checkNameKeywords` | `matroska_name_keywords` | Yes | Enforces strict two-way correlation between flags and keywords (SDH, Forced, Commentary, Descriptive/AD) in track names. |
| Duplicate Tracks | `checkDuplicateTracks` | `matroska_duplicate_tracks` | Yes | Identifies identical tracks (same language, flags, and name). |
| Default Flags | `checkDefaultFlags` | `matroska_default_flags` | Yes | Ensures only the first standard track per language is marked as Default. |
| Subtitle Format | `checkSubtitleFormat` | `matroska_subtitle_format` | Yes | Verifies that all subtitle tracks are in SRT format. |

## Media Database (MDB) Consistency Checks

| Check | Function | Identifier | Configurable | Description |
|-------|----------|------------|--------------|-------------|
| MDB Error | `checkMdbError` | `mdb_error` | No | Reports errors when communicating with TMDB/TVDB. |
| No Match Found | `checkNoMatch` | `mdb_no_match` | No | Issued when the media cannot be found in the database. |
| Title Match | `checkTitle` | `mdb_title` | Yes | Compares filename title with the official title from TMDB/TVDB. |
| Movie Year Match | `checkMovieYear` | `mdb_movie_year` | Yes | Verifies release year against database records for movies. |
| Series Year Match | `checkSeriesYear` | `mdb_series_year` | Yes | Verifies series start year against database records. |
| Unknown Original Language | `checkUnknownOriginalLang` | `mdb_unknown_original_lang` | Yes | Warns if the original language from TMDB/TVDB is not recognized or missing (could cause issues with other language checks). |
| Unwanted Audio Language | `checkUnwantedAudioLang` | `mdb_unwanted_audio_lang` | Yes | Flags audio tracks in languages that are neither the preferred nor the original language (often considered bloated) |
| Episode Existence | `checkEpisode` | `mdb_episode_existence` | Yes | Verifies that the Season/Episode exists in the database. |
| Episode Title Match | `checkEpisodeTitle` | `mdb_episode_title` | Yes | Compares filename episode title with the official database title. |
| Special Date Match | `checkSpecialDate` | `mdb_episode_date` | Yes | Verifies air date for TV specials against database records. |
| Track Languages | `checkTrackLanguages` | `mdb_track_languages` | Yes | Verifies presence of audio and subtitle tracks in both preferred and original languages. |
