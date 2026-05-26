# Parsec Validation Checks

This document lists all individual checks performed by the `parsec check` command to ensure media files adhere to the project specification.

## Filename Checks

| Check | Function | Identifier | Description |
|-------|----------|------------|-------------|
| Allowed Characters | `filename.CheckAllowedCharacters` | `filename_characters` | Ensures filename only contains `[a-zA-Z0-9\-\.]`. |
| Disallowed Sequences | `filename.CheckCharacterSequences` | `filename_sequences` | Detects invalid patterns like `.-.`, `..`, or multiple dots/dashes. |
| Parsing Consistency | `filename.Parse` | *Core* | Verifies that all expected tags can be successfully extracted from the filename. |

## Metadata Consistency Checks

| Check | Function | Identifier | Description |
|-------|----------|------------|-------------|
| Filename vs MediaInfo | `match.Override` | *Core* | Cross-references tags in the filename (resolution, codec, etc.) with technical data from MediaInfo. |

## Technical Quality Checks (MediaInfo)

| Check | Function | Identifier | Description |
|-------|----------|------------|-------------|
| Video Track Presence | `mediainfo.Get` | *Core* | Verifies that the file contains at least one video track. |
| Interlaced WEB | `checks.RunMediaInfoChecks` | `mediainfo_interlaced_web` | Issues a warning if a WEB source is detected as interlaced. |
| Frame Rate | `checks.CheckFrameRate` | `mediainfo_framerate` | Validates against standard framerates (23.976, 24, 25, 29.97, 30, 50, 59.94, 60). |
| Low Bitrate | `checks.CheckBitRate` | `mediainfo_bitrate` | Checks for minimum bitrate thresholds based on resolution (e.g., 2 Mbps for 1080p). |
| Track Durations | `checks.checkDurations` | `mediainfo_durations` | Detects significant timing discrepancies between video, audio, and subtitle tracks. |
| Redundant Audio | `checks.CheckRedundantAudio` | `mediainfo_redundant_audio` | Identifies multiple standard audio tracks for the same language. |
| Resolution | `checks.CheckResolution` | `mediainfo_resolution` | Checks for mod-2 resolution, standard widths, and sane aspect ratios. |

## Matroska / EBML Checks

| Check | Function | Identifier | Description |
|-------|----------|------------|-------------|
| Matroska Format | `metadata.GetEbmlMetadata` | *Core* | Verifies that the file is a valid Matroska (MKV) container. |
| Track Order | `metadata.VerifyTrackOrder` | `matroska_track_order` | Ensures audio and subtitle tracks are sorted by language priority and type. |
| Language Tags | `metadata.validateTrackBasics` | `matroska_language_tag` | Verifies that all tracks have a valid ISO language tag. |
| 'mul' Track Name | `metadata.validateTrackBasics` | `matroska_multi_lang` | Ensures that tracks with language 'mul' (Multiple) have a descriptive Name field. |
| Original Language Consistency | `metadata.checkOriginalLanguageConsistency` | `matroska_original_language` | Verifies that the `OriginalLanguage` flag is applied consistently. |
| Name Quality | `metadata.checkTrackNameQuality` | `matroska_name_quality` | Detects "junk" keywords (e.g., "STEREO", "ENCODED") in track names. |
| Flag Keywords | `metadata.checkNameKeywords` | `matroska_name_keywords` | Ensures track names contain appropriate keywords matching their flags. |
| Duplicate Tracks | `metadata.checkDuplicateTracks` | `matroska_duplicate_tracks` | Identifies identical tracks (same language, flags, and name). |
| Default Flags | `metadata.CheckDefaultFlags` | `matroska_default_flags` | Ensures only the first standard track per language is marked as Default. |
| Subtitle Format | `metadata.CheckSubtitleFormat` | `matroska_subtitle_format` | Verifies that all subtitle tracks are in SRT format. |

## Generic Specification Checks

| Check | Function | Identifier | Description |
|-------|----------|------------|-------------|
| Missing Year | `checks.CheckYear` | `generic_year_missing` | Ensures movies have a release year in the filename. |
| Redundant Year | `checks.CheckYear` | `generic_year_redundant` | Checks if a year tag is redundant (e.g., when using S20YY style naming). |
| Streaming Service | `checks.CheckStreaming` | `generic_streaming` | Ensures a streaming service tag is present for WEB sources. |
| TV Specials | `checks.CheckTvSpecial` | `generic_tv_special` | Requires air date and episode title for TV specials (Season 00). |

## Media Database (MDB) Consistency Checks

| Check | Function | Identifier | Description |
|-------|----------|------------|-------------|
| Title Match | `checks.CheckTitle` | `mdb_title` | Compares filename title with the official title from TMDB/TVDB. |
| Movie Year Match | `checks.CheckMovieYear` | `mdb_movie_year` | Verifies release year against database records for movies. |
| Series Year Match | `checks.CheckSeriesYear` | `mdb_series_year` | Verifies series start year against database records. |
| Episode Existence | `checks.CheckEpisode` | `mdb_episode_existence` | Verifies that the Season/Episode exists in the database. |
| Episode Title Match | `checks.CheckEpisodeTitle` | `mdb_episode_title` | Compares filename episode title with the official database title. |
| Special Date Match | `checks.CheckSpecialDate` | `mdb_episode_date` | Verifies air date for TV specials against database records. |


