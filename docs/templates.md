# Naming Templates

Parsec uses a flexible template system to generate filenames for the `rename` command and to verify filenames in the `check` command. Templates consist of fixed text and various placeholders (tokens) wrapped in curly braces (e.g., `{title}`).

## Usage

The template is defined in your configuration file under the `template` key. You can also override it within specific presets. If you don't specify a template in a preset, the default template will be used:

```toml
template = "{title}.{year}.{season_id}{episode_id}.{date}.{cut_edition}.{episode_title}.{language}.{language_ext}.{accessibility}.{repack}.{resolution}.{service}.{source}.{audio_codec}{audio_channels}.{audio_meta}.{hdr}.{video_codec}-{group}"
```

## Available Tokens

| Token | Description | Example |
|-------|-------------|---------|
| `{title}` | The main title of the movie or show. | `The.Mandalorian` |
| `{year}` | Release year. | `2019` |
| `{season_raw}` | Season number without padding. | `1` |
| `{season_02}` | Season number with 2-digit padding. | `01` |
| `{season_id}` | Season ID (S + 2-digit padding). | `S01` |
| `{episode_raw}` | Episode number without padding. | `1` |
| `{episode_02}` | Episode number with 2-digit padding. | `01` |
| `{episode_03}` | Episode number with 3-digit padding. | `001` |
| `{episode_03}` | Episode number with 4-digit padding. | `0001` |
| `{episode_id}` | Episode ID (E + 2-digit padding). | `E01` |
| `{date}` | Air date for TV specials (YYYY-MM-DD). | `2019-11-12` |
| `{cut_edition}` | Movie edition/cut. | `Directors.Cut` |
| `{episode_title}` | The title of the specific episode. | `Chapter.1.The.Child` |
| `{language}` | Full English name of the language. | `GERMAN`, `ENGLISH` |
| `{language_ext}` | Language extension tag (e.g. for Dual audio). | `DL`, `ML`, `SUBBED` |
| `{accessibility}` | Accessibility tags ( e.g. `with.Audio.Description`). | `with.Audio.Description` |
| `{repack}` | Inserts `REPACK` if applicable. | `REPACK` |
| `{resolution}` | Video resolution. | `1080p`, `2160p` |
| `{service}` | Streaming service tag. | `ARD`, `DSNP`, `NF` |
| `{source}` | Media source. | `WEB-DL`, `BluRay` |
| `{hdr}` | HDR format. | `HDR`, `DV`, `DV.HDR10Plus` |
| `{bit_depth}` | Video bit depth (only if > 8-bit). | `10bit` |
| `{audio_codec}` | Audio codec name. | `AAC`, `DTS-HD.MA` |
| `{audio_channels}` | Audio channel notation. | `5.1`, `2.0` |
| `{audio_meta}` | Additional audio info (e.g., Atmos). | `Atmos` |
| `{video_codec}` | Video codec name. | `H.264`, `H.265` |
| `{group}` | Release group name. | `YourGroup` |

## Formatting & Cleanup

Parsec automatically cleans up the generated filename to ensure it follows common release naming conventions:

1.  **Empty Tokens:** If a token has no value (e.g., `{service}` when no service is identified), it is removed.
2.  **Empty Enclosures:** If removing a token leaves empty parentheses `()`, brackets `[]`, or braces `{}`, the entire enclosure is removed.
3.  **Separator Collapsing:** Multiple dots `..` or dashes `--` are collapsed into a single separator.
4.  **Cleaning:** Separator combinations like `.-` are cleaned up, and leading/trailing separators are trimmed.
