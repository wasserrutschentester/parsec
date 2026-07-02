# Tag Templates

Parsec allows you to completely customize how metadata is written to Matroska tags using **Tag Templates**. This system uses Go's `text/template` engine to give you ultimate flexibility in translating scraped metadata to MKV tags.

## Configuration

By default, Parsec includes a tagging scheme that writes `TITLE`, `IMDB`, `TMDB`, and `TVDB` tags mimicking the old hardcoded behavior. 

If you want to customize these tags, you can create a custom tag configuration. In your main `parsec.toml`, set the `tag_template` option:

```toml
# parsec.toml
tag_template = "my_custom_tags"
```

Parsec will then look for a file named `my_custom_tags.toml` in your configuration's `tags` directory (e.g. `~/.config/parsec/tags/my_custom_tags.toml`).

## Template File Structure

The `tags` configuration file is a TOML document containing an array of tag sets. Each tag set corresponds to a Matroska `TargetTypeValue` and defines a map of fields to write.

Example `~/.config/parsec/tags/my_custom_tags.toml`:

```toml
[[tags]]
target_value = 50 # Movie / Episode target
[tags.fields]
TITLE = "{{if .Episode}}{{.Episode.Name}}{{else}}{{.Media.Title}}{{end}}"
IMDB = "{{.Media.ImdbID}}"
TMDB = "{{if .Media.TmdbID}}{{if .Media.IsTV}}tv{{else}}movie{{end}}/{{.Media.TmdbID}}{{end}}"
DATE_RELEASED = "{{.Media.Airdate}}"

[[tags]]
target_value = 60 # Season / Volume target
[tags.fields]
TITLE = "Season {{.Episode.Season}}"
```

### Example XML Output

Using the default configuration (or a custom one like the above), Parsec constructs standard Matroska XML tags to feed to `mkvpropedit`. 

For a **Movie**, the output typically looks like this:

```xml
<Tags>
  <Tag>
    <Targets>
      <TargetTypeValue>50</TargetTypeValue>
    </Targets>
    <Simple>
      <Name>TITLE</Name>
      <String>The Matrix</String>
    </Simple>
    <Simple>
      <Name>DATE_RELEASED</Name>
      <String>1999</String>
    </Simple>
    <Simple>
      <Name>IMDB</Name>
      <String>tt0133093</String>
    </Simple>
    <Simple>
      <Name>TMDB</Name>
      <String>movie/603</String>
    </Simple>
    <Simple>
      <Name>TVDB2</Name>
      <String>movies/174</String>
    </Simple>
  </Tag>
</Tags>
```

For a **TV Episode**, the default template beautifully separates the metadata into a hierarchy of Show (70), Season (60), and Episode (50) target levels:

```xml
<Tags>
  <Tag>
    <Targets>
      <TargetTypeValue>70</TargetTypeValue>
    </Targets>
    <Simple>
      <Name>TITLE</Name>
      <String>Breaking Bad</String>
    </Simple>
    <Simple>
      <Name>DATE_RELEASED</Name>
      <String>2008</String>
    </Simple>
    <Simple>
      <Name>IMDB</Name>
      <String>tt0903747</String>
    </Simple>
    <Simple>
      <Name>TMDB</Name>
      <String>tv/1396</String>
    </Simple>
    <Simple>
      <Name>TVDB</Name>
      <String>81189</String>
    </Simple>
    <Simple>
      <Name>TVDB2</Name>
      <String>series/81189</String>
    </Simple>
  </Tag>
  <Tag>
    <Targets>
      <TargetTypeValue>60</TargetTypeValue>
    </Targets>
    <Simple>
      <Name>PART_NUMBER</Name>
      <String>5</String>
    </Simple>
    <Simple>
      <Name>TOTAL_PARTS</Name>
      <String>16</String>
    </Simple>
  </Tag>
  <Tag>
    <Targets>
      <TargetTypeValue>50</TargetTypeValue>
    </Targets>
    <Simple>
      <Name>TITLE</Name>
      <String>Ozymandias</String>
    </Simple>
    <Simple>
      <Name>PART_NUMBER</Name>
      <String>14</String>
    </Simple>
    <Simple>
      <Name>DATE_RELEASED</Name>
      <String>2013-09-15</String>
    </Simple>
    <Simple>
      <Name>IMDB</Name>
      <String>tt2301451</String>
    </Simple>
    <Simple>
      <Name>TVDB2</Name>
      <String>episodes/4599981</String>
    </Simple>
  </Tag>
</Tags>
```

## Available Data

Templates have access to these core objects under the context root:

- `.Media`: Contains the matched metadata from the database.
- `.Episode`: Contains detailed episode information (only populated if the target is a TV episode).
- `.Comment`: The user-provided string from the `--comment` flag.
- `.ReleaseName`: The name of the file being processed (without the extension).

### `.Media` (Always present)
Contains the metadata of the matched Movie or TV Show. Key fields include:
- `.Media.Title` (string)
- `.Media.OriginalTitle` (string)
- `.Media.AltTitle` ([]string)
- `.Media.Overview` (string)
- `.Media.Genres` ([]string)
- `.Media.Year` (int)
- `.Media.OriginalLanguage` (string)
- `.Media.TmdbID` (int)
- `.Media.TmdbType` (string)
- `.Media.ImdbID` (string)
- `.Media.TvdbID` (int)
- `.Media.TvdbType` (string)
- `.Media.TvdbSlug` (string)
- `.Media.IsTV` (bool)

### `.Episode` (Present only for TV Episodes)
If the file being tagged is an episode, `.Episode` contains its data. Otherwise, `.Episode` is `nil`.
- `.Episode.Name` (string)
- `.Episode.Overview` (string)
- `.Episode.Season` (int)
- `.Episode.Episode` (int)
- `.Episode.Airdate` (string)
- `.Episode.TvdbID` (int)
- `.Episode.TotalEpisodes` (int)
- `.Episode.ImdbID` (string)

## Template Functions

In addition to standard Go `text/template` built-ins (like `and`, `or`, `eq`), Parsec bundles several helpful custom functions for formatting your tags:

### String Manipulation
*   `join`: Joins a string array with a separator. Example: `{{join .Media.Genres ", "}}`
*   `upper`: Converts a string to uppercase. Example: `{{upper .ReleaseName}}`
*   `lower`: Converts a string to lowercase. Example: `{{lower .Media.Title}}`
*   `title`: Converts a string to Title Case. Example: `{{title .Comment}}`
*   `replace`: Replaces all occurrences of a string. Example: `{{replace .ReleaseName "." " "}}`
*   `trim`: Removes leading and trailing whitespace. Example: `{{trim .Media.Overview}}`

### Mathematical Operations
*   `add`: Adds two integers. Example: `{{add .Episode.Episode 1}}`
*   `sub`: Subtracts two integers. Example: `{{sub .Episode.Season 1}}`
*   `mul`: Multiplies two integers. Example: `{{mul .Episode.TotalEpisodes 2}}`
*   `div`: Divides two integers. Example: `{{div .Media.Year 10}}`

## Official Matroska Tags

Matroska defines [official tag names](https://www.matroska.org/technical/tagging.html) for standard metadata. Parsec maps to any of them freely via your TOML configuration. Here are the most commonly used official tags:

*   **`TITLE`**: The title of the entity (e.g. Movie title, Show title, or Episode name).
*   **`DATE_RELEASED`**: The release year or specific airdate.
*   **`PART_NUMBER`**: The episode or season number.
*   **`TOTAL_PARTS`**: The total episodes in the season.
*   **`SUMMARY`**: A description or overview of the movie/episode.
*   **`COMMENT`**: Custom notes, repack reasons, or scene release information.
*   **`IMDB`**, **`TMDB`**, **`TVDB`**, **`TVDB2`**: Official database identifier tags.

Unofficial database IDs (like `WIKIDATA`) are widely adopted by the community and can be safely written exactly as uppercase custom keys.

## Dynamic Target Values

The `target_value` itself can also be a template string! This is incredibly useful for writing a single tag block that dynamically applies to the Show level (70) if it's a TV show, or the Movie level (50) if it's a movie:

```toml
[[tags]]
target_value = "{{if .Media.IsTV}}70{{else}}50{{end}}"
[tags.fields]
TITLE = "{{.Media.Title}}"
DATE_RELEASED = "{{.Media.Year}}"
```

## Handling Empty Values

If a template string evaluates to an empty string `""` after execution, Parsec will automatically skip writing that tag to the Matroska file. 

For simple mappings like `IMDB = "{{.Media.ImdbID}}"`, no `if` blocks are required. If `.Media.ImdbID` is missing, the template outputs `""`, and the tag is not written.

However, if you combine variables with static text, you should use `{{if}}` to avoid writing partial strings (e.g. `movie/0`).

```toml
# Bad (Might write "movie/0" if TmdbID is empty/0)
TMDB = "movie/{{.Media.TmdbID}}"

# Good
TMDB = "{{if .Media.TmdbID}}movie/{{.Media.TmdbID}}{{end}}"
```

## Presets Support

The `tag_template` option supports Parsec's built-in preset system. You can easily switch between tag schemes on the fly:

```toml
# parsec.toml
[preset.anime]
tag_template = "anime_tags"
```

Running `parsec identify --preset anime` will automatically load `tags/anime_tags.toml`.
