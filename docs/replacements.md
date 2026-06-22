# Regex Replacements

Parsec allows you to define custom regular expression replacement rules to handle complex sanitization, custom formatting, and parsing edge cases. These rules can be applied at three distinct stages of the renaming pipeline: `input`, `title`, and `output`.

Replacement rules can be defined globally or overridden within specific [Presets](config.md#presets).

## How it works

Replacements are configured as an array of tables in your `config.toml` file under the `replacements` key. Because they are defined as an array of tables (`[[replacements.TYPE]]`), you can define multiple rules for each type. They are processed sequentially in the exact order they appear in your configuration file.

### Configuration Format

```toml
[[replacements.<type>]]
pattern = '<regex_pattern>'
replacement = '<replacement_string>'
```

-   **`pattern`**: A valid Go regular expression. You can use capturing groups `()` and case-insensitivity flags like `(?i)`.
-   **`replacement`**: The string to replace matches with. You can reference capturing groups using `$1`, `$2`, etc.

---

## Replacement Stages

### 1. Input Replacements (`replacements.input`)

**When it runs:** At the very beginning, *before* Parsec attempts to parse any metadata from the filename.
**Use case:** Fixing unusual filename formatting so that Parsec's standard parsing logic can correctly identify tags (e.g., converting underscores to dots, mapping custom P2P group source tags to standard ones).

#### Example: Convert underscores to dots
If your files are named like `My_Show_S01E01_1080p_WEB-DL`, Parsec might struggle to parse them. You can replace underscores with dots:

```toml
[[replacements.input]]
pattern = '_'
replacement = '.'
```

#### Example: Normalize obscure web sources
```toml
[[replacements.input]]
pattern = '(?i)web-?rip'
replacement = 'WEBRip'
```

### 2. Title Replacements (`replacements.title`)

**When it runs:** After the title has been extracted from the filename, but *before* the final normalization (which handles diacritics and illegal characters).
**Use case:** Removing unwanted substrings that commonly end up inside parsed titles, such as trailing part numbers, redundant episode indicators, or specific regional descriptors.

*Note: Parsec includes several default `title` replacements out of the box (e.g., converting `&` to `und`). You can view these defaults by running `parsec config init` or reviewing the internal defaults.*

#### Example: Remove "Part X" from the title
```toml
[[replacements.title]]
pattern = '(?i)\s*\((Teil|Part)\s*\d+\)'
replacement = ''
```

#### Example: Keep ampersands (Override default behavior)
If you prefer to keep `&` instead of translating it to `und`, you would redefine the `title` replacements in your config to exclude that rule, or replace it with something else:

```toml
[[replacements.title]]
pattern = '&'
replacement = 'and'
```

### 3. Output Replacements (`replacements.output`)

**When it runs:** At the very end, *after* the new filename has been generated from your template, but before the file extension is appended.
**Use case:** Applying purely subjective stylistic preferences to the final filename that Parsec's standard normalizer wouldn't do automatically.

#### Example: Stylistic Source Renaming
Parsec outputs standard P2P tags like `WEB-DL`. If you prefer a non-standard notation like `WebDL`:

```toml
[[replacements.output]]
pattern = 'WEB-DL'
replacement = 'WebDL'
```

---

## Using Replacements in Presets

Replacement rules fully support Parsec's preset system. You can define base rules at the top level of your configuration, and then completely override them for specific profiles.

**Important:** Because `viper` treats the `[[replacements.TYPE]]` arrays as a single block of values, defining replacements inside a preset completely **overrides** the base rules for that type, rather than appending to them. 

```toml
# Base global input replacements
[[replacements.input]]
pattern = '_'
replacement = '.'

[preset.anime]
# For the anime preset, the base '_' rule is ignored. 
# ONLY the rules defined below will execute when this preset is active.
[[preset.anime.replacements.input]]
pattern = '(?i)Dual-Audio'
replacement = 'Dual Audio'

[[preset.anime.replacements.output]]
pattern = '\[CRC_'
replacement = '['
```

*(Note: Global `replacements.title` rules will still apply to the `anime` preset in this example, because we only overrode `replacements.input` and `replacements.output`)*
