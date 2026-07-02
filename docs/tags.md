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

## Available Data

Templates have access to two core objects under the context root:

### `.Comment` (String)
The comment provided via the `--comment` CLI flag, allowing you to manually inject data (like a repack reason) into your tags.

### `.Media` (Always present)
Contains the metadata of the matched Movie or TV Show. Key fields include:
- `.Media.Title` (string)
- `.Media.OriginalTitle` (string)
- `.Media.TmdbID` (int)
- `.Media.ImdbID` (string)
- `.Media.TvdbID` (int)
- `.Media.IsTV` (bool)

### `.Episode` (Present only for TV Episodes)
If the file being tagged is an episode, `.Episode` contains its data. Otherwise, `.Episode` is `nil`.
- `.Episode.Name` (string)
- `.Episode.Season` (int)
- `.Episode.Episode` (int)
- `.Episode.Airdate` (string)
- `.Episode.TvdbID` (int)
- `.Episode.TotalEpisodes` (int)
- `.Episode.ImdbID` (string)

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
