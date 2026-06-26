package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	fixer "codeberg.org/upPollo/parsec/internal/correct"
	"codeberg.org/upPollo/parsec/internal/ui"
)

var remuxFlag bool

// correctCmd represents the correct command
var correctCmd = &cobra.Command{
	Use:   "correct [path...]",
	Short: "automatically fix Matroska track issues reported by check",
	Long: fmt.Sprintf("%s\n%s", ui.Banner(".: COURSE CORRECTION :."),
		`Automatically repairs Matroska issues that can be fixed without re-encoding:
  1. Track flags (default and original-language)
  2. Track names (removing codecs, junk and redundant language tags)
  3. Missing language tags, multi-language names and keyword/flag mismatches (prompted)
  4. Container metadata and font attachments (prompted)

With --remux, also applies fixes that require rewriting the container: track
order, container compression and (with confirmation) removal of unwanted-language
or empty audio tracks when enough metadata is available.

Issues that require re-encoding (bitrate, resolution, framerate, ...) or human
judgement are left untouched and should be resolved manually. Filename fixes are
handled by the rename command.

You can pass files or directories. Directories are scanned recursively for Matroska files.`),
	Args: cobra.MinimumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		ui.Println(ui.Banner(".: COURSE CORRECTION :."))

		if originalLanguageFlag != "" {
			viper.Set("original_language", originalLanguageFlag)
		}

		expandedArgs := expandArgs(args)
		for _, filePath := range expandedArgs {
			if err := correctFile(filePath); err != nil {
				return err
			}
		}

		return nil
	},
}

func correctFile(filePath string) error {
	return fixer.ApplyFile(filePath, fixer.Options{
		DryRun:     dryRunFlag,
		Remux:      remuxFlag,
		Unattended: unattendedFlag,
		ImdbID:     imdbIDFlag,
		TmdbID:     tmdbIDFlag,
		TvdbID:     tvdbIDFlag,
	})
}

func init() {
	rootCmd.AddCommand(correctCmd)

	correctCmd.Flags().BoolVar(&remuxFlag, "remux", false, "also apply fixes that require rewriting the container (track order, compression, track removal)")
	correctCmd.Flags().BoolVarP(&unattendedFlag, "unattended", "u", false, "do not prompt for confirmation")
	correctCmd.Flags().BoolVarP(&dryRunFlag, "dry-run", "d", false, "preview changes without modifying files")
	correctCmd.Flags().StringVar(&originalLanguageFlag, "original-language", "", "override original language (sets original_language config key)")
	correctCmd.Flags().StringVar(&imdbIDFlag, "imdb", "", "IMDb ID")
	correctCmd.Flags().IntVar(&tmdbIDFlag, "tmdb", 0, "TMDB ID")
	correctCmd.Flags().IntVar(&tvdbIDFlag, "tvdb", 0, "TVDB ID")

	for _, f := range []string{"imdb", "tmdb", "tvdb"} {
		_ = correctCmd.Flags().SetAnnotation(f, "group", []string{"id"})
	}

	correctCmd.Flags().SortFlags = false
}
