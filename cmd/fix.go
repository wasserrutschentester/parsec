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

With --remux, also applies fixes that require rewriting the container: track
order, container compression and (with confirmation) removal of duplicate or
unwanted-language tracks when enough MDB data is available.

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
	registerFixFlags(fixCmd)
}

func registerFixFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&remuxFlag, "remux", false, "also apply fixes that require rewriting the container (track order, compression, track removal)")
	cmd.Flags().BoolVarP(&unattendedFlag, "unattended", "u", false, "do not prompt for confirmation")
	cmd.Flags().BoolVarP(&dryRunFlag, "dry-run", "d", false, "preview changes without modifying files")
	cmd.Flags().StringVar(&ovFlag, "ov", "", "override MDB original language/OV (2- or 3-letter code)")
	cmd.Flags().StringVar(&imdbIDFlag, "imdb", "", "IMDb ID")
	cmd.Flags().IntVar(&tmdbIDFlag, "tmdb", 0, "TMDB ID")
	cmd.Flags().IntVar(&tvdbIDFlag, "tvdb", 0, "TVDB ID")

	for _, f := range []string{"imdb", "tmdb", "tvdb"} {
		_ = cmd.Flags().SetAnnotation(f, "group", []string{"id"})
	}

	cmd.Flags().SortFlags = false
}
