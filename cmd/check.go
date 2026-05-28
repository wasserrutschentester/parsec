package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"codeberg.org/n0ne/parsec/internal/checks"
	"codeberg.org/n0ne/parsec/internal/metadata"
	"codeberg.org/n0ne/parsec/internal/metadata/filename"
	"codeberg.org/n0ne/parsec/internal/metadata/matroska"
	"codeberg.org/n0ne/parsec/internal/metadata/mediainfo"
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
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ui.IsSilent = jsonOutputFlag
		filePath := args[0]

		report, err := collectCheckData(cmd, filePath)
		if err != nil {
			ui.PrintError(err.Error())
			return
		}

		if jsonOutputFlag {
			printJSONReport(report)
		} else {
			printInteractiveReport(report)
		}
	},
}

func collectCheckData(cmd *cobra.Command, filePath string) (checkReport, error) {
	filenameNoExt := filename.GetBaseName(filePath)

	ui.Println(ui.Header.Render("Parsec File Check"))
	ui.Println(ui.LabelValue("Current Name:", filenameNoExt))

	var allIssues []issueGroup

	// 1. Filename Basic Checks
	match := filename.Parse(filenameNoExt)
	appendFailed(&allIssues, "FILENAME", checks.RunFilenameChecks(filenameNoExt, match))

	// 2. MediaInfo & EBML Checks
	mi, err := mediainfo.Get(filePath)
	if err != nil {
		return checkReport{}, fmt.Errorf("error getting mediainfo: %v", err)
	}
	mediaMeta := mi.GetMetadata()
	updated := match.Override(mediaMeta)

	ebml, ebmlErr := matroska.GetEbmlMetadata(filePath)
	if ebmlErr == nil {
		if ebml.HasVisualImpairedAudio() && !match.HasAudioDesc {
			match.HasAudioDesc = true
			updated = true
		}
	}

	if updated {
		ui.Println(ui.Info.Render("\nUpdates applied from MediaInfo/EBML:"))
		ui.Println(ui.LabelValue("Generated Name:", match.String()))
	}

	appendFailed(&allIssues, "MEDIAINFO", checks.RunMediaInfoChecks(mi, match))
	appendFailed(&allIssues, "MATROSKA", checks.RunMatroskaChecks(filePath))

	// 3. Generic Checks
	appendFailed(&allIssues, "GENERIC", checks.RunGenericChecks(match))

	// 4. MDB Checks
	setupMdbIDs(cmd, mi, match)
	appendFailed(&allIssues, "MDB", checks.RunMdbChecks(mi, match))

	return checkReport{
		File:          filePath,
		Passed:        len(allIssues) == 0,
		ReleaseName:   filenameNoExt,
		GeneratedName: match.String(),
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
		ui.Println("\n" + ui.IconCheck + ui.Success.Render(" All checks passed! The file fits the specification."))
		ui.Println()
		return
	}

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
			if len(res.Tracks) == 0 {
				ui.Println("  " + ui.IconWarn + " " + res.Warning)
				continue
			}

			ui.Println("  " + ui.IconWarn + " " + res.Description)
			ui.Println(ui.FormatTrackTable(res.Tracks))
		}
	}
	ui.Println()
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
		checkCmd.Flags().SetAnnotation(f, "group", []string{"id"})
	}
}
