package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"codeberg.org/n0ne/parsec/internal/checks"
	"codeberg.org/n0ne/parsec/internal/metadata"
	"codeberg.org/n0ne/parsec/internal/metadata/filename"
	"codeberg.org/n0ne/parsec/internal/metadata/matroska"
	"codeberg.org/n0ne/parsec/internal/metadata/mediainfo"
	"codeberg.org/n0ne/parsec/internal/types"
	"codeberg.org/n0ne/parsec/internal/ui"

	"github.com/spf13/cobra"
)

type issueGroup struct {
	Category string               `json:"category"`
	Results  []checks.CheckResult `json:"results"`
}

type checkReport struct {
	File          string       `json:"file"`
	Passed        bool         `json:"passed"`
	ReleaseName   string       `json:"filename"`
	GeneratedName string       `json:"generated_name"`
	Issues        []issueGroup `json:"issues"`
}

var jsonOutputFlag bool

var checkCmd = &cobra.Command{
	Use:   "check [file]",
	Short: "Check if the file fits the specification",
	Long: fmt.Sprintf("%s\n%s", ui.Banner(".: VERIFY INTEGRITY :."),
		`Performs comprehensive integrity and consistency checks on a media file.
It validates:
  1. Filename parsing and naming conventions
  2. Technical metadata (via MediaInfo) for quality and standards
  3. Matroska container integrity and track tagging
  4. Consistency with online databases (TMDB/TVDB) for titles and episodes`),
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]

		report, err := collectCheckData(cmd, filePath)
		if err != nil {
			ui.PrintError(err.Error())
			return fmt.Errorf("collecting check data failed")
		}

		if jsonOutputFlag {
			printJSONReport(report)
		} else {
			printInteractiveReport(report)
		}
		return nil
	},
}

func collectCheckData(cmd *cobra.Command, filePath string) (checkReport, error) {
	filenameNoExt := filename.GetBaseName(filePath)

	ui.Println(ui.Banner(".: INTEGRITY VERIFICATION :."))
	ui.Println(ui.LabelValue("Target Name:", filenameNoExt))

	match := filename.Parse(filenameNoExt)
	mi, err := mediainfo.Get(filePath)
	if err != nil {
		return checkReport{}, fmt.Errorf("error getting mediainfo: %v", err)
	}

	mediaMeta := mi.GetMetadata()
	match.Override(mediaMeta)

	ebml, ebmlErr := matroska.GetEbmlMetadata(filePath)
	if ebmlErr == nil {
		if ebml.HasVisualImpairedAudio() && !match.HasAudioDesc {
			match.HasAudioDesc = true
		}
	}

	setupMdbIDs(cmd, mi, match)

	// Run Checks
	var allIssues []issueGroup
	appendFailed(&allIssues, "FILENAME", checks.RunFilenameChecks(filenameNoExt, match))
	appendFailed(&allIssues, "MDB", checks.RunMdbChecks(mi, match))
	appendFailed(&allIssues, "MEDIAINFO", checks.RunMediaInfoChecks(mi, match))
	appendFailed(&allIssues, "MATROSKA", checks.RunMatroskaChecks(filePath))

	return checkReport{
		File:          filePath,
		Passed:        len(allIssues) == 0,
		ReleaseName:   filenameNoExt,
		GeneratedName: match.GetReleaseName(),
		Issues:        allIssues,
	}, nil
}

func appendFailed(allIssues *[]issueGroup, category string, results []checks.CheckResult) {
	var failed []checks.CheckResult
	for _, r := range results {
		if !r.Passed {
			failed = append(failed, r)
		}
	}
	if len(failed) > 0 {
		*allIssues = append(*allIssues, issueGroup{category, failed})
	}
}

func setupMdbIDs(cmd *cobra.Command, mi *mediainfo.MediaInfo, match *metadata.Metadata) {
	tagImdb, tagTmdb, tagTvdb, tagIsTV := mi.GetMdbIDs()
	if match.ImdbID == "" {
		match.ImdbID = tagImdb
	}
	if match.TmdbID == 0 {
		match.TmdbID = tagTmdb
	}
	if match.TvdbID == 0 {
		match.TvdbID = tagTvdb
	}
	if !cmd.Flags().Changed("tv") && !cmd.Flags().Changed("movie") && tagIsTV {
		match.IsTV = true
	}

	if imdbIDFlag != "" {
		match.ImdbID = imdbIDFlag
	}
	if tmdbIDFlag != 0 {
		match.TmdbID = tmdbIDFlag
	}
	if tvdbIDFlag != 0 {
		match.TvdbID = tvdbIDFlag
	}
}

func printJSONReport(report checkReport) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		ui.PrintError(fmt.Sprintf("Error generating output: %v", err))
		os.Exit(1)
	}
	fmt.Println(string(data))
}

func printInteractiveReport(report checkReport) {
	if report.Passed {
		ui.Println("\n" + ui.IconCheck + ui.Success.Render(" All systems nominal! The file fits the specification."))
		ui.Println()
		return
	}

	// Calculate shared column widths across all tracks to ensure table alignment
	var allTracks []types.TrackCheckResult
	for _, group := range report.Issues {
		for _, res := range group.Results {
			allTracks = append(allTracks, res.Tracks...)
		}
	}
	sharedWidths := ui.CalculateTrackTableWidths(allTracks)

	totalIssues := countIssues(report.Issues)
	ui.Println("\n" + ui.IconCross + ui.Error.Render(fmt.Sprintf(" %d issues found:", totalIssues)))
	for _, group := range report.Issues {
		count := len(group.Results)
		if !unattendedFlag {
			if !ui.ConfirmContinue(fmt.Sprintf("\nDisplay %d %s issues?", count, group.Category)) {
				return
			}
		}

		ui.Println(ui.ReportSection(fmt.Sprintf("%s (%d)", group.Category, count)))
		for _, res := range group.Results {
			switch res.Severity {
			case "error":
				ui.PrintError(res.Warning)
			default:
				ui.PrintWarning(res.Warning)
			}
			if len(res.Tracks) == 0 {
				printUnexpectedDiff(res)
				continue
			}

			ui.Println(ui.FormatTrackTable(res.Tracks, sharedWidths))
		}
	}
	ui.Println()
}

func printUnexpectedDiff(res checks.CheckResult) {
	if res.Expected != "" && res.Actual != "" {
		labelE := "Expected"
		labelA := "Actual"

		// Use Official/Parsed for all MDB related checks
		if strings.HasPrefix(res.Identifier, "mdb_") {
			labelE, labelA = "Official", "Parsed"
		} else if res.Identifier == "filename_generation_mismatch" {
			labelE, labelA = "Original", "Generated"
		}
		indent := "   "
		diff := ui.FormatStringDiffAligned(labelE, res.Expected, labelA, res.Actual)
		indentedDiff := indent + strings.ReplaceAll(diff, "\n", "\n"+indent)
		ui.Println(indentedDiff)
	}
}

func countIssues(groups []issueGroup) int {
	count := 0
	for _, group := range groups {
		count += len(group.Results)
	}
	return count
}

func init() {
	rootCmd.AddCommand(checkCmd)
	checkCmd.Flags().IntVar(&tmdbIDFlag, "tmdb", 0, "TMDB ID")
	checkCmd.Flags().IntVar(&tvdbIDFlag, "tvdb", 0, "TVDB ID")
	checkCmd.Flags().StringVar(&imdbIDFlag, "imdb", "", "IMDb ID")
	checkCmd.Flags().BoolVarP(&jsonOutputFlag, "json", "j", false, "Output check results in JSON")
	checkCmd.Flags().BoolVarP(&unattendedFlag, "unattended", "u", false, "Do not prompt for confirmation")
	checkCmd.Flags().BoolVar(&verboseFlag, "verbose", false, "Verbose output")

	idFlags := []string{"imdb", "tmdb", "tvdb"}
	for _, f := range idFlags {
		_ = checkCmd.Flags().SetAnnotation(f, "group", []string{"id"})
	}
}
