# Fix Command

The `fix` command automatically repairs Matroska issues reported by [`check`](checks.md) that can be resolved **without re-encoding**. It corrects track metadata in place and can optionally rewrite the container. Filename fixes are handled by the [`rename`](rename.md) command.

## Usage

```bash
parsec fix [path...] [flags]
```

You can provide one or more files or directories to be processed. Directories will be scanned recursively for Matroska (.mkv) files.

## How It Works

`fix` runs in up to two stages per file:

1. **In-place track fixes** (always): track flags and names are corrected directly with `mkvpropedit`. This is fast and lossless — the container is not rewritten.
2. **Container remux** (only with `--remux`): track order, container compression and track removals are applied by rewriting the file with `mkvmerge`. If non-preferred audio languages are present, `fix` uses `--ov` or looks up the MDB original language and only proposes unwanted-language removals when that information is available. The output replaces the original atomically and the original file mode is preserved.

Each check is only fixed if it is enabled in your [configuration](config.md); disabled checks are skipped, just as they are by `check`.

## What Gets Fixed

The mechanism column indicates how a fix is applied: **In-place** (`mkvpropedit`) or **Remux** (`mkvmerge`, requires `--remux`). Fixes marked *prompt* ask for confirmation or input.

### Matroska Track Metadata (in place)

| Check | Mechanism | Notes |
|-------|-----------|-------|
| `matroska_default_flags` | In-place | Sets the correct `default` flag per language. |
| `matroska_original_language` | In-place | Applies the `original` flag consistently. |
| `matroska_name_quality` | In-place | Removes junk keywords from track names. |
| `matroska_name_codecs` | In-place | Removes simple codec names (keeps `DTS-HD`, channel notations). |
| `matroska_name_redundant_lang` | In-place | Removes the redundant language word. |
| `matroska_name_keywords` | In-place / *prompt* | Appends a missing keyword automatically; asks before setting a flag when the name advertises one, and prompts for a `mul` track's Name when it lists fewer than two languages. |
| `matroska_language_tag` | In-place / *prompt* | Prompts for the language of `und` tracks. |
| `matroska_multi_lang` | In-place / *prompt* | Prompts for the name of `mul` tracks. |

### Container Metadata (in place)

| Check | Mechanism | Notes |
|-------|-----------|-------|
| `matroska_title_hygiene` | In-place / *prompt* | Clears the global title when it contains technical/release metadata noise (resolution, codec, bracketed/parenthesized tags). It is cleared rather than rewritten, since `fix` has no official title to fall back to. |
| `matroska_app_hygiene` | In-place / *prompt* | Clears the `WritingApplication` field when it leaks a local path or UUID. |
| `matroska_unused_fonts` | In-place / *prompt* | Deletes font attachments not referenced by any subtitle track's Styles or inline tags. Deletion is destructive, so it is always confirmed and skipped in `--unattended` mode, even though it doesn't require `--remux`. |
| `matroska_font_filename_compliance` | In-place / *prompt* | Renames a font attachment's filename to match its internal font name via `mkvpropedit --update-attachment`, which only touches the name and never re-uploads the font data. |

### Container Rewrite (requires `--remux`)

| Check | Mechanism | Notes |
|-------|-----------|-------|
| `matroska_track_order` | Remux | Reorders tracks by language and type priority. |
| `matroska_zlib_compression` | Remux | Strips zlib track compression. |
| `matroska_duplicate_tracks` | Remux / *prompt* | Removes exact-duplicate tracks (same language, flags and name). |
| `mdb_unwanted_audio_lang` | Remux / *prompt* | Removes audio in languages other than the preferred or MDB original language. Skipped if the original language is unavailable. Lists the affected languages before confirmation. |
| `mediainfo_empty_tracks` | Remux / *prompt* | Removes audio tracks reporting zero channels. Only the audio case is covered; a subtitle track with zero elements needs MediaInfo data `fix` does not yet read, so it is still reported by `check` only. |

### Not Fixed

Some issues cannot be fixed automatically and are left for manual resolution:

- **Filename and MDB naming issues** — `filename_generation_mismatch`, filename formatting checks, `mdb_title`, year and episode metadata checks. Use [`rename`](rename.md) for those.
- **Re-encoding required** — `mediainfo_framerate`, `mediainfo_bitrate`, `mediainfo_resolution`, `mediainfo_interlaced_web`, `mediainfo_durations`.
- **Bitstream metadata** — `mediainfo_dialogue_normalization` lives inside the audio stream, not the container.
- **Missing source data** — `mdb_audio_language_preferred`, `mdb_subtitle_language_preferred`, `mdb_audio_language_original`, `mdb_subtitle_language_original` (a track that is not present cannot be added), `mdb_episode_existence`, `mdb_error`, `mdb_no_match`, `mdb_unknown_original_lang`.
- **Same-language audio bloat** — `mediainfo_redundant_audio` is reported by `check` but **not** auto-removed. When one language has several audio tracks (e.g. a lossless track plus a lossy variant, or DTS-HD MA alongside DTS), `fix` keeps them all, because choosing which to drop needs codec/quality awareness that is not yet implemented. Only **unwanted-language** audio (anything other than the preferred or MDB original language) is pruned. Remove same-language duplicates manually for now.
- **Missing subtitle fonts** — `matroska_subtitle_fonts` and `matroska_subtitle_inline_fonts` are reported by `check` but not auto-fixed. Embedding the missing fonts means locating the actual font files for the family names an ASS/SSA track references (and honouring their embedding licences), which is out of scope for now. Attach them manually with `mkvpropedit --add-attachment`.
- **No safe target value** — `matroska_video_cropping` only detects an aspect-ratio mismatch; it doesn't compute the actual crop pixels needed, so there's nothing safe to write. `matroska_track_delay` flags a container-level timestamp offset that `mkvpropedit` cannot rewrite in place, and resetting it via remux risks turning a legitimate A/V offset into a real sync error.
- **Subtitle content issues** — `matroska_ass_script_info`, `matroska_ass_styles` and `matroska_ass_events` validate the ASS/SSA subtitle script itself, not container metadata, so fixing them is out of scope for `fix`.
- **Other** — `matroska_ebml_error` (broken file), `matroska_subtitle_format` (subtitle conversion), `filename_streaming` (use `rename --service`).

## Prompts

Fixes whose correct value cannot be derived from the file ask for input or confirmation: missing language tags, `mul` track names, keyword/flag mismatches, and every track or font attachment removal. These prompts are **skipped** in `--unattended` mode and when not running in a terminal, so unattended runs only apply the deterministic, non-destructive fixes. Track and font attachment removals are never performed without explicit confirmation.

## Flags

### ID Flags

| Flag | Type | Description |
|------|------|-------------|
| `--imdb` | string | IMDb ID used for the MDB original-language lookup. |
| `--tmdb` | int | TMDB ID used for the MDB original-language lookup. |
| `--tvdb` | int | TVDB ID used for the MDB original-language lookup. |

### Other Flags

| Flag | Shorthand | Type | Description |
|------|-----------|------|-------------|
| `--remux` | | boolean | Also apply fixes that require rewriting the container (track order, compression, track removal). |
| `--unattended`| `-u` | boolean | Do not prompt for confirmation; skips all interactive and destructive fixes. |
| `--dry-run` | `-d` | boolean | Preview the changes without modifying any files. |
| `--ov` | | string | Override the MDB original language/OV for unwanted-language audio removal. Accepts a 2- or 3-letter language code. |

## Examples

**Fix a single file with interactive confirmation:**
```bash
parsec fix Movie.2023.1080p.mkv
```

**Preview the changes without touching the file:**
```bash
parsec fix movie.mkv --dry-run
```

**Apply in-place fixes unattended (no remux, no prompts):**
```bash
parsec fix Series.S01E*.mkv -u
```

**Also rewrite the container to fix track order, compression and removals:**
```bash
parsec fix movie.mkv --remux
```

**Override the original language used for unwanted-language audio removal:**
```bash
parsec fix movie.mkv --remux --ov jpn
```
