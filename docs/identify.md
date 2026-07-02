# Identify Command

The `identify` command allows you to search for and identify movies or TV shows in media databases (TMDB, TVDB, IMDb). It can also write identified metadata back to Matroska files as tags.

## Usage

```bash
parsec identify [path...] [flags]
```

If filenames or directories are provided, `parsec` will process each one. Directories will be scanned recursively for Matroska (.mkv) files. Each file will be automatically parsed to extract initial metadata like title, year, season, and episode to seed the search.

## Features

- **Batch Processing**: Identify multiple files or entire directories in one command.
- **Recursive Scanning**: Automatically finds all Matroska files within provided directories.
- **Flexible Search**: Fuzzy search by title/year or direct lookup via IMDb, TMDB, or TVDB IDs.
- **Automatic Parsing**: Tries to extract Title, Year, Season, Episode from filenames to seed searches.
- **Matroska Tags**: Writes Title, IMDb ID, TMDB ID, TVDB ID, and Episode/Movie Title as Matroska tags.
- **Episode Search**: Finds episode details via Season/Episode numbers, Episode Title, or Air Date.
- **Release Search**: Search for existing releases on your indexers via Prowlarr.
- **ID Verification**: If a file already contains metadata tags, `parsec` will warn you if the selected search result differs from the existing tags.

## Flags

### Metadata Flags
These flags override or provide information that might be missing from the filename.

| Flag | Shorthand | Type | Description |
|------|-----------|------|-------------|
| `--title` | `-t` | string | Title of the movie or TV show. |
| `--year` | `-y` | integer | Release year. |
| `--season` | `-s` | integer | Season number. |
| `--episode` | `-e` | integer | Episode number. |
| `--date` | `-D` | string | Episode air date (YYYY-MM-DD). |
| `--episode-title`| | string | Title of the episode. |

### ID Flags
Force identification using specific database IDs.

| Flag | Shorthand | Type | Description |
|------|-----------|------|-------------|
| `--tv` | `-T` | boolean | Identify as a TV show. |
| `--movie` | `-M` | boolean | Identify as a movie. |
| `--imdb` | | string | IMDb ID (e.g., `tt1234567`). |
| `--tmdb` | | integer | TMDB ID. |
| `--tvdb` | | integer | TVDB ID. |

### P2P Info Flags
Additional metadata that can be written to tags.

| Flag | Shorthand | Type | Description |
|------|-----------|------|-------------|
| `--service` | `-S` | string | Streaming service (e.g., `DSNP`, `NF`). |
| `--source` | `-O` | string | Source (e.g., `WEB-DL`, `BluRay`). |
| `--group` | `-g` | string | Release group name. |

### Other Flags

| Flag | Shorthand | Type | Description |
|------|-----------|------|-------------|
| `--write-tags` | | boolean| Write metadata tags to the file. |
| `--comment` | | string| A comment exposed to tag templates (e.g. repack reason). |
| `--unattended` | `-u` | boolean| Run in unattended mode (selects the first search result if it's a high-confidence match). |
| `--releases` | `-r` | boolean | Search for releases via Prowlarr for the identified entity. |
| `--best-release`| `-b` | boolean | Only show the best release (same resolution and most seeders) when searching for releases. |

## Examples

**Identify a file and write tags interactively:**
```bash
parsec identify "Loki.S01E01.mkv"
```

**Identify multiple files in unattended mode:**
```bash
parsec identify Series.S01E*.mkv -u
```

**Search for releases for a specific movie:**
```bash
parsec identify --imdb tt0111161 --releases
```

**Identify a specific episode by date:**
```bash
parsec identify "The.Daily.Show.mkv" --date 2023-10-24
```
