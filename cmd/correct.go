package cmd

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	fixer "codeberg.org/upPollo/parsec/internal/correct"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/ui"
)

var remuxFlag bool

type correctResult struct {
	File string         `json:"file"`
	Plan *fixer.FixPlan `json:"plan"`
}

var errJSONRequiresDryRunOrUnattended = errors.New("json output mode requires --dry-run or --unattended")

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
		if jsonOutputFlag && !dryRunFlag && !unattendedFlag {
			return errJSONRequiresDryRunOrUnattended
		}

		if jsonOutputFlag {
			ui.IsSilent = true
			ui.IsJSON = true
		} else {
			ui.Println(ui.Banner(".: COURSE CORRECTION :."))
		}

		if originalLanguageFlag != "" {
			viper.Set("original_language", originalLanguageFlag)
		}

		var results []correctResult

		expandedArgs := expandArgs(args)
		for _, filePath := range expandedArgs {
			plan, err := correctFile(filePath)
			if err != nil {
				return err
			}

			if plan != nil {
				results = append(results, correctResult{File: filePath, Plan: plan})
			}
		}

		if jsonOutputFlag {
			//nolint:musttag // FixPlan doesn't have json tags but default marshaling is acceptable
			data, err := json.MarshalIndent(results, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal json: %w", err)
			}

			fmt.Println(string(data))
		}

		return nil
	},
}

//nolint:cyclop // branching for json output adds minor complexity
func correctFile(filePath string) (*fixer.FixPlan, error) {
	opts := fixer.Options{
		DryRun:     dryRunFlag,
		Remux:      remuxFlag,
		Unattended: unattendedFlag,
		ImdbID:     imdbIDFlag,
		TmdbID:     tmdbIDFlag,
		TvdbID:     tvdbIDFlag,
	}

	if !jsonOutputFlag {
		ui.Println(ui.LabelValue("Target Name:", filePath))
	}

	plan, err := fixer.PlanFile(filePath, opts)
	if err != nil {
		return nil, err
	}

	if jsonOutputFlag {
		if !opts.DryRun {
			if err := fixer.ExecutePlan(filePath, plan); err != nil {
				return nil, err
			}
		}

		return plan, nil
	}

	if err := fixer.AppendInteractiveTrackEdits(filePath, plan, opts); err != nil {
		return nil, err
	}

	ebml, err := matroska.GetEbmlMetadata(filePath)
	if err != nil {
		// Log a warning or just pass nil to ReviewPlan? PlanFile probably already warned/errored.
		ebml = nil
	}

	if fixer.ReviewPlan(plan, ebml, opts) {
		if opts.DryRun {
			ui.PrintSuccess("Dry-run complete. No files were modified.")

			return plan, nil
		}

		if err := fixer.ExecutePlan(filePath, plan); err != nil {
			return nil, err
		}

		ui.PrintSuccess("Fixes applied.")
	}

	return plan, nil
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

	correctCmd.Flags().BoolVarP(&jsonOutputFlag, "json", "j", false, "output fix plan in JSON (requires --dry-run or --unattended)")
	correctCmd.Flags().SortFlags = false
}
