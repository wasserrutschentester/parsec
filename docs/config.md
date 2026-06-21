# Configuration and Presets

Parsec uses a TOML-based configuration system. By default, it looks for a configuration file in the following locations:

1.  `$HOME/.config/parsec/config.toml` (Linux/macOS) or `%AppData%\parsec\config.toml` (Windows)
2.  `parsec.toml` in the current working directory or the config directory
3.  `.parsec.toml` in the current working directory

## Managing Configuration

You can use the `config` command to manage your configuration.

### Initialize Configuration

To create a default configuration file in the default location (`$HOME/.config/parsec/config.toml`), run:

```bash
parsec config init
```

### Validate Configuration

To verify your current configuration and check if API keys are set, run:

```bash
parsec config validate
```

## Presets

Presets allow you to define groups of settings that can be activated via the `--preset` flag. This is useful for recurring release types or specific shows.

### Defining Presets

Presets are defined under the `[preset.NAME]` section. Almost all configuration options (except API keys) can be used within a preset to override global settings.

```toml
# Global defaults
source = "WEB-DL"
group = "YourGroup"
video_codec_avc = "H.264"
video_codec_hevc = "H.265"

[api_keys]
tmdb = "your_tmdb_api_key"
tvdb = "your_tvdb_api_key"

[preset.marvel]
title = "Loki"
is_tv = true
tmdb_id = 84958

[preset.remux]
source = "BluRay"
video_codec_avc = "AVC"
video_codec_hevc = "HEVC"
template = "{title}.{year}.{resolution}.{source}.REMUX.{video_codec}.{audio_codec}{audio_channels}-{group}"
```

### Using Presets

Activate a preset using the `--preset` flag with any command:

```bash
parsec rename --preset marvel Episode01.mkv
```

When a preset is active, Parsec will first look for a value in the preset's configuration. If not found, it falls back to the global configuration, and finally to the project's internal defaults.

**Note:** Flags provided on the command line always take precedence over configuration and presets.

## Configuration Options

### Global-only Options

These options can only be set at the top level of the configuration file and are not affected by presets.

| Key | Type | Description |
|-----|------|-------------|
| `api_keys.tmdb` | string | API key for TMDB. |
| `api_keys.tvdb` | string | API key for TVDB. |

#### Prowlarr Settings

These settings are used for release searching via Prowlarr.

| Key | Type | Description |
|-----|------|-------------|
| `prowlarr.url` | string | URL of your Prowlarr instance (e.g., `http://localhost:9696`). |
| `prowlarr.api_key` | string | API key for Prowlarr. |
| `prowlarr.indexers` | array of integers | Array of indexer IDs to limit the search (empty for all). |
| `prowlarr.movie_categories` | array of integers | Prowlarr categories to search for movies (default: `[2000]`). |
| `prowlarr.tv_categories` | array of integers | Prowlarr categories to search for TV shows (default: `[5000]`). |

### Preset-aware Options

These options can be set globally OR within a `[preset.NAME]` block.

#### General Settings

| Key | Type | Description |
|-----|------|-------------|
| `template` | string | The naming template used for renaming and checking. See [Naming Templates](templates.md) for available tokens and formatting rules. |
| `preferred_language` | string | Preferred language code (default: `de`). |
| `subbed_tagging` | boolean | If there are subtitles but no audio for the preferred language (e.g. `de`) set language Info to GERMAN.SUBBED (default: `true`). |
| `disable_update_check` | boolean | Disable automatic update checks in the background and the warning notice if the version is outdated |
| `video_codec_avc` | string | Display name for AVC/H.264 (default: `H.264`). |
| `video_codec_hevc` | string | Display name for HEVC/H.265 (default: `H.265`). |
| `word_separator` | string | The character used to replace spaces in the generated filename, such as in the Title, Episode Title, and Audio Codec names. Use `" "` to preserve spaces (default: `.` ). |
| `allow_special_matches` | boolean | Allow fallback matching of episodes by Air Date or Episode Title against Specials (Season 0). Prevents incorrectly matching a regular episode without season/episode numbers to a TV special (default: `false`). |
| `normalize_diacritics` | boolean | Replace diacritics and special characters with their ASCII equivalents (e.g., ä -> ae, ß -> ss) (default: `true`). |
| `output_path` | string | Output path where files should be moved after renaming. Can be absolute or relative to the current working directory. |

#### Regex Replacements

Parsec supports powerful regex replacements for input filenames, title cleaning, and output filenames. See the [Regex Replacements Documentation](replacements.md) for detailed configuration instructions and examples.

#### Metadata Overrides

These are especially useful within presets to provide missing information or override parsed data.

| Key | Type | Description |
|-----|------|-------------|
| `title` | string | Title of the movie or TV show. |
| `year` | integer | Release year. |
| `season` | integer | Season number. |
| `episode` | integer | Episode number. |
| `date` | string | Episode air date (YYYY-MM-DD). |
| `episode_title`| string | Title of the episode. |
| `cut_edition` | string | Override for the release edition (e.g., `Director's Cut`). |
| `hdr` | string | Override for HDR information (e.g., `HDR10`, `DV`). |
| `service` | string | Streaming service (e.g., `DSNP`, `NF`). |
| `source` | string | Default source (e.g., `WEB-DL`, `BluRay`). |
| `group` | string | Default release group name. |
| `repack` | boolean | Set to `true` if the release is a repack. |
| `is_tv` | boolean | Force identification as a TV show. |
| `is_movie` | boolean | Force identification as a movie. |

#### Database IDs

Provide specific IDs to ensure the correct metadata is fetched from databases.

| Key | Type | Description |
|-----|------|-------------|
| `imdb_id` | string | IMDb ID (e.g., `tt1234567`). |
| `tmdb_id` | integer | TMDB ID. |
| `tvdb_id` | integer | TVDB ID. |

#### Check Control

You can enable or disable specific quality checks on a per-preset basis.
See [Validation Checks](checks.md) for more details about what checks are availible

| Key | Type | Description |
|-----|------|-------------|
| `enabled_checks` | array of strings | If set, only the listed checks will be performed. Use `["all"]` to enable all possible checks. |
| `disabled_checks` | array of strings | Listed checks will be skipped. (Ignored if `enabled_checks` is set) |

##### Available Checks

-   `filename_characters`: Check for disallowed characters in filename.
-   `filename_sequences`: Check for disallowed character sequences (e.g., `..`).
-   `filename_generation_mismatch`: Warn if the current filename does not match the generated name from template.
-   `mediainfo_interlaced_web`: Warn if WEB source is interlaced.
-   `mediainfo_framerate`: Check for non-standard framerates.
-   `mediainfo_bitrate`: Check for low bitrates.
-   `mediainfo_durations`: Check for inconsistent track durations.
-   `mediainfo_redundant_audio`: Check for redundant audio tracks.
-   `mediainfo_resolution`: Check for non-standard resolutions.
-   `mediainfo_dialogue_normalization`: Check for dialogue normalization in lossless audio tracks.
-   `mediainfo_stereo_lossless`: Warn if an audio track with 2 or less channels uses a different lossless codec than FLAC.
-   `matroska_track_order`: Verify track ordering rules.
-   `matroska_language_tag`: Verify valid ISO language tags on tracks.
-   `matroska_multi_lang`: Ensure 'mul' tracks have at least two full language names.
-   `matroska_original_language`: Verify consistent OriginalLanguage flag.
-   `matroska_name_quality`: Check for junk keywords (STEREO, ENCODED, etc.).
-   `matroska_name_codecs`: Detect simple codecs (AC3, AAC, DTS) in track names.
-   `matroska_name_redundant_lang`: Detect redundant language names (matching track tag).
-   `matroska_name_keywords`: Ensure names match flags (SDH, Forced, AD, etc.).
-   `matroska_duplicate_tracks`: Identify identical tracks.
-   `matroska_default_flags`: Ensure specialized tracks (Forced, SDH, etc.) are NOT default and standard tracks have correct default flags.
-   `matroska_subtitle_format`: Verify SRT or ASS requirement for text subtitles.
-   `matroska_subtitle_fonts`: Ensure all fonts used in SubStationAlpha subtitles are attached.
-   `matroska_subtitle_inline_fonts`: Verify fonts used in inline tags (slow).
-   `matroska_unused_fonts`: Identifies font attachments that are not used by any subtitle track.
-   `matroska_font_filename_compliance`: Verify that font attachment filenames match their internal names.
-   `matroska_ass_script_info`: Verify ASS Script Info headers.
-   `matroska_ass_styles`: Deep validation of ASS styles.
-   `matroska_ass_events`: Validation of ASS event lines.
-   `matroska_zlib_compression`: Detect tracks using zlib compression.
-   `matroska_track_delay`: Warn if a track has container delay exceeding ±1001ms (excluding TrueHD audio).
-   `matroska_video_cropping`: Warn if resolution-based black bars are detected but no MKV crop values are set.
-   `matroska_title_hygiene`: Verify container title doesn't contain technical metadata noise.
-   `matroska_app_hygiene`: Verify writing application metadata is clean of local paths/UUIDs.
-   `matroska_truehd_compatibility`: Verify Dolby TrueHD tracks are followed by a lossy compatibility track (AC3/E-AC3) in the same language.
-   `filename_year_missing`: Ensure movies have a year tag.
-   `filename_year_redundant`: Check for redundant year tags in series.
-   `filename_streaming`: Check for service tags on WEB sources.
-   `filename_tv_special`: Check required tags for TV Specials.
-   `mdb_title`: Verify title against TMDB/TVDB.
-   `mdb_movie_year`: Verify movie release year.
-   `mdb_series_year`: Verify series start year.
-   `mdb_unknown_original_lang`: Warn if the original language is missing or unrecognized by MDB.
-   `mdb_unwanted_audio_lang`: Flag audio tracks in languages other than preferred/original.
-   `mdb_episode_existence`: Check if episode exists in database.
-   `mdb_episode_title`: Verify episode title.
-   `mdb_episode_date`: Verify episode air date.
-   `mdb_track_languages`: Verify presence of preferred and original language tracks.

##### Examples

**Blacklisting (Disable specific checks globally)**

```toml
# Skip bitrate and resolution warnings by default
disabled_checks = ["mediainfo_bitrate", "mediainfo_resolution"]
```

**Whitelisting (Enable only specific checks in a preset)**

```toml
[preset.fast]
# For this preset, skip everything EXCEPT filename and track order
enabled_checks = ["filename_characters", "matroska_track_order"]
```

**Overriding global settings in a preset**

```toml
# Global setting
disabled_checks = ["mediainfo_bitrate"]

[preset.strict]
# Enable everything (including bitrate) for this preset
disabled_checks = []
```
