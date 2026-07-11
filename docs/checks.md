# Parsec Validation Checks

The `check` command performs comprehensive integrity and consistency checks on a media file to ensure it adheres to the project specification.

## Usage

```bash
parsec check [path...] [flags]
```

You can provide one or more files or directories to be checked. Directories will be scanned recursively for Matroska (.mkv) files.

### Rendering Saved Reports

The `check` command can also be used to view previously saved JSON reports in the interactive human-readable format. `parsec` automatically detects if a file is a JSON report by sniffing its content.

```bash
# Save a report to a file
parsec check movie.mkv --json > report.json

# View the saved report later in interactive mode
parsec check report.json
```

## Features

- **Batch Processing**: Validate multiple files or entire directories in one command.
- **Recursive Scanning**: Automatically finds all Matroska files within provided directories.
- **Filename Integrity**: Verifies that the filename matches the internal metadata.
- **Technical Analysis**: Checks for technical anomalies using MediaInfo.
- **Matroska Verification**: Ensures the container and track tagging meet standards.
- **MDB Consistency**: Validates that file metadata matches information in online databases.
- **JSON Reports**: Machine-readable output for integration with other tools.

## Flags

### ID Flags
Force identification using specific database IDs.

| Flag | Shorthand | Type | Description |
|------|-----------|------|-------------|
| `--imdb` | | string | IMDb ID (e.g., `tt1234567`). |
| `--tmdb` | | integer | TMDB ID. |
| `--tvdb` | | integer | TVDB ID. |

### Other Flags

| Flag | Shorthand | Type | Description |
|------|-----------|------|-------------|
| `--json` | `-j` | boolean | Output check results in JSON format. |
| `--unattended`| `-u` | boolean | Do not prompt for confirmation before displaying issue details. |
| `--verbose` | | boolean | Enable verbose output. |

## Available Checks

This document lists all individual checks performed by the `parsec check` command. Most of these can be disabled in the [Config](config.md) if you don't want to use them.

### Filename Checks

| Check | Function | Identifier | Configurable | Description |
|-------|----------|------------|--------------|-------------|
| Allowed Characters | `checkAllowedCharacters` | `filename_characters` | Yes | Ensures filename only contains `[a-zA-Z0-9\-\.]`. |
| Disallowed Sequences | `checkCharacterSequences` | `filename_sequences` | Yes | Detects invalid patterns like `.-.`, `..`, or multiple dots/dashes. |
| Missing Year | `checkYearMissing` | `filename_year_missing` | Yes | Ensures movies have a release year in the filename. |
| Redundant Year | `checkYearRedundant` | `filename_year_redundant` | Yes | Checks if a year tag is redundant (e.g., when using S20YY style naming for episodes). |
| Streaming Service | `checkStreamingService` | `filename_streaming` | Yes | Ensures a streaming service tag is present for WEB sources. |
| TV Specials | `checkTvSpecial` | `filename_tv_special` | Yes | Requires air date and episode title for TV specials (Season 00). |
| Name Mismatch | `checkNameMismatch` | `filename_generation_mismatch` | Yes | Verifies that the filename matches the name generated from its metadata. |

### Technical Quality Checks (MediaInfo)

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
| Dialogue Normalization | `checkDialogueNormalization` | `mediainfo_dialogue_normalization` | Yes | Verifies that dialogue normalization is removed for lossless (TrueHD, DTS-HD MA) and DTS-HD HRA tracks. |
| Stereo/Mono Lossless Codec | `checkStereoLossless` | `mediainfo_stereo_lossless` | Yes | Warns if an audio track with 2 or less channels uses a different lossless codec than FLAC (e.g., TrueHD, DTS-HD MA, or PCM). |
| Empty Tracks | `checkEmptyTracks` | `mediainfo_empty_tracks` | Yes | Issues an error if a track is determined to be empty (e.g., duration is 0, size is 0 bytes, audio has 0 channels, or subtitles have 0 elements). |
| Missing Statistics | `checkMissingStatistics` | `mediainfo_missing_statistics` | Yes | Warns if a track is missing statistics tags (e.g., DURATION, NUMBER_OF_BYTES, or ElementCount), making it difficult to determine if it is empty. |

### Matroska / EBML Checks

These checks ensure the Matroska container and its components meet quality standards.

#### Container & Audio

| Check | Function | Identifier | Configurable | Description |
|-------|----------|------------|--------------|-------------|
| Matroska Format | `checkMatroskaFormat` | `matroska_ebml_error` | No | Verifies that the file is a valid Matroska (MKV) container. |
| Title Hygiene | `checkTitleHygiene` | `matroska_title_hygiene` | Yes | Verifies that the global container title is either empty or matches the official database title, and doesn't contain technical metadata noise. |
| Metadata Privacy | `checkAppHygiene` | `matroska_app_hygiene` | Yes | Verifies that the `WritingApplication` field doesn't contain potentially identifiable information like local file paths or UUIDs. |
| Matroska Format | `checkDataLayout` | `matroska_data_layout` | yes | Verifies that the Info and Tracks Elemnts are placed before the first Cluster |
| Creation Time Privacy | `checkCreationTimePrivacy` | `matroska_creation_time_privacy` | Yes | Warns if the file contains creation/encode time metadata such as `DateUTC`, `DateLocal`, or track-level encoded dates, which might be a privacy concern. Disabled by default. |
| Video Cropping | `checkVideoCropping` | `matroska_video_cropping` | Yes | Warns if resolution-based black bars are detected but no MKV crop values are set. |
| Track Delay | `checkTrackDelay` | `matroska_track_delay` | Yes | Warns if a track has a container delay exceeding ±1001ms (excluding TrueHD audio). |
| TrueHD Compatibility | `checkTrueHDCompatibility` | `matroska_truehd_compatibility` | Yes | Verifies that any Dolby TrueHD audio track is followed by a lossy compatibility track (AC3/E-AC3) of the same language. |
| Commentary Channels | `checkCommentaryChannels` | `matroska_commentary_channels` | Yes | Warns if a commentary audio track has more than 2 channels. |
| Commentary Bitrate | `checkCommentaryBitrate` | `matroska_commentary_bitrate` | Yes | Warns if a commentary audio track has a bitrate exceeding 128 kbps (except for lossless codecs). |
| Commentary Prefix | `checkCommentaryPrefix` | `matroska_commentary_prefix` | Yes | Verifies commentary track names start with a standard prefix like "Commentary by...". |
| Commentary Pairing | `checkCommentaryPairing` | `matroska_commentary_pairing` | Yes | Verifies that subtitle commentary tracks have a corresponding audio commentary track. |

#### Tracks

| Check | Function | Identifier | Configurable | Description |
|-------|----------|------------|--------------|-------------|
| Language Tags | `validateTrackBasics` | `matroska_language_tag` | Yes | Verifies that all tracks have a valid ISO language tag. |
| 'mul' Track Name | `validateTrackBasics` | `matroska_multi_lang` | Yes | Ensures that tracks with language 'mul' (Multiple) have a Name field listing at least two full language names. |
| Name Quality | `checkTrackNameQuality` | `matroska_name_quality` | Yes | Detects "junk" keywords (STEREO, ENCODED, SURROUND, etc.) in track names. |
| Simple Codecs | `checkTrackNameCodecs` | `matroska_name_codecs` | Yes | Detects simple codecs (AC3, AAC, DTS) in track names that are easily identified from technical metadata. |
| Redundant Language | `checkTrackNameRedundantLang` | `matroska_name_redundant_lang` | Yes | Flags full language names (e.g., "German") in the track title that match the track's language tag. |
| Original Language Consistency | `checkOriginalLanguageConsistency` | `matroska_original_language` | Yes | Verifies that the `OriginalLanguage` flag is applied consistently. |
| Duplicate Tracks | `checkDuplicateTracks` | `matroska_duplicate_tracks` | Yes | Identifies identical tracks (same language, flags, and name). |
| Flag Keywords | `checkNameKeywords` | `matroska_name_keywords` | Yes | Enforces strict two-way correlation between flags and keywords (SDH, Forced, Commentary, Descriptive/AD) in track names. |
| Default Flags | `checkDefaultFlags` | `matroska_default_flags` | Yes | Ensures specialized tracks (Forced, SDH, Commentary, etc.) are NOT marked as Default, and that the first standard track per language IS marked as Default. |
| Track Order | `checkTrackOrder` | `matroska_track_order` | Yes | Ensures audio and subtitle tracks are sorted by language priority (preferred_language, Original, and then alphabetical by English language and dialect name ) and type (Forced, Standard/Default, SDH/Descriptive, Commentary). |

#### Subtitles

| Check | Function | Identifier | Configurable | Description |
|-------|----------|------------|--------------|-------------|
| Subtitle Format | `checkSubtitleFormat` | `matroska_subtitle_format` | Yes | Verifies that all text subtitle tracks are in SRT or ASS format. All other text formats should be converted to SRT. |
| Subtitle Fonts | `checkSubtitleFonts` | `matroska_subtitle_fonts` | Yes | Verifies that all fonts used in SubStationAlpha (SSA/ASS) subtitle track *Styles* are included as attachments. Matching is done using internal font names (via `sfnt`), making it independent of attachment filenames. |
| Subtitle Inline Fonts | `checkSubtitleInlineFonts` | `matroska_subtitle_inline_fonts` | Yes | Verifies fonts used in *inline tags* within SSA/ASS subtitle tracks. Uses internal font names for matching. Requires demuxing the track, which makes this check significantly slower. Disabled by default. |
| SRT Validation | `checkSRTValidation` | `matroska_srt_validation` | Yes | Verifies that SRT subtitle tracks contain valid HTML formatting tags, all tags are properly nested and closed, and throws a warning if positioning/alignment information is present (which should use ASS instead). Disabled by default. |
| Unused Fonts | `checkUnusedFonts` | `matroska_unused_fonts` | Yes | Identifies font attachments that are not used by any subtitle track. Uses internal font names to ensure accuracy. |
| Font Filename Compliance | `checkFontFilenameCompliance` | `matroska_font_filename_compliance` | Yes | Verifies that the filename of a font attachment matches its internal font name (Family or Full Name). |
| ASS Script Info | `checkASSScriptInfo` | `matroska_ass_script_info` | Yes | Verifies that the `[Script Info]` section of an ASS subtitle track contains recommended headers like `ScaledBorderAndShadow` and `YCbCr Matrix`. |
| ASS Style Validation | `checkASSStyles` | `matroska_ass_styles` | Yes | Performs deep validation of ASS `[V4+ Styles]`, checking for valid font sizes, alignments, encodings, and avoiding trailing whitespace in style names. |
| ASS Event Validation | `checkASSEvents` | `matroska_ass_events` | Yes | Validates ASS `[Events]`, ensuring all used styles are defined, time formats are correct, and forbidden tags (like `\fe`) are avoided. |
| Zlib Compression | `checkZlibCompression` | `matroska_zlib_compression` | Yes | Verifies that zlib compression is disabled for all tracks. |

#### Chapters

| Check | Function | Identifier | Configurable | Description |
|-------|----------|------------|--------------|-------------|
| Chapter Non-Zero Start | `checkChaptersStartNonZero` | `matroska_chapters_start_non_zero` | Yes | Verifies that the first chapter starts at exactly `00:00:00.000`. |
| Chapter Non-Monotonic Order | `checkChaptersNonMonotonic` | `matroska_chapters_non_monotonic` | Yes | Verifies that chapter start times are strictly increasing. |
| Chapter Duplicate Timestamps | `checkChaptersDuplicate` | `matroska_chapters_duplicate` | Yes | Flags cases where multiple chapters share the exact same timestamp. |
| Chapter Interval Too Short | `checkChaptersTooClose` | `matroska_chapters_too_close` | Yes | Flags consecutive chapters that are less than 10 seconds apart. |
| Chapter Exceeds Duration | `checkChaptersExceedDuration` | `matroska_chapters_exceed_duration` | Yes | Ensures no chapter starts after the total duration of the video. |
| Chapter Name Hygiene | `checkChaptersNameHygiene` | `matroska_chapters_name_hygiene` | Yes | Verifies chapter display names are present, and have no consecutive duplicate names. |
| Chapter Language Hygiene | `checkChaptersLanguageHygiene` | `matroska_chapters_language_hygiene` | Yes | Ensures all chapter displays have valid, consistent language tags (and are not undetermined/missing). |
| Chapter Keyframe Alignment | `checkChaptersKeyframeAlignment` | `matroska_chapters_keyframe_alignment` | Yes | Verifies that chapter timestamps fall exactly on video keyframes (seek points) using the container's Cues index. |

### Media Database (MDB) Consistency Checks

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

## Aggregate Checks

Aggregate checks are performed after all individual files have been processed. They evaluate consistency and completeness across the entire set of provided files.

| Check | Function | Identifier | Description |
|-------|----------|------------|-------------|
| Season Completeness | `RunSeasonCompletenessCheck` | `mdb_season_completeness` | Compares the set of local episodes against the official episode list on TVDB/TMDB for a given season. |

### Season Completeness Check

This check identifies missing episodes in a season pack. It is designed to be helpful but non-intrusive:

- **Activation**: Only runs when more than one episode of the same season is provided in a single command.
- **Verification**: Cross-references local episode numbers with the official list from TVDB (preferred) or TMDB (fallback).
- **Season 0**: Automatically skips Season 0 (Specials), as these are often inconsistent across databases.
- **Output**: Provides a summary of whether the season is complete or lists the specific missing episode numbers.

Note: This check is currently only displayed in the interactive terminal output and is not included in JSON reports.

## JSON Output

When the `--json` flag is used, `parsec check` always outputs detailed reports as a JSON **array** of objects, even when processing a single file.
This ensures consistent machine-readable output.

Passed checks and empty fields are generally omitted to reduce noise.

The interactive output contains the same information as the JSON output, but in an easy-to-read human-readable format.

### Top-Level Structure

| Field | Type | Description |
|-------|------|-------------|
| `file` | string | Full path or filename of the checked file. |
| `passed` | boolean | `true` if all checks passed, `false` otherwise. |
| `filename` | string | The original filename without extension. |
| `generated_name` | string | The expected filename generated based on metadata and naming conventions. |
| `version` | string | The version of parsec used to generate the report. |
| `issues` | array | A list of issue groups, categorized by source. |

**Example:**
```json
{
  "file": "Die.Kaenguru.Chroniken.2020.German.AC3.1080p.BluRay.x265-FuN.mkv",
  "passed": false,
  "filename": "Die.Kaenguru.Chroniken.2020.German.AC3.1080p.BluRay.x265-FuN",
  "generated_name": "Die.Kaenguru.Chroniken.2020.GERMAN.1080p.BluRay.DD5.1.H.265-FuN",
  "version": "v0.3.0",
  "issues": []
}
```

### Issue Group

| Field | Type | Description |
|-------|------|-------------|
| `category` | string | The category of issues. Possible values: `FILENAME`, `MDB`, `MEDIAINFO`, `MATROSKA`. |
| `results` | array | A list of individual check results for this category. |

**Example:**
```json
{
  "category": "FILENAME",
  "results": []
}
```

### Result Object

| Field | Type | Description |
|-------|------|-------------|
| `identifier` | string | Unique identifier for the check (see [Available Checks](#available-checks)). |
| `passed` | boolean | Whether this specific check passed. |
| `severity` | string | The importance of the issue. Possible values: `info`, `warning`, `error`. |
| `warning` | string | Human-readable description of the problem. |
| `expected` | string | The expected value (optional, depends on the check). |
| `actual` | string | The actual value found (optional, depends on the check). |
| `list` | array | A list of string bullet points (optional, used for multi-line formatting). |
| `tracks` | array | List of track-specific results (optional, for checks that evaluate individual tracks). |
| `table` | object | Table-formatted data (optional, see [TableData Object](#tabledata-object)). |

**Example:**
```json
{
  "identifier": "filename_generation_mismatch",
  "passed": false,
  "severity": "warning",
  "warning": "Generated name does not match the original",
  "expected": "Die.Kaenguru.Chroniken.2020.German.AC3.1080p.BluRay.x265-FuN",
  "actual": "Die.Kaenguru.Chroniken.2020.GERMAN.1080p.BluRay.DD5.1.x265-FuN",
  "tracks": []
}
```

### Track Object

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | The MediaInfo track ID (Number, starting from 1). |
| `type` | string | The track type (e.g., `video`, `audio`, `subtitle`). |
| `passed` | boolean | Whether the check passed for this specific track. |
| `type_order` | integer | The sequential order of this track among tracks of the same type. |
| `codec` | string | Technical codec name (e.g., `V_MPEG4/ISO/AVC`, `A_AC3`). |
| `name` | string | The track's `Name` field in Matroska. |
| `language` | string | The track's ISO 639-2/T language tag (e.g., `ger`, `eng`). |
| `flags` | array | List of applied track flags. See [Track Flags](#track-flags). |
| `warning` | string | Human-readable description of the track-specific issue. |
| `list` | array | A list of string bullet points (optional, used for multi-line formatting). |
| `table` | object | Table-formatted data for this track (optional, see [TableData Object](#tabledata-object)). |

**Example:**
```json
{
  "id": "2",
  "type": "audio",
  "passed": false,
  "type_order": 1,
  "codec": "AC-3",
  "name": "Surround",
  "language": "ger",
  "flags": [
    "Default"
  ],
  "warning": "junk keyword 'SURROUND' in Name"
}
```

### Track Flags

Possible values in the `flags` array:

* `Default`: Track is marked as the default for its type.
* `Forced`: Track is marked as forced.
* `Hearing Impaired`: Track is marked for hearing impaired (SDH).
* `Visual Impaired`: Track is marked for visual impaired (Descriptive Audio).
* `Commentary`: Track is marked as commentary.
* `Original`: Track is marked as being in the original language.

### TableData Object

Used to display structured, multi-dimensional tabular data.

| Field | Type | Description |
|-------|------|-------------|
| `headers` | array | List of column header names (strings). |
| `rows` | array | List of row entries, where each entry is a list of strings corresponding to the headers. |

**Example:**
```json
{
  "headers": [
    "Line #",
    "Style Name",
    "Validation Issue"
  ],
  "rows": [
    [
      "18",
      "Default",
      "Encoding should be 1, got 0"
    ]
  ]
}
```

### Full Example
```bash
parsec check Die.Kaenguru.Chroniken.2020.German.AC3.1080p.BluRay.x265-FuN.mkv --json
```
```json
[
  {
    "file": "Die.Kaenguru.Chroniken.2020.German.AC3.1080p.BluRay.x265-FuN.mkv",
    "passed": false,
    "filename": "Die.Kaenguru.Chroniken.2020.German.AC3.1080p.BluRay.x265-FuN",
    "generated_name": "Die.Kaenguru.Chroniken.2020.GERMAN.1080p.BluRay.DD5.1.x265-FuN",
    "version": "v0.3.0",
    "issues": [
      {
        "category": "FILENAME",
        "results": [
          {
            "identifier": "filename_generation_mismatch",
            "passed": false,
            "severity": "warning",
            "warning": "Generated name does not match the original",
            "expected": "Die.Kaenguru.Chroniken.2020.German.AC3.1080p.BluRay.x265-FuN",
            "actual": "Die.Kaenguru.Chroniken.2020.GERMAN.1080p.BluRay.DD5.1.x265-FuN"
          }
        ]
      },
      {
        "category": "MDB",
        "results": [
          {
            "identifier": "mdb_subtitle_language_preferred",
            "passed": false,
            "severity": "warning",
            "warning": "Subtitle track in preferred language 'de' is missing",
            "expected": "de"
          }
        ]
      },
      {
        "category": "MATROSKA",
        "results": [
          {
            "identifier": "matroska_name_quality",
            "passed": false,
            "severity": "info",
            "warning": "Track Name contains junk keywords",
            "tracks": [
              {
                "id": "2",
                "type": "audio",
                "passed": false,
                "type_order": 1,
                "codec": "AC-3",
                "name": "Surround",
                "language": "ger",
                "flags": [
                  "Default"
                ],
                "warning": "junk keyword 'SURROUND' in Name"
              }
            ]
          }
        ]
      }
    ]
  }
]
```
