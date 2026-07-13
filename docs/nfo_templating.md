# NFO Templating System

Parsec uses Go's powerful `text/template` engine to dynamically generate `.nfo` files based on media metadata, API data (TMDB/TVDB), and internal tracking data.

## Go Template Basics

If you have never written a Go template before, here is a very quick crash course. For comprehensive guides, you can refer to the [Official Go `text/template` Documentation](https://pkg.go.dev/text/template)

### Variables and Control Flow
- **The Dot (`.`)**: Represents the current data cursor. Access fields using `{{ .FieldName }}`.
- **Conditionals**: `{{ if .Title }} Title is: {{ .Title }} {{ else }} No Title {{ end }}`
- **Loops**: Use `range` to iterate over arrays. Inside a loop, the Dot (`.`) becomes the current item:
  ```gotemplate
  {{ range .Audio }}
    Codec: {{ .Codec }}
  {{ end }}
  ```

### Whitespace Trimming (The Hyphen `-`)
One of the most common issues people run into with `.nfo` XML files is accidental blank lines or excessive spaces. 
In Go templates, every newline or space *outside* of `{{ }}` tags is printed exactly as written.

You can tell the template engine to aggressively strip all whitespace (including newlines) on either side of an action by adding a hyphen (`-`) and a space inside the brackets:
- `{{- .Title }}`: Removes all whitespace immediately **before** the variable.
- `{{ .Title -}}`: Removes all whitespace immediately **after** the variable.
- `{{- .Title -}}`: Removes all whitespace on **both sides**.

**Example:**
```gotemplate
{{ range .Genres -}}
  <genre>{{ . }}</genre>
{{- end }}
```
Without the hyphens, the above loop would insert a bunch of blank lines between every `<genre>` tag because of the newlines before and after the `<genre>` string in the template file.

## The Context Object

When an NFO is generated, Parsec constructs a `Context` object containing all parsed metadata and database API results. This context is passed into the template as the root element (`.`).

To view the exact context available for any given file, run:
```bash
parsec nfogen file.mkv --dump-context
```

### Common Root Fields
- `{{ .Title }}`: The title parsed from the filename.
- `{{ .OriginalTitle }}`: The original, non-localized title from TMDB/TVDB.
- `{{ .Year }}`: The release year.
- `{{ .Plot }}`: The plot/overview of the media.
- `{{ .Genres }}`: A list of genres (e.g., `["Action", "Drama"]`).
- `{{ .ImdbID }}`, `{{ .TmdbID }}`, `{{ .TvdbID }}`: The parsed IDs for the TV Show or Movie.
- `{{ .ImdbURL }}`, `{{ .TmdbURL }}`, `{{ .TvdbURL }}`: Full URLs to each database page.
- `{{ .EpisodeTitle }}`: Episode title(s), joined with ` / ` for multi-episode files.
- `{{ .ReleaseName }}`: The filename stem (without extension).
- `{{ .Size }}`: Pre-formatted file size string (e.g. `"2.34 GiB"`).
- `{{ .Duration }}`: Human-readable duration (e.g. `"2 h 10 min"` or `"45 min 30 s"`).
- `{{ .DurationSec }}`: Duration as a raw integer in seconds.
- `{{ .SizeBytes }}`: Raw file size in bytes.
- `{{ .Notes }}`: The custom notes string passed via `--notes`.
- `{{ .Sources }}`: A `[]string` slice of all source names added via `--source`.
- `{{ .ServiceName }}`: Expanded streaming service name (e.g. `"NF"` → `"Netflix"`). See the full context reference for a complete list.
- `{{ .LineWidth }}`: The current line width (default `72`). Used by built-in partials.
- `{{ .AppVersion }}`: The running Parsec version string.

> [!NOTE]
> For a complete reference of every available field — including all `Video`, `Audio`, `Subtitle`, and `Flags` sub-fields, and all embedded metadata fields — see [nfo_context.md](nfo_context.md).

### Media Tracks
MediaInfo parsing exposes deeply nested structs for Video, Audio, and Subtitles:
```gotemplate
Video Codec: {{ .Video.Codec }}
Video Bitrate: {{ .Video.Bitrate }}
Video HDR: {{ .Video.HDRFormat }}

{{ range .Audio }}
Audio Track: {{ .Title }} ({{ .Language }})
Audio Codec: {{ .Codec }} | {{ .Channels }}
  {{ if .Atmos }}(Atmos){{ end }}
  {{ if .Flags.Default }}[Default]{{ end }}
{{ end }}

{{ range .Subtitles }}
Subtitle: {{ .Language }} ({{ .Format }})
  {{ if .SDH }}[SDH]{{ end }}
  {{ if .Flags.Forced }}[Forced]{{ end }}
{{ end }}
```

See [nfo_context.md](nfo_context.md) for a full list of fields on each track struct.

## Template Validation

You can statically check if your custom NFO templates have syntax errors or reference non-existent variables using:
```bash
parsec config validate
```
This performs a full static analysis of your templates, simulating an execution run to ensure you haven't misspelled any fields (e.g., `{{ .AudioCodec }}` vs `{{ .Audio.Codec }}`).

## Built-in Helper Functions

Parsec provides a massive library of utility functions to help format strings and arrays inside your templates.

### A Note on Pipes (`|`)
Go templates support piping, where the output of one command is passed as the **last argument** to the next command. 
For example, `{{ .Genres | join ", " }}` is equivalent to `{{ join ", " .Genres }}`.

### String Manipulation
| Function | Description | Pipe-friendly? | Example Usage |
| :--- | :--- | :--- | :--- |
| `toUpper` | Converts a string to uppercase. | Yes | `{{ .Title \| toUpper }}` |
| `toLower` | Converts a string to lowercase. | Yes | `{{ .Title \| toLower }}` |
| `titleCase` | Converts a string to Title Case. | Yes | `{{ "hello world" \| titleCase }}` |
| `replace` | Replaces occurrences of a substring. Parameters: `old`, `new`, `input`. | Yes | `{{ .Plot \| replace "old" "new" }}` |
| `split` | Splits a string into an array. Parameters: `separator`, `input`. | Yes | `{{ .Title \| split " " }}` |
| `join` | Joins an array into a string. Parameters: `separator`, `array`. | Yes | `{{ .Genres \| join ", " }}` |
| `wordWrap` | Wraps text to a max width. Existing `\n` newlines are preserved. Parameters: `limit`, `input`. | Yes | `{{ .Plot \| wordWrap 72 }}` |
| `breakReleaseName` | Breaks a release name at `.`, `-`, `_` characters. **Returns `[]string`**, not a string — pipe to `join "\n"` to render, or use `indexOrEmpty` for a specific line. Parameters: `limit`, `input`. | Yes | `{{ .ReleaseName \| breakReleaseName 30 \| join "\n" }}` |
| `repeat` | Repeats a string `n` times. Parameters: `count`, `input`. | Yes | `{{ "-" \| repeat 10 }}` |
| `indexOrEmpty` | Safely returns the item at a given index of a `[]string` slice, or `""` if the index is out of bounds. Parameters: `index`, `slice`. | Yes | `{{ .ReleaseName \| breakReleaseName 30 \| indexOrEmpty 0 }}` |
| `trimPrefix` | Removes a prefix from the start of a string. Parameters: `prefix`, `input`. | Yes | `{{ .Video.Codec \| trimPrefix "V_" }}` |
| `trimSuffix` | Removes a suffix from the end of a string. Parameters: `suffix`, `input`. | Yes | `{{ .Audio.Codec \| trimSuffix " MA" }}` |
| `hasPrefix` | Checks if a string starts with a prefix. Parameters: `prefix`, `input`. | Yes | `{{ if .Video.Codec \| hasPrefix "V_" }}` |
| `hasSuffix` | Checks if a string ends with a suffix. Parameters: `suffix`, `input`. | Yes | `{{ if .Audio.Codec \| hasSuffix " MA" }}` |
| `humanize` | Normalizes snake_case, kebab-case, or camelCase to a clean sentence. | Yes | `{{ "HearingImpaired" \| humanize }}` → `Hearing impaired` |
| `truncate` | Truncates a string to a limit, respecting word boundaries and adding an ellipsis. Parameters: `limit`, `[ellipsis]`, `input`. | Yes | `{{ .Plot \| truncate 150 }}` |
| `chomp` | Strips trailing newlines. | Yes | `{{ include "partial" . \| chomp }}` |

### Layout & Padding
These functions are designed to help you align text into neat columns. They all accept the target string as the last argument, making them fully pipe-friendly.
| Function | Description | Pipe-friendly? | Example Usage |
| :--- | :--- | :--- | :--- |
| `padRight` | Pads string on the right. If the string is **longer** than `length`, it is truncated (unless `allowOverflow` is `true`). Handles multi-line input by padding each line individually. Parameters: `length`, `[padChar]`, `[allowOverflow]`, `input`. | Yes | `{{ .Video.Codec \| padRight 10 }}` <br> `{{ .Title \| padRight 20 "." }}` <br> `{{ .Title \| padRight 20 " " true }}` |
| `padLeft` | Pads string on the left. Truncates (or not, if `allowOverflow` is `true`) when input exceeds `length`. Parameters: `length`, `[padChar]`, `[allowOverflow]`, `input`. | Yes | `{{ .Video.Bitrate \| padLeft 15 }}` |
| `center` | Centers a string within a width. Parameters: `length`, `input`. | Yes | `{{ "INFO" \| center 20 }}` |
| `applyBorder` | Wraps multiline text with a border string on each side. Strips leading/trailing newlines from input before applying borders. Parameters: `left`, `right`, `input`. | Yes | `{{ .Plot \| applyBorder "│ " " │" }}` |

### Media Formatting
| Function | Description | Pipe-friendly? | Example Usage |
| :--- | :--- | :--- | :--- |
| `formatNumber` | Formats a number with space separators (e.g. `1 000 000`). | Yes | `{{ .Video.Bitrate \| formatNumber }}` |
| `formatSize` | Converts bytes to GiB with a specific decimal precision. Parameters: `precision`, `input`. | Yes | `{{ .SizeBytes \| formatSize 2 }}` |
| `formatSizeDynamic`| Converts bytes to KiB, MiB, GiB, TiB, or PiB dynamically based on scale. Parameters: `precision`, `input`. | Yes | `{{ .SizeBytes \| formatSizeDynamic 2 }}` |
| `formatDuration` | Converts seconds to `HH:MM:SS` (default) or a custom layout string (e.g. `"h'h' m'm' s's'"`). Parameters: `[layout]`, `input`. | Yes | `{{ .DurationSec \| formatDuration "h'h' m'min'" }}` |
| `formatDate` | Formats a date string using standard Go date formatting (based on the reference date `2006-01-02`). Passing `""` as `formatStr` returns the raw date string unchanged. Parameters: `formatStr`, `date`. | Yes | `{{ .Date \| formatDate "02.01.2006" }}` → `21.07.2023` |
| `languageName` | Converts an ISO-639 code to its full English name in uppercase. Special codes: `mul` → `MULTI`, `zxx` → `SILENT`. | Yes | `{{ "de" \| languageName }}` → `GERMAN` |
| `formatBitrate` | Formats raw numeric bitrates into clean units (default `kb/s` with space separation, or `mbps`). Parameters: `[unit]`, `input`. | Yes | `{{ 5120000 \| formatBitrate "mbps" }}` → `5.12 Mbps` |
| `extractCrf` | Extracts `(crfXX)` from an encoder settings string if a `crf=` token is present. Returns `""` if not found. | Yes | `{{ .Video.LibrarySettings \| extractCrf }}` |
| `now` | Returns the current local time object. | No | `{{ now \| formatDate "2006-01-02" }}` |

### Array & Object Utilities
| Function | Description | Pipe-friendly? | Example Usage |
| :--- | :--- | :--- | :--- |
| `pluck` | Extracts a specific field from an array of structs. Parameters: `key`, `array`. | Yes | `{{ .Audio \| pluck "Codec" \| join ", " }}` |
| `uniq` | Removes duplicate items from an array. | Yes | `{{ .Audio \| pluck "Language" \| uniq }}` |
| `contains`| Checks if an array contains a value. Parameters: `search`, `array`. | Yes | `{{ if .Genres \| contains "Action" }}` |
| `dict` | Creates a key-value map from pairs of arguments. Parameters: `key1`, `val1`, `key2`, `val2`, ... | No | `{{ include "partial" (dict "Track" . "Width" 40) }}` |
| `default` | Returns a fallback default value if the input is empty/falsy. Parameters: `defaultVal`, `input`. | Yes | `{{ .Plot \| default "No plot available." }}` |
| `where` | Filters an array/slice of structs/maps based on a field key and matching value. Parameters: `key`, `[operator]`, `matchVal`, `array`. | Yes | `{{ range where "Flags.Forced" true .Subtitles }}` |
| `first` | Returns the first N elements of a slice. Parameters: `limit`, `slice`. | Yes | `{{ range first 2 .Audio }}` |
| `last` | Returns the last N elements of a slice. Parameters: `limit`, `slice`. | Yes | `{{ range last 2 .Audio }}` |

### Math
| Function | Description | Pipe-friendly? | Example Usage |
| :--- | :--- | :--- | :--- |
| `add`, `sub` | Addition / Subtraction. Parameters: `a`, `b`. | **No** | `{{ add 5 2 }}` |
| `mul`, `div` | Multiplication / Division. Parameters: `a`, `b`. | **No** | `{{ div 10 2 }}` |
| `max` | Returns the maximum value of the numeric arguments. Parameters: `a`, `b`, ... | **No** | `{{ max 5 2 8 }}` |
| `min` | Returns the minimum value of the numeric arguments. Parameters: `a`, `b`, ... | **No** | `{{ min 5 2 8 }}` |
| `round` | Rounds a number to the nearest integer. | Yes | `{{ 5.6 \| round }}` |

## Built-in Templates

Parsec ships with the following built-in templates that can be used directly with `-t`:

| Name | Description |
| :--- | :--- |
| `default` | Full ASCII-art box layout with sections for video, audio, subtitles, plot, and links. |
| `minimal` | Compact plain-text layout. Overrides the `art` partial with a simple banner. |

you can look at the built-in templates and partials under [internal/nfo/templates](../internal/nfo/templates/)

## Where to Put Your Templates

Parsec will automatically look for your custom `.tmpl` files in the `nfo/` directory alongside your Parsec configuration file (usually `~/.config/parsec/nfo/` on Linux/macOS, or `%APPDATA%\parsec\nfo\` on Windows).

For example, if you run `parsec nfogen -t custom`, Parsec will look for a file at `~/.config/parsec/nfo/custom.tmpl`.

### Adding Descriptions for Shell Completion

If you use Parsec's CLI autocompletion for the `--template` flag, you can add a description to your custom templates so they show up nicely in the suggestions list.

To do this, simply add a Go template comment on the **very first line** of your `.tmpl` file formatted exactly like this:

```gotemplate
{{/* Description: My custom template for anime releases */}}
```

Parsec parses this first line when generating shell completions and displays the text after `Description:` next to your template's name.

## Partials and Includes

To keep templates organized, Parsec supports breaking down templates into smaller `partial` files. Any template placed in your config directory under `nfo/partial/*.tmpl` (e.g. `~/.config/parsec/nfo/partial/`) will be automatically loaded.

However, simply dropping a file in that folder is not enough. Because of how Go's templating engine works, you **must** wrap the contents of your partial file in a `{{ define "name" }}` block. This tells the engine exactly what name to register the partial under, so it can be called later.

**Example: `nfo/partial/video_info.tmpl`**
```gotemplate
{{ define "video_info" }}
Video Codec: {{ .Codec }}
Bitrate: {{ .Bitrate }}
{{ end }}
```

### `include` vs `template`
The standard Go way to render a partial is using the built-in `template` action (e.g., `{{ template "video_info" .Video }}`). However, the `template` action writes its output directly to the document, which means **you cannot pipe its output to other functions**.

To solve this, Parsec provides a custom `include` function. The `include` function executes the partial, captures the output as a string, and returns it. This allows you to aggressively format the output of an entire partial using pipes!

```gotemplate
{{/* Standard Go way (Cannot be piped) */}}
{{ template "video_info" .Video }}

{{/* Parsec's include function (Can be piped!) */}}
{{ include "video_info" .Video | applyBorder "║ " " ║" }}
```

### Overriding Built-in Partials

Because Parsec loads local partials *after* the built-in ones, you can easily override specific sections of the default NFO template without having to copy and maintain the entire `default.tmpl` file!

For example, if you love the default layout but just want to replace the ASCII art at the top:
1. Create a file at `~/.config/parsec/nfo/partial/art.tmpl` (or your equivalent OS config directory).
2. Define the `art` block inside it:
```gotemplate
{{ define "art" }}
  ___  ___        _      
  |  \/  |       (_)     
  | .  . | _____  _  ___ 
  | |\/| |/ _ \ \/ // _ \
  | |  | | (_) >  <|  __/
  \_|  |_/\___/_/\_\\___|
{{ end }}
```
3. Run `parsec nfogen` using the `default` template. Because Go template names are global, your local `art` definition will silently overwrite the built-in `art` definition, seamlessly injecting your custom ASCII art block in place of the default one!

> [!IMPORTANT]
> The **`{{ define "name" }}`** block inside the file controls the override — not the filename. You can name the file anything you like (e.g. `my_art.tmpl`), as long as the `define` block uses the correct name (e.g. `"art"`).

## Context Methods

The context object exposes two methods for controlling `LineWidth` from within a template:

### `SetLineWidth`

```gotemplate
{{ .SetLineWidth 68 -}}
```

Mutates `LineWidth` on the shared context **in-place** and returns `""` (no output). Because the entire template execution shares a single `*Context` pointer, this change is visible to every partial called after it — making it the right tool for setting a global width at the top of a root template. The built-in `default.tmpl` uses this pattern.

### `WithLineWidth`

```gotemplate
{{ include "release_name" (.WithLineWidth 70) | applyBorder "|" "|" }}
```

Returns a **shallow copy** of the context with `LineWidth` overridden. The original context is untouched, so this only affects the partial it is passed to. Use this when one section needs a different width without disturbing the rest of the template.

| | `SetLineWidth` | `WithLineWidth` |
|---|---|---|
| **Effect** | Mutates the shared context permanently | Isolated to the partial it's passed to |
| **Returns** | `""` (no output) | A new context pointer |
| **Use when** | Setting a global width at the top of a template | Passing a one-off width to a single `include` call |

> [!NOTE]
> The `include` function always strips leading and trailing newlines (`\n`, `\r`) from the output of the partial before returning it. Keep this in mind if you need to preserve a leading or trailing blank line — you will need to add it back after the pipe.
