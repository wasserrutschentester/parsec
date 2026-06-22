package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	fixer "codeberg.org/upPollo/parsec/internal/fix"
	"codeberg.org/upPollo/parsec/internal/ui"
)

// fixCmd represents the fix command
var fixCmd = &cobra.Command{
	Use:   "fix [path...]",
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

		expandedArgs := expandArgs(args)
		for _, filePath := range expandedArgs {
			if err := fixFile(filePath); err != nil {
				return err
			}
		}

		return nil
	},
}

func fixFile(filePath string) error {
	return fixer.ApplyFile(filePath, fixer.Options{
		DryRun:           dryRunFlag,
		Remux:            remuxFlag,
		Unattended:       unattendedFlag,
		OriginalLanguage: ovFlag,
		ImdbID:           imdbIDFlag,
		TmdbID:           tmdbIDFlag,
		TvdbID:           tvdbIDFlag,
	})
}

func init() {
	rootCmd.AddCommand(fixCmd)

	fixCmd.Flags().BoolVar(&remuxFlag, "remux", false, "also apply fixes that require rewriting the container (track order, compression, track removal)")
	fixCmd.Flags().BoolVarP(&unattendedFlag, "unattended", "u", false, "do not prompt for confirmation")
	fixCmd.Flags().BoolVarP(&dryRunFlag, "dry-run", "d", false, "preview changes without modifying files")
	fixCmd.Flags().StringVar(&ovFlag, "ov", "", "override MDB original language/OV (2- or 3-letter code)")
	fixCmd.Flags().StringVar(&imdbIDFlag, "imdb", "", "IMDb ID")
	fixCmd.Flags().IntVar(&tmdbIDFlag, "tmdb", 0, "TMDB ID")
	fixCmd.Flags().IntVar(&tvdbIDFlag, "tvdb", 0, "TVDB ID")

	for _, f := range []string{"imdb", "tmdb", "tvdb"} {
		_ = fixCmd.Flags().SetAnnotation(f, "group", []string{"id"})
	}

	fixCmd.Flags().SortFlags = false
}
