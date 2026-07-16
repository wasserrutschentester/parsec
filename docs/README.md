# Parsec Documentation

Welcome to the Parsec documentation. Below is an index of all available guides to help you configure, run, and customize Parsec.

## Getting Started
* [Configuration and Presets](config.md) - Learn how to set up your `.toml` configuration and use presets.
* [Docker Guide](docker.md) - Instructions for running Parsec via Docker (temporary or persistent containers).
* [Shell Completion](completion.md) - Generate autocompletion scripts for your terminal.
* [Update Command](update.md) - How to perform in-place updates of the Parsec binary.

## Core Commands
* [Check Command](checks.md) - Perform comprehensive integrity and consistency checks on media files to ensure they meet your specifications.
* [Identify Command](identify.md) - Search for and identify movies or TV shows using external databases (TMDB, TVDB, IMDb).
* [Rename Command](rename.md) - Automatically rename files using metadata extracted from the file, MediaInfo, and external databases.
* [Generate NFO](nfogen.md) - Generate rich XML or text NFO files by querying databases and applying templates.

## Advanced Customization & Templating
* [Naming Templates](templates.md) - Guide to Parsec's flexible placeholder token system for renaming and verifying filenames.
* [Regex Replacements](replacements.md) - Define custom regex replacement rules for the input, title, and output stages.
* [Tag Templates](tags.md) - Customize how scraped metadata is translated and written to Matroska MKV tags.
* [NFO Templating System](nfo_templating.md) - Learn how to build custom NFO templates using Go's `text/template` engine.
* [NFO Context Reference](nfo_context.md) - A complete reference guide of all the variables and data fields available for use inside your NFO templates.
