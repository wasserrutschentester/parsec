# Autocorrect Command

The `autocorrect` command automatically repairs Matroska issues reported by [`check`](checks.md) that can be resolved **without re-encoding**. It corrects track metadata in place and can optionally rewrite the container. Filename corrections are handled by the [`rename`](rename.md) command.

## Usage

```bash
parsec autocorrect [path...] [flags]
```

You can provide one or more files or directories to be processed. Directories will be scanned recursively for Matroska (.mkv) files.

## How It Works

`autocorrect` computes a comprehensive **Correction Plan** for each file before applying any changes. This plan includes:

1. **In-place corrections**: Track flags, names, and simple container metadata are corrected directly. This is fast and lossless.
2. **Container remuxing**: Operations that require rebuilding the file (such as track reordering, compression removal, and pruning unwanted tracks) are computed automatically. The output replaces the original atomically and preserves the original file mode.

Each check is only corrected if it is enabled in your [configuration](config.md); disabled checks are skipped entirely.

After calculating the necessary changes, `autocorrect` presents a unified preview of the entire plan. You will be prompted once with `Apply corrections? [y,n,I]`. Pressing `i` or pressing Enter (since **inspect** is the default) opens an interactive selection menu where you can toggle specific correction categories or individual items before proceeding.

To see what would change without writing anything, use `--dry-run`. For deterministic non-destructive corrections without prompts, use `--unattended`.

## What Gets Corrected

Note that while most corrections are applied instantly in-place, operations that reorder or remove tracks require the file to be remuxed.

### Track Metadata

| Check | Notes |
|-------|-------|
| `matroska_default_flags` | Sets the correct `default` flag per language. |
| `matroska_original_language` | Applies the `original` flag consistently. |
| `matroska_name_quality` | Removes junk keywords from track names. |
| `matroska_name_codecs` | Removes simple codec names (keeps `DTS-HD`, channel notations). |
| `matroska_name_redundant_lang` | Removes the redundant language word. |
| `matroska_name_keywords` | Appends a missing keyword automatically; prompts for confirmation when the name advertises a flag, and asks for a `mul` track's Name when it lacks languages. |
| `matroska_commentary_prefix` | Adds the standard `Commentary by ...` prefix to non-empty commentary track names that lack it. |
| `matroska_commentary_pairing` | Renames commentary subtitle tracks to match the sole audio commentary when exactly one unambiguous audio commentary exists. |
| `matroska_language_tag` | Prompts for the language of `und` tracks. |
| `matroska_multi_lang` | Prompts for the name of `mul` tracks. |

### Container Properties

| Check | Notes |
|-------|-------|
| `matroska_title_hygiene` | Rewrites the global title with the clean parsed title (if available) or clears it if it contains noise. |
| `matroska_app_hygiene` | Clears the `WritingApplication` field when it leaks a local path or UUID. |
| `matroska_creation_time_privacy` | Clears the Segment `DateUTC`/`DateLocal` fields and removes creation/encode-time tag entries. Disabled by default. |
| `mediainfo_missing_statistics` | Recomputes and writes DURATION/NUMBER_OF_BYTES/element-count statistics tags for every track when missing. |

### Attachments & Chapters

| Check | Notes |
|-------|-------|
| `matroska_subtitle_fonts`, `matroska_subtitle_inline_fonts` | Attaches missing ASS/SSA fonts found locally or via Google Fonts (GitHub / Developer API). Remote downloads are skipped during `--dry-run` or `--unattended`. |
| `matroska_unused_fonts` | Deletes font attachments not referenced by any subtitle track. |
| `matroska_font_filename_compliance` | Renames font attachment filenames to match their internal font name, avoiding collisions with numbered suffixes. |
| `matroska_chapters_keyframe_alignment` | Snaps each misaligned chapter's start time to the nearest video keyframe. Changes seek points. |
| `matroska_chapters_language_hygiene` | Fills missing chapter display languages. Prompts if no unique language can be inferred. |

### Removals & Reordering (Requires Remux)

| Check | Notes |
|-------|-------|
| `matroska_track_order` | Reorders tracks by language and type priority. Out-of-place tracks are highlighted in the preview. |
| `matroska_zlib_compression` | Strips zlib track compression. |
| `mdb_unwanted_audio_lang` | Removes audio in languages other than the preferred or MDB original language. |
| `mediainfo_empty_tracks` | Removes audio tracks reporting zero channels. |

## Not Corrected

Some issues cannot be corrected automatically and are left for manual resolution. These generally fall into the following categories:

- **Requires Re-encoding:** Issues related to video framerates, resolutions, or audio codecs (e.g., TrueHD compatibility tracks, FLAC conversions) cannot be resolved by metadata edits.
- **Requires Manual Curation:** `autocorrect` will not auto-delete same-language duplicate tracks, calculate video cropping pixels, or correct structurally broken chapters (e.g., duplicated or non-monotonic timestamps).
- **Missing External Data:** Issues like `mdb_title` or missing track languages from external databases cannot be corrected automatically.
- **Subtitle Content:** Issues regarding the actual script/text of ASS or SRT subtitles are beyond the scope of container-level corrections.

## Prompts

At the main summary prompt, the default action is **inspect** (an empty Enter opens the interactive selection menu). Within the detailed preview and interactive menus, confirmations default to **no** (an empty Enter declines).

Corrections whose correct value cannot be derived from the file (like missing language tags) will prompt for free-text input. These input prompts, along with remote downloads, are safely skipped in `--unattended` mode. However, `--unattended` will automatically apply all other proposed corrections, including destructive operations like track and font removals.

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
| `--unattended`| `-u` | boolean | Do not prompt; automatically apply all proposed corrections (skipping only those requiring interactive input or remote downloads). |
| `--dry-run` | `-d` | boolean | Preview the changes without modifying any files. |
| `--original-language` | | string | Override the MDB original language for unwanted-language audio removal. Accepts a 2- or 3-letter language code. |

## Examples

**Correct a single file with interactive confirmation:**
```bash
parsec autocorrect Movie.2023.1080p.mkv
```

**Preview the changes without touching the file:**
```bash
parsec autocorrect movie.mkv --dry-run
```

**Apply all corrections unattended (no prompts):**
```bash
parsec autocorrect Series.S01E*.mkv -u
```

**Override the original language used for unwanted-language audio removal:**
```bash
parsec autocorrect movie.mkv --original-language jpn
```
