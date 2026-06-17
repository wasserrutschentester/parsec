# parsec

<div align="center">
  <img src="docs/assets/header.webp" alt="Parsec header" width="600px"/>
  </br>
  <h2 id="tagline">parse - check - create - release</h2>
  </br>
</div>

`parsec` is a tool for managing media files, providing capabilities to identify, check, and rename files according to specific standards and metadata from online databases.

## Installation

**Parsec is currently in beta. The initial rapid development phase is over, but it isn't well tested yet. If you encounter any issues, please report them on the [issue tracker](https://codeberg.org/upPollo/parsec/issues).**

To install `parsec`, ensure you can just download the latest release from the [releases page](https://codeberg.org/upPollo/parsec/releases) and extract the binary to your `PATH` (and make sure it's set as executable)

for new versions you can use the `update` command to go to the latest version:

```bash
parsec update
```

### Dependencies

`parsec` depends on the following external tools which must be available in your `PATH`:

- **[MediaInfo](https://mediaarea.net/en/MediaInfo)**: Used for extracting technical metadata.
- **[MKVToolNix](https://mkvtoolnix.download/)**: Specifically `mkvmerge` and `mkvpropedit` for handling Matroska files.

### Manual Installation

Clone the repository and build with make:

```bash
git clone https://codeberg.org/upPollo/parsec.git
cd parsec
make build
```

this should create a `parsec` binary in the current directory.

Build Dependencies:
- **[Go](https://golang.org/)**: Go compiler and standard library.
- **[Git](https://git-scm.com/)**: Version control system.
- **[make](https://www.gnu.org/software/make/)**: Build automation tool.

## Commands

### `check`
Verify if a media file adheres to the specification.

**Features:**
- **Filename Validation**: Checks for allowed characters and correct sequence formatting.
- **Metadata Comparison**: Cross-references filename tags with technical data from MediaInfo.
- **Stream Analysis**: Performs quality checks on video and audio tracks (framerate, bitrate, resolution).
- **Matroska Verification**: Validates track order, default flags, and subtitle formats.
- **MDB Integration**: Supports validation against TMDB, TVDB, and IMDb data.
- **Interactive Output**: View the failing checks per group, not all at once.
- **Report Rendering**: Load and view previously saved JSON reports in interactive mode.
- **JSON Output**: Export check results as JSON using the `--json` flag.

For more information see the [Checks Documentation](docs/checks.md)

### `identify`
Search and identify movies or TV shows in media databases.

**Features:**
- **Flexible Search**: Fuzzy search by title/year or direct lookup via IMDb, TMDB, or TVDB IDs.
- **Automatic Parsing**: Tries to extract Title, Year, Season, Episode from filenames to seed searches.
- **Matroska Tags**: Can write Title, IMDb ID, TMDB ID, TVDB ID, and Episode/Movie Title as Matroska tags.
- **Episode Search**: Can find Episode details via Episode + Season Number, Episode Title or Aired Date.
- **Release Search**: Search for existing releases on your indexers via Prowlarr.

For more information see the [Identify Documentation](docs/identify.md)

### `rename`
Rename files based on metadata and naming conventions.

**Features:**
- **Batch Processing**: Rename multiple files at once.
- **Filename Parsing**: Extracts various metadata from filenames, including Title, Year, Season, Episode and existing P2P tags.
- **Technical Metadata**: Enriches metadata with technical data from MediaInfo.
- **External Metadata**: Fetches additional metadata from external sources such as TMDB, TVDB, and IMDb.
- **Interactive**: Preview changes and then confirm the modification. Or apply unattended
- **Metadata Overrides**: Manually specify details like `--hdr`, `--cut-edition`, or `--repack`.

For more information see the [Rename Documentation](docs/rename.md)

### `fix`
Automatically repair the issues reported by `check` that can be resolved without re-encoding.

**Features:**
- **In-Place Track Fixes**: Corrects Matroska track flags and names directly, without rewriting the file.
- **Guided Fixes**: Prompts for values that can't be derived automatically (missing language tags, `mul` names, flag mismatches).
- **Container Remux**: With `--remux`, also fixes track order, compression, and removes duplicate, redundant or unwanted-language tracks when enough MDB data is available.
- **Safe by Default**: Destructive and interactive fixes are skipped in unattended mode; track removals always require confirmation.

For more information see the [Fix Documentation](docs/fix.md)

### `update`
Update `parsec` to the latest version.

For more information see the [Update Documentation](docs/update.md)

### `completion`
Generate autocompletion scripts for various shells (bash, zsh, fish, powershell).

For more information see the [Shell Completion Documentation](docs/completion.md)

## Configuration

`parsec` uses a TOML-based configuration system. By default, it looks for a configuration file at `$HOME/.config/parsec/config.toml` or `config.toml` in the current directory.

For a full list of available options and detailed information on the preset system, see the [Configuration Documentation](docs/config.md).

### Global Flags
- `--config`: Path to a specific configuration file.
- `--preset`: Activate a configuration preset.
- `--debug`: Enable verbose debug output.
- `--no-cache`: Bypass the API cache and fetch fresh data from MDBs.

### Presets

Presets allow you to define groups of settings that can be activated via the `--preset` flag. This is useful for recurring release types or specific shows.

Example `config.toml`:

```toml
template = "{title}.{year}.{resolution}.{source}.{audio_codec}{audio_channels}.{video_codec}-{group}"
group = "DefaultGroup"
source = "WEB-DL"

[preset.remux]
source = "BluRay"
template = "{title}.{year}.{resolution}.{source}.REMUX.{video_codec}.{audio_codec}{audio_channels}.{group}"

[preset.marvel]
title = "Loki"
is_tv = true
tmdb_id = 84958
```

You can apply a preset using the `--preset` flag:

```bash
parsec rename --preset remux movie.mkv
```
