# Naming Templates

Parsec uses a flexible template engine powered by Go `text/template` to generate filenames for the `rename` command and to verify filenames in the `check` command.

Templates support both modern Go template syntax with pipeline functions and backward-compatible legacy `{token}` placeholders (which are automatically transpiled at runtime).

---

## 1. Quick Start & Configuration

The template is defined in your configuration file under the `template` key or overridden within presets:

```toml
# Modern default Go template using join helper:
template = '{{join "." .Title .YearTag .SeasonEpisode .Edition .EpisodeTitle .VersionTag .RepackTag .LanguageName .LanguageExtra .Accessibility .Resolution .Service .Source (when .IsRemux "REMUX") .AudioSpec .HDR .VideoCodec}}-{{.Group}}'
```

---

## 2. Template Context Fields

### Domain & Media Properties

#### Title & Series

| Field | Type | Description | Example |
|---|---|---|---|
| `.Title` | `string` | Main release or show title | `The Mandalorian` |
| `.OriginalTitle` | `string` | Foreign or alternative title (empty if identical to `.Title`) | `Sen to Chihiro no Kamikakushi` |
| `.Year` | `int` | Release or production year (numerical, `0` if unknown) | `2019` |
| `.Date` | `string` | Broadcast date for daily shows or specials (`YYYY-MM-DD`) | `2024-03-15` |
| `.Country` | `string` | Country of origin | `ITA`, `JPN` |
| `.Season` | `int` | Season number (`0` for specials) | `1` |
| `.Episodes` | `[]int` | Slice of episode numbers | `[1, 2]` |
| `.EpisodeTitles` | `[]string` | Slice of raw episode titles | `["Pilot"]` |
| `.IsTV` | `bool` | True if identified as a TV show | `true` |

#### Video

| Field | Type | Description | Example |
|---|---|---|---|
| `.Resolution` | `string` | Video resolution | `1080p`, `2160p` |
| `.Service` | `string` | Streaming service identifier | `NF`, `DSNP`, `AMZN` |
| `.Source` | `string` | Media source | `WEB-DL`, `BluRay`, `UHD BluRay` |
| `.HDR` | `string` | HDR format tags | `HDR`, `DV`, `DV HDR10` |
| `.BitDepth` | `int` | Video bit depth | `10`, `8` |
| `.FrameRate` | `float64` | Video track frame rate | `23.976`, `25.0` |
| `.ScanType` | `string` | Video scan type | `Progressive`, `Interlaced` |
| `.VideoCodec` | `string` | Resolved video codec name according to codec style matrix | `H.264`, `H.265`, `x264`, `x265`, `AVC`, `HEVC` |
| `.CodecStyle` | `string` | Explicit video codec style override | `web_dl`, `encode`, `remux` |
| `.IsRemux` | `bool` | True if identified as a REMUX | `true` |
| `.IsEncode` | `bool` | True if identified as an encode | `true` |

#### Audio & Accessibility

| Field | Type | Description | Example |
|---|---|---|---|
| `.AudioCodec` | `string` | Primary audio codec | `DDP`, `TrueHD`, `DTS-HD MA`, `FLAC` |
| `.AudioChannels` | `string` | Audio channel notation | `5.1`, `7.1`, `2.0` |
| `.AudioExtra` | `string` | Spatial audio or extra features | `Atmos` |
| `.Accessibility` | `string` | Accessibility tag from filename | `with.Audio.Description`, `WITH.AD` |
| `.HasAudioDesc` | `bool` | True if audio description is present | `true` |

#### Languages & Subtitles

| Field | Type | Description | Example |
|---|---|---|---|
| `.LanguageISO` | `string` | Primary selected audio ISO language code | `de`, `en`, `ja` |
| `.OriginalLanguageISO`| `string` | Source / MDB original language code | `ja`, `fr` |
| `.LanguageExtra` | `string` | Language extra tag | `DL`, `ML`, `SUBBED` |
| `.AudioLanguages` | `[]string` | Slice of all unique audio language ISO codes in file | `["en", "de"]` |
| `.SubtitleLanguages` | `[]string` | Slice of all unique subtitle language ISO codes in file | `["de", "en"]` |
| `.IsDualAudio` | `bool` | True if audio has exactly 2 language tracks | `true` |
| `.IsMultiAudio` | `bool` | True if audio has 3 or more language tracks | `true` |
| `.IsSubbed` | `bool` | True if video audio is foreign but preferred subs exist (only set automatically if `subbed_tagging=true`) | `true` |
| `.HasPreferredAudio` | `bool` | True if audio includes user's `preferred_language` | `true` |

#### Release & Edition

| Field | Type | Description | Example |
|---|---|---|---|
| `.Edition` | `string` | Cut or edition | `Extended`, `Remastered` |
| `.Group` | `string` | Release group identifier | `YourGroup` |
| `.CRC32` | `string` | File CRC32 hash (uppercase, no brackets) | `DEADBEEF` |
| `.IsRepack` | `bool` | True if release is marked as a repack | `true` |
| `.RepackLevel` | `int` | Repack level (`1` for REPACK, `2` for REPACK2) | `2` |
| `.ReleaseVersion` | `int` | Quality upgrade version (`2` for v2, `3` for v3) | `2` |
| `.ImdbID` | `string` | IMDb identifier | `tt1234567` |
| `.TmdbID` | `int` | TMDB identifier | `82856` |
| `.TvdbID` | `int` | TVDb identifier | `361111` |

### Precomputed Convenience Fields

| Field | Type | Description | Example |
|---|---|---|---|
| `.YearTag` | `string` | Formatted release year (omitted if unknown or `0`) | `2019`, `""` |
| `.SeasonEpisode` | `string` | Formatted season and episode tag (or pack) | `S01E01`, `S01E01-E04`, `S01` (pack) |
| `.SeasonID` | `string` | Formatted season tag | `S01`, `S00` |
| `.EpisodeID` | `string` | Formatted episode tag | `E01`, `E01-E04` |
| `.EpisodeTitle` | `string` | Space-joined episode titles | `Chapter 1 The Child` |
| `.AudioSpec` | `string` | Merged audio codec, channels, and extra | `DDP5.1.Atmos`, `TrueHD7.1.Atmos` |
| `.LanguageName` | `string` | Full uppercase language name | `GERMAN`, `ENGLISH` |
| `.RepackTag` | `string` | Formatted repack string (`REPACK`, `REPACK2`) | `REPACK`, `REPACK2` |
| `.VersionTag` | `string` | Release quality upgrade version (`v2`, `v3`) | `v2`, `v3` |
| `.IsPack` | `bool` | True if release is a season pack | `true` |

### Raw Context Objects

For advanced inspection and nested queries using `where` or `pluck`:

| Field | Type | Description |
|---|---|---|
| `.RawMediaInfo` | `*mediainfo.MediaInfo` | Full MediaInfo track structure. |
| `.RawEbmlMetadata` | `*matroska.EbmlMetadata` | Raw Matroska EBML element metadata. |
| `.RawSearchResult` | `*mdb.SearchResult` | Raw metadata database search result from TMDB/TVDB. |
| `.RawEpisodeResults` | `[]mdb.EpisodeResult` | Raw episode result list from TMDB/TVDB search. |
| `.RawMDB` | `*mdb.SearchResult` | Backward-compatible alias for `.RawSearchResult`. |

---

## 3. Template Helper Functions

### Joining & Formatting

- `join <sep> <parts...>`: Joins values using separator `sep`, automatically omitting empty strings, `nil`, Automatically unpacks slices.
  ```gotemplate
  {{join "." .Title .YearTag .Resolution .Source .VideoCodec}}
  ```
- `cat <parts...>`: Concatenates values without separator.
  ```gotemplate
  {{cat "[" .RepackTag "]"}}
  ```
- `when <cond1> <val1> [<cond2> <val2>...] [<fallback>]`: Evaluates condition/value pairs sequentially. If single condition is used, fallback defaults to `""`.
  ```gotemplate
  {{when .IsRemux "REMUX"}}
  {{when .IsRemux "REMUX" "ENCODE"}}
  {{when .IsMultiAudio "MULTI" .IsDualAudio "DUAL" (title .LanguageName)}}
  ```
- `pad <digits> <int>`: Formats integer with zero padding.
  ```gotemplate
  {{pad 2 .Season}} -> "01"
  ```
- `eprange <prefix> <digits> <episodes>`: Formats slice of episode integers.
  ```gotemplate
  {{eprange "E" 2 .Episodes}} -> "E01" or "E01-E04"
  {{eprange "" 2 .Episodes}}  -> "01" or "01-04"
  ```
- `aka <origTitle> <title> [<year>]`: Formats foreign AKA release titles using configured `word_separator`. Falls back cleanly to `<title>` if `<origTitle>` is missing or identical to `<title>`.
  ```gotemplate
  {{aka .OriginalTitle .Title}} -> "Sen.to.Chihiro.no.Kamikakushi.AKA.Spirited.Away"
  {{aka .OriginalTitle .Title .YearTag}} -> "Gisaengchung.2019.AKA.Parasite"
  {{aka "" .Title}} -> "Inception"
  ```
- `vcodec <style> <codec>`: Formats video codec name according to a specific style table.
  ```gotemplate
  {{vcodec "encode" .VideoCodec}} -> "x264"
  ```
- `default <fallback> <value>`: Returns fallback if value is `nil`, empty string, or `0`.
  ```gotemplate
  {{default "Unknown" .Edition}}
  ```
- `languageName <isoCode>`: Converts an ISO-639 code into an uppercase English language name.
  ```gotemplate
  {{languageName .OriginalLanguageISO}} -> "JAPANESE"
  ```

### String Utilities

All string helpers are pipe-friendly with the target string as the last argument:

- `toUpper <str>` / `upper <str>`: Converts string to uppercase (`UPPERCASE`).
- `toLower <str>` / `lower <str>`: Converts string to lowercase (`lowercase`).
- `titleCase <str>` / `title <str>`: Titlecases string (`Titlecase`).
- `replace <old> <new> <str>`: Replaces all occurrences of `old` with `new` (pipe-friendly: `{{ .Title | replace " " "." }}`).
- `regexReplace <pattern> <repl> <str>`: Replaces regex matches (pipe-friendly: `{{ .Title | regexReplace "[._]" " " }}`).
- `contains <item> <container>`: Returns true if substring exists in string, or if item exists in a slice/array.
  ```gotemplate
  {{if contains "de" .AudioLanguages}}German{{end}}
  {{if contains "UHD" .Source}}4K{{end}}
  ```
- `trimPrefix <prefix> <str>` / `trimSuffix <suffix> <str>`: Trims prefix or suffix (pipe-friendly: `{{ .Title | trimPrefix "The." }}`).
- `hasPrefix <prefix> <str>` / `hasSuffix <suffix> <str>`: Checks prefix or suffix (pipe-friendly: `{{ if .Title | hasPrefix "The." }}`).

### Query & Slicing Helpers

- `where <key> [<operator>] <matchValue> <slice>`: Filters a slice of structs or maps by field value. Supports operators `==`, `!=`, `>`, `<`, `>=`, `<=`, `in`, `contains`.
- `pluck <key> <slice>`: Extracts all values of a named property across a slice.
- `first <limit> <slice>` / `last <limit> <slice>`: Takes the first or last N elements.
- `uniq <slice>`: Removes duplicate values.
- `indexOrEmpty <index> <slice>`: Safely gets element at index without panics.
- `list <items...>`: Turns arguments into a slice: `{{ list "A" "B" | join "." }}`.
- `formatDate <format> <date>`: Formats a date string or `time.Time` (pipe-friendly: `{{ .Date | formatDate "02.01.2006" }}`).


### Standard Go Template Operators

All standard Go `text/template` primitives and pipelines are fully supported:
- Comparisons: `eq`, `ne`, `lt`, `le`, `gt`, `ge`
- Logic: `and`, `or`, `not`
- Conditionals: `{{if ...}} ... {{else if ...}} ... {{else}} ... {{end}}`
- Iteration: `{{range .Episodes}} ... {{end}}`
- Pipelines: `{{.Title | upper}}`

---

## 4. Video Codec Styling Matrix

Parsec automatically styles video codecs depending on release type (`web_dl`, `remux`, `encode`):

```toml
# Video codec styling matrix
[video_codec_style.web_dl]
AVC = "H.264"
HEVC = "H.265"

[video_codec_style.remux]
AVC = "AVC"
HEVC = "HEVC"

[video_codec_style.encode]
AVC = "x264"
HEVC = "x265"
```

### Setting a Default Codec Style

When release type cannot be auto-detected, or when a specific style table lacks a mapping for a codec, Parsec falls back to `default`. You can define the default style in two ways:

1. **Mapping table:**
   ```toml
   [video_codec_style.default]
   AVC = "H.264"
   HEVC = "H.265"
   ```

2. **Style alias:**
   ```toml
   [video_codec_style]
   default = "web_dl"
   ```

The active style can also be overridden on the command line using `--vcodec-style <web_dl|remux|encode>` or within a preset via `codec_style`.

---

## 5. Inspection & Testing

To inspect the exact values available to your template for any file, use the context dump flags with the `rename` command:

```bash
# Dump clean TemplateContext JSON
parsec rename --dump-context "My.Show.S01E01.mkv"

# Dump full context including raw MediaInfo tracks and MDB results
parsec rename --dump-context-raw "My.Show.S01E01.mkv"
```

---

## 6. Pipeline Mechanics & Sanitization

1. **Word Separator & Cleaning (`cleanName`):**
   - Redundant separators (multiple dots, dashes, or spaces) are automatically collapsed.
   - Empty brackets (such as `()`, `[]`, `{}`) left by omitted conditional tags are automatically removed.
   - Spaces are converted to the configured `word_separator` (default `.`).
2. **Length Safeguard (245 Bytes):**
   - If the generated filename exceeds 245 bytes, Parsec automatically removes `.EpisodeTitle` and re-renders to safeguard against filesystem limits.
3. **Season Pack Mode (`--season-pack`):**
   - Episode-specific fields (`.Episodes`, `.EpisodeTitles`, `.EpisodeID`, `.EpisodeTitle`, `.Date`) are cleared.
   - `.SeasonEpisode` automatically falls back to `.SeasonID`.
   - `.IsPack` is set to `true`.

---

## 7. Backwards Compatibility with Legacy Tokens

Existing configuration files with legacy `{token}` syntax continue to work without modification. Parsec automatically transpiles legacy tokens to modern Go template expressions at runtime:

| Legacy Token | Modern Expression |
|---|---|
| `{title}` | `{{.Title}}` |
| `{year}` | `{{.YearTag}}` |
| `{season_id}` | `{{.SeasonID}}` |
| `{season_raw}` | `{{when (or .IsTV (gt .Season 0)) .Season}}` |
| `{season_02}` | `{{when (or .IsTV (gt .Season 0)) (pad 2 .Season)}}` |
| `{episode_id}` | `{{.EpisodeID}}` |
| `{episode_raw}` | `{{eprange "" 0 .Episodes}}` |
| `{episode_02}` | `{{eprange "" 2 .Episodes}}` |
| `{episode_03}` | `{{eprange "" 3 .Episodes}}` |
| `{episode_04}` | `{{eprange "" 4 .Episodes}}` |
| `{episode_title}` | `{{.EpisodeTitle}}` |
| `{date}` | `{{.Date}}` |
| `{language}` | `{{.LanguageName}}` |
| `{language_ext}` | `{{.LanguageExtra}}` |
| `{cut_edition}` | `{{.Edition}}` |
| `{accessibility}` | `{{.Accessibility}}` |
| `{resolution}` | `{{.Resolution}}` |
| `{service}` | `{{.Service}}` |
| `{source}` | `{{.Source}}` |
| `{hdr}` | `{{.HDR}}` |
| `{audio_codec}` | `{{.AudioCodec}}` |
| `{audio_channels}` | `{{.AudioChannels}}` |
| `{audio_meta}` | `{{.AudioExtra}}` |
| `{video_codec}` | `{{.VideoCodec}}` |
| `{group}` | `{{.Group}}` |
| `{dual_audio}` | `{{when .IsDualAudio "Dual-Audio"}}` |
| `{subbed}` | `{{when .IsSubbed "[SUBBED]"}}` |
| `{crc32}` | `{{.CRC32}}` |
| `{bit_depth}` | `{{when (gt .BitDepth 8) (cat .BitDepth "bit")}}` |
| `{repack}` | `{{.RepackTag}}` |
