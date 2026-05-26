# Parsec Validation Checks

This document lists all individual checks performed by the `parsec check` command to ensure media files adhere to the project specification.

## Filename Checks

| Check | Function | Description |
|-------|----------|-------------|
| Allowed Characters | `filename.CheckAllowedCharacters` | Ensures filename only contains `[a-zA-Z0-9\-\.]`. |
| Disallowed Sequences | `filename.CheckCharacterSequences` | Detects invalid patterns like `.-.`, `..`, or multiple dots/dashes. |
| Parsing Consistency | `filename.Parse` | Verifies that all expected tags can be successfully extracted from the filename. |

## Metadata Consistency Checks

| Check | Function | Description |
|-------|----------|-------------|
| Filename vs MediaInfo | `match.Override` | Cross-references tags in the filename (resolution, codec, etc.) with technical data from MediaInfo. |

## Technical Quality Checks (MediaInfo)

| Check | Function | Description |
|-------|----------|-------------|
| Video Track Presence | `mediainfo.Get` | Verifies that the file contains at least one video track. |
| Interlaced WEB | `checks.RunMediaInfoChecks` | Issues a warning if a WEB source is detected as interlaced. |
| Frame Rate | `checks.CheckFrameRate` | Validates against standard framerates (23.976, 24, 25, 29.97, 30, 50, 59.94, 60). |
| Low Bitrate | `checks.CheckBitRate` | Checks for minimum bitrate thresholds based on resolution (e.g., 2 Mbps for 1080p). |
| Track Durations | `checks.checkDurations` | Detects significant timing discrepancies between video, audio, and subtitle tracks. |
| Redundant Audio | `checks.CheckRedundantAudio` | Identifies multiple standard audio tracks for the same language. |
| Resolution | `checks.CheckResolution` | Checks for mod-2 resolution, standard widths, and sane aspect ratios. |

## Matroska / EBML Checks

| Check | Function | Description |
|-------|----------|-------------|
| Matroska Format | `metadata.GetEbmlMetadata` | Verifies that the file is a valid Matroska (MKV) container. |
| Track Order | `metadata.VerifyTrackOrder` | Ensures audio and subtitle tracks are sorted by language priority and type (e.g., Forced before SDH). |
| Language Tags | `metadata.validateTrackBasics` | Verifies that all tracks have a valid ISO language tag. |
| 'mul' Track Name | `metadata.validateTrackBasics` | Ensures that tracks with language 'mul' (Multiple) have a descriptive Name field. |
| Original Language Consistency | `metadata.checkOriginalLanguageConsistency` | Verifies that the `OriginalLanguage` flag is applied consistently across all tracks of the same language. |
| Name Quality | `metadata.checkTrackNameQuality` | Detects "junk" keywords (e.g., "STEREO", "ENCODED") in track names. |
| Flag Keywords | `metadata.checkNameKeywords` | Ensures track names contain appropriate keywords (SDH, Forced, etc.) matching their flags. |
| Duplicate Tracks | `metadata.checkDuplicateTracks` | Identifies identical tracks (same language, flags, and name). |
| Default Flags | `metadata.CheckDefaultFlags` | Ensures only the first standard track per language is marked as Default. |
| Subtitle Format | `metadata.CheckSubtitleFormat` | Verifies that all subtitle tracks are in SRT format. |

## Generic Specification Checks

| Check | Function | Description |
|-------|----------|-------------|
| Missing Year | `checks.CheckYear` | Ensures movies have a release year in the filename. |
| Redundant Year | `checks.CheckYear` | Checks if a year tag is redundant (e.g., when using S20YY style naming). |
| Streaming Service | `checks.CheckStreaming` | Ensures a streaming service tag is present for WEB sources. |
| TV Specials | `checks.CheckTvSpecial` | Requires air date and episode title for TV specials (Season 00). |

## Media Database (MDB) Consistency Checks

| Check | Function | Description |
|-------|----------|-------------|
| Title Match | `checks.CheckTitle` | Compares filename title with the official title from TMDB/TVDB. |
| Movie Year Match | `checks.CheckMovieYear` | Verifies release year against database records for movies. |
| Series Year Match | `checks.CheckSeriesYear` | Verifies series start year against database records. |
| Episode Existence | `checks.CheckEpisode` | Verifies that the specific Season/Episode exists in the database. |
| Episode Title Match | `checks.CheckEpisodeTitle` | Compares filename episode title with the official database title. |
| Special Date Match | `checks.CheckSpecialDate` | Verifies air date for TV specials against database records. |
