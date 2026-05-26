# Configuration and Presets

Parsec uses a TOML-based configuration system. By default, it looks for a configuration file in the following locations:

1.  `$HOME/.config/parsec/config.toml` (Linux/macOS) or `%AppData%\parsec\config.toml` (Windows)
2.  `config.toml` in the current working directory
3.  `.parsec.toml` in the current working directory

## Presets

Presets allow you to define groups of settings that can be activated via the `--preset` flag. This is useful for recurring release types or specific shows.

### Defining Presets

Presets are defined under the `[preset.NAME]` section. Almost all configuration options (except API keys) can be used within a preset to override global settings.

```toml
# Global defaults
source = "WEB-DL"
group = "4Rocket"

[api_keys]
tmdb = "your_tmdb_api_key"
tvdb = "your_tvdb_api_key"

[preset.marvel]
title = "Loki"
is_tv = true
tmdb_id = 84958

[preset.bluray]
source = "BluRay"
template = "{title}.{year}.{resolution}.{source}.{audio_codec}{audio_channels}.{video_codec}-{group}"
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

### Preset-aware Options

These options can be set globally OR within a `[preset.NAME]` block.

#### General Settings

| Key | Type | Description |
|-----|------|-------------|
| `template` | string | The naming template used for renaming and checking. |
| `preferred_language` | string | Preferred language code (default: `de`). |
| `source` | string | Default source (e.g., `WEB-DL`, `BluRay`). |
| `group` | string | Default release group name. |
| `video_codec_avc` | string | Display name for AVC/H.264 (default: `H.264`). |
| `video_codec_hevc` | string | Display name for HEVC/H.265 (default: `H.265`). |

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
| `service` | string | Streaming service (e.g., `DSNP`, `NF`). |
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

| Key | Type | Description |
|-----|------|-------------|
| `enabled_checks` | array of strings | If set, only the listed checks will be performed. |
| `disabled_checks` | array of strings | Listed checks will be skipped. (Ignored if `enabled_checks` is set) |

##### Available Checks

-   `filename_characters`: Check for disallowed characters in filename.
-   `filename_sequences`: Check for disallowed character sequences (e.g., `..`).
-   `mediainfo_interlaced_web`: Warn if WEB source is interlaced.
-   `mediainfo_framerate`: Check for non-standard framerates.
-   `mediainfo_bitrate`: Check for low bitrates.
-   `mediainfo_durations`: Check for inconsistent track durations.
-   `mediainfo_redundant_audio`: Check for redundant audio tracks.
-   `mediainfo_resolution`: Check for non-standard resolutions.
-   `matroska_track_order`: Verify track ordering rules.
-   `matroska_language_tag`: Verify valid ISO language tags on tracks.
-   `matroska_multi_lang`: Ensure 'mul' tracks have a descriptive name.
-   `matroska_original_language`: Verify consistent OriginalLanguage flag.
-   `matroska_name_quality`: Check for junk keywords in track names.
-   `matroska_name_keywords`: Ensure names match flags (SDH, Forced, etc.).
-   `matroska_duplicate_tracks`: Identify identical tracks.
-   `matroska_default_flags`: Verify first-standard-track default rules.
-   `matroska_subtitle_format`: Verify SRT-only requirement.
-   `generic_year_missing`: Ensure movies have a year tag.
-   `generic_year_redundant`: Check for redundant year tags in series.
-   `generic_streaming`: Check for service tags on WEB sources.
-   `generic_tv_special`: Check required tags for TV Specials.
-   `mdb_title`: Verify title against TMDB/TVDB.
-   `mdb_movie_year`: Verify movie release year.
-   `mdb_series_year`: Verify series start year.
-   `mdb_episode_existence`: Check if episode exists in database.
-   `mdb_episode_title`: Verify episode title.
-   `mdb_episode_date`: Verify episode air date.

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

