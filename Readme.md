# parsec

parse - check - create - release

`parsec` is a tool for managing media files, providing capabilities to identify, check, and rename files according to specific standards and metadata from online databases.

## Installation

**Parsec is currently in early development and may not be stable.**

To install `parsec`, ensure you have [Go](https://go.dev/) installed and run:

```bash
go install codeberg.org/n0ne/parsec@main
```

### Prerequisites

`parsec` depends on the following external tools which must be available in your `PATH`:

- **[MediaInfo](https://mediaarea.net/en/MediaInfo)**: Used for extracting technical metadata.
- **[MKVToolNix](https://mkvtoolnix.download/)**: Specifically `mkvmerge` and `mkvpropedit` for handling Matroska files.

## Commands

### `check`
Verify if a media file adheres to the specification.

**Features:**
- **Filename Validation**: Checks for allowed characters and correct sequence formatting.
- **Metadata Comparison**: Cross-references filename tags with technical data from MediaInfo.
- **Stream Analysis**: Performs quality checks on video and audio tracks.
- **Matroska Verification**: Validates track order, default flags, and subtitle formats.
- **MDB Integration**: Supports validation against TMDB, TVDB, and IMDb data.

A list of available checks can be found in the [checks documentation](docs/checks.md).

### `identify`
Search and identify movies or TV shows in media databases.

**Features:**
- **Flexible Search**: Fuzzy search by title/year or direct lookup via IMDb, TMDB, or TVDB IDs.
- **Automatic Parsing**: tries to extract Title, Year, Season, Episode from filenames to seed searches.
- **Matroska Tags**: Can write Title, IMDb ID, TMDB ID, and TVDB ID as Matroska tags in the file.

### `rename`
Rename files based on metadata and naming conventions.

**Features:**
- **Filename Parsing**: Extracts various metadata from filenames, including Title, Year, Season, Episode and existing P2P tags.
- **Technical Metadata**: Enriches metadata with technical data from MediaInfo.
- **External Metadata**: Fetches additional metadata from external sources such as TMDB, TVDB, and IMDb.
- **Filename Generation**: Generates new filenames based on the metadata and naming conventions.

## Configuration

By default, `parsec` looks for a configuration file at `$HOME/.config/parsec/config.toml` or `.parsec.toml` in the current directory.

### Presets

Presets allow you to override global configuration values for specific use cases, such as different naming conventions for different release groups or sources.

Example `config.toml`:

```toml
template = "{title}.{year}.{resolution}.{source}.{audio_codec}{audio_channels}.{video_codec}-{group}"
group = "DefaultGroup"
source = "WEB-DL"

[presets.remux]
source = "BluRay"
video_codec_avc = "AVC"
video_codec_hevc = "H265"
template = "{title}.{year}.{resolution}.{source}.REMUX.{video_codec}.{audio_codec}{audio_channels}.{group}"

[presets.webrip]
source = "WEBRip"
group = "OtherGroup"
```

You can apply a preset using the `--preset` flag:

```bash
parsec rename --preset remux movie.mkv
```
