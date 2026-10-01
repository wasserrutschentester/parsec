# Rename Command

The `rename` command renames files according to metadata extracted from the file itself, technical data from MediaInfo, and information from media databases (TMDB/TVDB/IMDb).

## Usage

```bash
parsec rename [path...] [flags]
```

You can provide one or more files or directories to be processed. Directories will be scanned recursively for Matroska (.mkv) files.

## Features

-   **Batch Processing**: Rename multiple files or entire directories in one command.
-   **Recursive Scanning**: Automatically finds all Matroska files within provided directories.
-   **Filename Parsing**: Extracts metadata like Title, Year, Season, Episode, and existing P2P tags from the current filename.
-   **Technical Metadata**: Enriches metadata with technical data (resolution, codecs) from MediaInfo.
-   **External Metadata**: Fetches "official" titles and episode names from TMDB, TVDB, and IMDb.
-   **Interactive Preview**: Shows a comparison between the current and proposed filename before applying changes.
-   **Metadata Overrides**: Manually specify details using flags to correct misparsed information.
-   **Regex Replacements**: Define custom input, title, and output regex replacements to handle edge cases and enforce precise naming formats. See [Regex Replacements Documentation](replacements.md) for details.
-   **Normalization**: Automatically normalizes titles and service names according to common standards.

## Flags

### Metadata Flags

| Flag | Shorthand | Type | Description |
|------|-----------|------|-------------|
| `--title` | `-t` | string | Title of the movie or TV show. |
| `--original-title` | | string | Original foreign or alternative title. |
| `--year` | `-y` | integer | Release year. |
| `--season` | `-s` | integer | Season number. |
| `--episode` | `-e` | integer | Episode numbers (comma-separated). |
| `--date` | `-D` | string | Episode air date (YYYY-MM-DD). |
| `--episode-title`| | string | Title of the episode. |
| `--cut-edition` | | string | Override for the release edition (e.g., `Director's Cut`). |
| `--hdr` | | string | Override for HDR information (e.g., `HDR10`, `DV`). |

### P2P Info Flags

| Flag | Shorthand | Type | Description |
|------|-----------|------|-------------|
| `--service` | `-S` | string | Streaming service (e.g., `DSNP`, `NF`). |
| `--source` | `-o` | string | Source (e.g., `WEB-DL`, `BluRay`). |
| `--repack` | `-R` | integer (optional) | Mark the release as a repack (optional level, e.g. `-R` or `-R 2`). |
| `--remux` | | boolean | Identify release as a REMUX. |
| `--vcodec-style` | | string | Override video codec style (`web_dl`, `encode`, `remux`). |
| `--version-tag` | | integer | Quality upgrade version iteration (e.g. `2` for v2). |
| `--subbed` | | boolean | Add `.SUBBED` tag. |
| `--audio-description`| | boolean | Add audio description tag. |
| `--group` | `-g` | string | Release group name. |

### ID Flags

| Flag | Shorthand | Type | Description |
|------|-----------|------|-------------|
| `--tv` | `-T` | boolean | Identify as a TV show. |
| `--movie` | `-M` | boolean | Identify as a movie. |
| `--imdb` | | string | IMDb ID. |
| `--tmdb` | | integer | TMDB ID. |
| `--tvdb` | | integer | TVDB ID. |

### Other & Inspection Flags

| Flag | Shorthand | Type | Description |
|------|-----------|------|-------------|
| `--unattended`| `-u` | boolean | Do not prompt for confirmation before renaming. |
| `--dry-run` | `-d` | boolean | Print the proposed new filename but do not perform the actual rename. |
| `--season-pack`| `-P` | boolean | Move episodes into a correctly named season pack folder (omitting episode-specific info). |
| `--release-folder`| `-F` | boolean | Move each release into an identically named folder. |
| `--output` | `-O` | string | Output path where to move the files after renaming (absolute or relative to current working directory). |
| `--dump-context` | | boolean | Dump the template context data as JSON and exit without renaming. |
| `--dump-context-raw` | | boolean | Dump full context including raw MediaInfo and MDB search data as JSON. |

## Examples

**Rename a single file with interactive confirmation:**
```bash
parsec rename Movie.2023.1080p.mkv
```

**Rename multiple files in unattended mode:**
```bash
parsec rename Series.S01E*.mkv -u
```

**Override metadata during rename:**
```bash
parsec rename file.mkv --title "Better Title" --year 2022
```

**Use a specific database ID for identification:**
```bash
parsec rename movie.mkv --tmdb 12345
```
