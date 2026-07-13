# Generating NFO Files

Parsec provides a robust `nfogen` command for generating XML or text `.nfo` files for your media. It uses the media file's name and internal Matroska tags (if available), queries TMDB/TVDB/IMDB for rich metadata, and formats it all using Go `text/template` files.

## Basic Usage

To generate an NFO file for a media file:
```bash
parsec nfogen "Movie.Name.2023.1080p.WEB-DL.x264-GROUP.mkv"
```
Parsec will automatically scan the file with `mediainfo`, fetch metadata from the configured databases, and write `Movie.Name.2023.1080p.WEB-DL.x264-GROUP.nfo` using the default NFO template.

## Flags

| Flag | Shorthand | Type | Description |
|------|-----------|------|-------------|
| `--template` | `-t` | string | The [NFO template](nfo_templating.md) to use (builtin or `.tmpl` in config dir). Defaults to the `nfogen.template` config value. |
| `--notes` | | string | Custom notes to embed in the NFO under `{{ .Notes }}`. |
| `--source` | | string | Add a source release name (can be used multiple times). Populates `{{ .Sources }}` in the template. |
| `--source-map` | | string | Map tracks to a source (e.g., `v1,a1-3:Release-Name`). Sets `.Source` on the matched track structs. |
| `--full-diff` | | boolean | Show the full context for NFO diffs instead of just changed lines. |
| `--unattended` | `-u` | boolean | Run without interactive prompts. |
| `--dry-run` | | boolean | Print the generated NFO to the console without writing to disk. |
| `--dump-context` | | boolean | Dump the template context data as JSON (hides raw fields) for debugging templates. |
| `--dump-context-raw` | | boolean | Dump the template context data as JSON, including all raw database API responses. |

## Source Mapping

Sometimes a media release is composed of multiple sources (e.g., a BluRay video track remuxed with a WEB-DL audio track). The `--source` and `--source-map` flags allow you to document these hybrid releases accurately inside the generated NFO.

The source map format is `<selectors>:<SourceName>`. 
Selectors use a track prefix followed by a 1-indexed track number:
- `v`: Video Track (e.g. `v1`)
- `a`: Audio Track (e.g. `a1`, `a2`)
- `s`: Subtitle Track (e.g. `s1`, `s2`)

You can define ranges (e.g., `a1-3`) and comma-separate multiple selectors (e.g., `v1,a1-3,s1`).

## Examples

**Generate an NFO file using the default template:**
```bash
parsec nfogen "Movie.Name.2023.1080p.WEB-DL.x264-GROUP.mkv"
```

**Preview NFO output without writing to disk:**
```bash
parsec nfogen file.mkv --dry-run
```

**Dump template context as JSON for debugging:**
```bash
parsec nfogen file.mkv --dump-context
```

**Generate an NFO with complex source mapping:**
```bash
parsec nfogen hybrid.mkv \
  --source "My.Movie.2023.1080p.BluRay.x264-GROUP1" \
  --source "My.Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GROUP2" \
  --source-map "v1:My.Movie.2023.1080p.BluRay.x264-GROUP1" \
  --source-map "a1-3,s1-5:My.Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GROUP2"
```

In the template, this mapping will inject the string `My.Movie.2023.1080p.BluRay.x264-GROUP1` into `{{ .Video.Source }}`, and the WEB-DL source string into the `.Source` field for Audio tracks 1 through 3, and Subtitle tracks 1 through 5.

See the **[NFO Context Reference](nfo_context.md)** for a complete list of all template fields available across the `Video`, `Audio`, `Subtitle`, and root context objects.

see the **[NFO Templating Guide](nfo_templating.md)** for the syntax used to create/modify the nfo templates
