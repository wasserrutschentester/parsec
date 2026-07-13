# NFO Template Context Reference

This document is a complete reference for every field available inside NFO templates. The root object (`.`) is the `Context` struct, which aggregates data from MediaInfo, TMDB/TVDB API results, and the file itself.

Use `parsec nfogen file.mkv --dump-context` to inspect the exact values for a specific file.

---

## Root Fields

### Titles & IDs

| Template Field | Type | Description |
| :--- | :--- | :--- |
| `{{ .Title }}` | `string` | Title, sourced from the filename parser and overridden by DB results. |
| `{{ .OriginalTitle }}` | `string` | Non-localized original title from TMDB/TVDB. |
| `{{ .Year }}` | `int` | Release year. |
| `{{ .ImdbID }}` | `string` | IMDB ID (e.g. `tt1234567`). |
| `{{ .TmdbID }}` | `int` | TMDB numeric ID. |
| `{{ .TvdbID }}` | `int` | TVDB numeric ID. |
| `{{ .ImdbURL }}` | `string` | Full IMDB URL (e.g. `https://www.imdb.com/title/tt1234567`). |
| `{{ .TmdbURL }}` | `string` | Full TMDB URL. Includes the correct `movie` or `tv` path based on media type. |
| `{{ .TvdbURL }}` | `string` | Full TVDB URL. Uses a slug-based URL when available, otherwise falls back to `?id=` format. |

### Episode-Specific

| Template Field | Type | Description |
| :--- | :--- | :--- |
| `{{ .Season }}` | `int` | Season number, parsed from the filename. |
| `{{ .Episodes }}` | `[]int` | Slice of episode numbers (supports multi-episode files). |
| `{{ .EpisodeTitle }}` | `string` | Episode title(s), joined with ` / ` for multi-episode files. Populated from the DB or EBML tags. |
| `{{ .EpisodeTitles }}` | `[]string` | Raw slice of individual episode titles before joining. |
| `{{ .Date }}` | `string` | Air date in `YYYY-MM-DD` format, overridden by DB episode airdate. |
| `{{ .IsTV }}` | `bool` | `true` if the media is identified as a TV show. |

### File & Release Info

| Template Field | Type | Description |
| :--- | :--- | :--- |
| `{{ .ReleaseName }}` | `string` | The filename stem (basename without extension). |
| `{{ .Group }}` | `string` | Release group name, parsed from the filename. |
| `{{ .Size }}` | `string` | Pre-formatted file size string (e.g. `"2.34 GiB"`). |
| `{{ .SizeBytes }}` | `int64` | Raw file size in bytes. Useful with `formatSize` or `formatSizeDynamic`. |
| `{{ .Duration }}` | `string` | Human-readable duration (e.g. `"2 h 10 min"` or `"45 min 30 s"`). |
| `{{ .DurationSec }}` | `int` | Duration as a raw integer in seconds. Useful with `formatDuration`. |
| `{{ .Notes }}` | `string` | Custom notes string, passed via `--notes`. |
| `{{ .Sources }}` | `[]string` | Slice of source release names, one per `--source` flag. |
| `{{ .AppVersion }}` | `string` | The running Parsec version string. |
| `{{ .LineWidth }}` | `int` | Current line width (default `72`). Used by built-in partials for column alignment. Can be changed with `.SetLineWidth` or `.WithLineWidth`. |

### Technical Metadata (from filename / config)

These fields are parsed from the filename and can be overridden by config defaults.

| Template Field | Type | Description |
| :--- | :--- | :--- |
| `{{ .Resolution }}` | `string` | Parsed resolution tag (e.g. `"1080p"`, `"2160p"`). Falls back to `{{ .Video.Resolution }}` if empty. |
| `{{ .Source }}` | `string` | Source tag parsed from the filename (e.g. `"WEB-DL"`, `"BluRay"`). |
| `{{ .Service }}` | `string` | Raw streaming service abbreviation from the filename (e.g. `"NF"`, `"AMZN"`). |
| `{{ .ServiceName }}` | `string` | Expanded streaming service name (e.g. `"Netflix"`, `"Amazon"`). Derived from `{{ .Service }}`. |
| `{{ .HDR }}` | `string` | HDR tag parsed from the filename (e.g. `"HDR10"`, `"DV"`). |
| `{{ .BitDepth }}` | `int` | Bit depth parsed from the filename (e.g. `10`). |
| `{{ .AudioCodec }}` | `string` | Audio codec name parsed from the filename (e.g. `"DDP5.1"`). |
| `{{ .AudioChannels }}` | `string` | Audio channel layout parsed from the filename. |
| `{{ .AudioMeta }}` | `string` | Audio metadata tag (e.g. `"Atmos"`). |
| `{{ .VideoCodec }}` | `string` | Video codec name parsed from the filename (e.g. `"H.264"`). |
| `{{ .Language }}` | `string` | Primary language ISO-639 code (e.g. `"en"`, `"de"`). |
| `{{ .LanguageExt }}` | `string` | Extended language tag parsed from the filename. |
| `{{ .CutEdition }}` | `string` | Cut/edition tag (e.g. `"Director's Cut"`). |
| `{{ .Accessibility }}` | `string` | Accessibility tag. |
| `{{ .CRC32 }}` | `string` | CRC32 checksum parsed from the filename. |
| `{{ .Repack }}` | `bool` | `true` if the release is a REPACK. |
| `{{ .Subbed }}` | `bool` | `true` if the filename indicates a subbed release. |
| `{{ .HasAudioDesc }}` | `bool` | `true` if the release includes audio description. |
| `{{ .DualAudio }}` | `bool` | `true` if the release contains dual audio. |

### Plot & Genres (from DB)

| Template Field | Type | Description |
| :--- | :--- | :--- |
| `{{ .Plot }}` | `string` | Plot/overview text from the database. For multi-episode files, individual episode overviews are joined with `\n\n`. |
| `{{ .Genres }}` | `[]string` | List of genre strings (e.g. `["Action", "Drama"]`). |

---

## `{{ .Video }}` — Video Track

Populated from MediaInfo data for the first (and typically only) video track.

| Field | Type | Description |
| :--- | :--- | :--- |
| `{{ .Video.Title }}` | `string` | Track title embedded in the container. |
| `{{ .Video.Codec }}` | `string` | Human-readable codec name (e.g. `"H.264"`, `"H.265"`, `"AV1"`). |
| `{{ .Video.CodecID }}` | `string` | Raw container codec ID (e.g. `"V_MPEG4/ISO/AVC"`). |
| `{{ .Video.Format }}` | `string` | Raw MediaInfo format string (e.g. `"AVC"`, `"HEVC"`). |
| `{{ .Video.Profile }}` | `string` | Codec profile (e.g. `"High"`, `"Main 10"`). |
| `{{ .Video.Level }}` | `string` | Codec level (e.g. `"4.1"`, `"5.1"`). |
| `{{ .Video.Bitrate }}` | `string` | Formatted bitrate string (e.g. `"5000 kb/s"`). |
| `{{ .Video.BitRateMode }}` | `string` | Bitrate mode: `"VBR"` or `"CBR"`. |
| `{{ .Video.BitDepth }}` | `int` | Bit depth (e.g. `8`, `10`, `12`). |
| `{{ .Video.Dimensions }}` | `string` | Formatted pixel dimensions (e.g. `"1920x1080"`). |
| `{{ .Video.AspectRatio }}` | `string` | Calculated aspect ratio (e.g. `"16:9"`). Computed using GCD; falls back to the decimal ratio from MediaInfo. |
| `{{ .Video.Resolution }}` | `string` | Normalized resolution string (e.g. `"1080p"`, `"2160p"`, `"1080i"`). |
| `{{ .Video.Framerate }}` | `string` | Formatted framerate (e.g. `"23.976 FPS"`). |
| `{{ .Video.ChromaSubsampling }}` | `string` | Chroma subsampling (e.g. `"4:2:0"`, `"4:2:2"`). |
| `{{ .Video.HDRFormat }}` | `string` | HDR format string (e.g. `"HDR10"`, `"Dolby Vision"`). Prefers `HDRFormat` over `HDRFormatCompatibility`. |
| `{{ .Video.ScanType }}` | `string` | Scan type: `"Progressive"` or `"Interlaced"`. |
| `{{ .Video.Library }}` | `string` | Encoder library name (e.g. `"x264"`, `"x265"`). |
| `{{ .Video.LibrarySettings }}` | `string` | Full encoder settings string from the library. |
| `{{ .Video.Settings }}` | `string` | Simplified settings summary (e.g. `"CABAC / 4 Ref Frames"`). |
| `{{ .Video.Source }}` | `string` | Source name assigned via `--source-map v1:...`. |
| `{{ .Video.Flags }}` | `Flags` | Track flags. See [Flags](#flags) below. |

---

## `{{ .Audio }}` — Audio Tracks

`{{ .Audio }}` is a `[]Audio` slice. Iterate with `{{ range .Audio }}`.

| Field | Type | Description |
| :--- | :--- | :--- |
| `{{ .Title }}` | `string` | Track title embedded in the container. |
| `{{ .Language }}` | `string` | ISO-639 language code (e.g. `"en"`, `"fr"`). Use with `languageName` to get the full name. |
| `{{ .Codec }}` | `string` | Human-readable codec name (e.g. `"DDP"`, `"TrueHD"`, `"DTS-HD MA"`). |
| `{{ .Channels }}` | `string` | Channel notation (e.g. `"5.1"`, `"7.1"`, `"2.0"`). |
| `{{ .Bitrate }}` | `string` | Formatted bitrate string (e.g. `"640 kb/s"`). |
| `{{ .BitRateMode }}` | `string` | Bitrate mode: `"CBR"` or `"VBR"`. |
| `{{ .Profile }}` | `string` | Codec profile string. |
| `{{ .SamplingRate }}` | `string` | Formatted sampling rate (e.g. `"48.0 kHz"`). |
| `{{ .BitDepth }}` | `int` | Bit depth (e.g. `16`, `24`). |
| `{{ .DialogNormalization }}` | `string` | Dialog normalization value. |
| `{{ .Atmos }}` | `bool` | `true` if the track is identified as Dolby Atmos. |
| `{{ .Source }}` | `string` | Source name assigned via `--source-map a<n>:...`. |
| `{{ .Flags }}` | `Flags` | Track flags. See [Flags](#flags) below. |

---

## `{{ .Subtitles }}` — Subtitle Tracks

`{{ .Subtitles }}` is a `[]Subtitle` slice. Iterate with `{{ range .Subtitles }}`.

| Field | Type | Description |
| :--- | :--- | :--- |
| `{{ .Title }}` | `string` | Track title embedded in the container. |
| `{{ .Language }}` | `string` | ISO-639 language code. Use with `languageName` to get the full name. |
| `{{ .Format }}` | `string` | Subtitle format (e.g. `"ASS"`, `"PGS"`, `"SRT"`). |
| `{{ .ElementCount }}` | `int` | Number of subtitle cue entries. |
| `{{ .SDH }}` | `bool` | `true` if the track title contains `"sdh"` or `"hearing impaired"` (case-insensitive). |
| `{{ .Source }}` | `string` | Source name assigned via `--source-map s<n>:...`. |
| `{{ .Flags }}` | `Flags` | Track flags. See [Flags](#flags) below. |

---

## `Flags`

The `Flags` struct appears on `{{ .Video.Flags }}`, and on every item in `{{ .Audio }}` and `{{ .Subtitles }}` as `.Flags`. These are read from the EBML (Matroska) track headers, falling back to MediaInfo values when EBML data is unavailable.

| Field | Type | Description |
| :--- | :--- | :--- |
| `{{ .Flags.Default }}` | `bool` | `true` if the track is the container's default selection. |
| `{{ .Flags.Forced }}` | `bool` | `true` if the track is marked as Forced. |
| `{{ .Flags.HearingImpaired }}` | `bool` | `true` if the track is marked for hearing impaired viewers (SDH). |
| `{{ .Flags.VisualImpaired }}` | `bool` | `true` if the track contains audio description for visually impaired viewers. |
| `{{ .Flags.TextDescriptions }}` | `bool` | `true` if the track contains text descriptions. |
| `{{ .Flags.Original }}` | `bool` | `true` if the track is in the original language. |
| `{{ .Flags.Commentary }}` | `bool` | `true` if the track is a commentary track. |

**Example: listing only the default audio track**
```gotemplate
{{ range .Audio -}}
  {{ if .Flags.Default }}Default Audio: {{ .Codec }} {{ .Channels }}{{ end }}
{{- end }}
```

---

## Raw Fields

These fields expose the complete, unprocessed API and MediaInfo responses. They are hidden by default in `--dump-context` and visible only with `--dump-context-raw`. They are primarily useful for building advanced templates that access data not surfaced by the standard fields.

| Field | Type | Description |
| :--- | :--- | :--- |
| `{{ .RawSearchResult }}` | `*SearchResult` | The raw TMDB/TVDB search result. `nil` if no DB lookup was performed. |
| `{{ .RawEpisodeResults }}` | `[]EpisodeResult` | One entry per matched episode. Used by the built-in `plot` partial to render per-episode overviews. |
| `{{ .RawMediaInfo }}` | `*MediaInfo` | The full raw MediaInfo JSON output. |
| `{{ .RawEbmlMetadata }}` | `*EbmlMetadata` | The full raw EBML (Matroska) metadata, including all track properties. |

### `EpisodeResult` fields (within `{{ .RawEpisodeResults }}`)

Access via `{{ range .RawEpisodeResults }}` inside the loop scope:

| Field | Type | Description |
| :--- | :--- | :--- |
| `{{ .Name }}` | `string` | Episode title. |
| `{{ .Airdate }}` | `string` | Air date in `YYYY-MM-DD` format. |
| `{{ .Overview }}` | `string` | Episode plot/overview. |
| `{{ .Season }}` | `int` | Season number. |
| `{{ .Episode }}` | `int` | Episode number within the season. |
| `{{ .TvdbID }}` | `int` | TVDB ID for this specific episode. |
| `{{ .ImdbID }}` | `string` | IMDB ID for this specific episode. |
| `{{ .IsFinale }}` | `bool` | `true` if this is a season or series finale. |
| `{{ .TotalEpisodes }}` | `int` | Total number of episodes in the season. |

---

## Built-in Streaming Service Codes

`{{ .ServiceName }}` is automatically expanded from `{{ .Service }}` using the following table:

| Code | Expanded Name | Code | Expanded Name |
| :--- | :--- | :--- | :--- |
| `AMZN` | Amazon | `NF` | Netflix |
| `ATVP` | Apple TV+ (original) | `DSNP` | Disney+ |
| `HULU` | Hulu | `MAX` | Max (Warner Bros. Discovery) |
| `HMAX` | HBO Max | `HBO` | HBO |
| `PCOK` | Peacock | `PMTP` | Paramount+ |
| `SHO` | Showtime | `STZ` | Starz |
| `CR` | Crunchyroll | `HIDI` | HIDIVE |
| `VIAP` | Viaplay | `NOW` | Now (Sky) |
| `SKST` | SkyShowtime | `BNGE` | Binge |
| `DSCP` | Discovery+ | `STAN` | Stan |
| `IP` | BBC iPlayer | `ALL4` | All4 (Channel 4) |
| `ITV` | ITV | `SVT` | Sveriges Television |
| `NRK` | Norsk Rikskringkasting | `ARD` | ARD Mediathek |
| `ZDF` | ZDF Mediathek | `ARTE` | ARTE |
| `ATV` | Apple TV (channel) | `IT` | iTunes |
| `PLAY` | Google Play | `ROKU` | The Roku Channel |
| `TUBI` | TubiTV | `CUR` | CuriosityStream |

> [!TIP]
> Use `{{ .Service }}` for the raw abbreviation and `{{ .ServiceName }}` for the full name. If the service code is not in the table, `{{ .ServiceName }}` will return the raw code unchanged.
