package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"codeberg.org/upPollo/parsec/internal/checks"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/filename"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/metadata/mediainfo"
	"codeberg.org/upPollo/parsec/internal/types"
	"codeberg.org/upPollo/parsec/internal/ui"
)

var jsonOutputFlag bool

var checkCmd = &cobra.Command{
	Use:   "check [path...]",
	Short: "Check if files fit the specification",
	Long: fmt.Sprintf("%s\n%s", ui.Banner(".: VERIFY INTEGRITY :."),
		`Performs comprehensive integrity and consistency checks on media files.
It validates:
  1. Filename parsing and naming conventions
  2. Technical metadata (via MediaInfo) for quality and standards
  3. Matroska container integrity and track tagging
  4. Consistency with online databases (TMDB/TVDB) for titles and episodes

You can pass files or directories. Directories are scanned recursively for Matroska files.
You can also pass a JSON check report file to render it.`),
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.IsSilent = jsonOutputFlag

		var allReports []types.CheckReport

		expandedArgs := expandArgs(args)

		for _, filePath := range expandedArgs {
			var currentReports []types.CheckReport

			isJSON, jsonReports := loadJSONReport(filePath)

			if isJSON {
				currentReports = jsonReports
			} else {
				report, err := collectCheckData(cmd, filePath)
				if err != nil {
					ui.PrintError(err.Error())
					return fmt.Errorf("collecting check data failed for %s", filePath)
				}

				currentReports = []types.CheckReport{report}
			}

			allReports = append(allReports, currentReports...)

			if !jsonOutputFlag {
				for _, r := range currentReports {
					ui.PrintInteractiveReport(r, unattendedFlag)
				}
			}
		}

		if jsonOutputFlag {
			printJSONReports(allReports)
		}

		return nil
	},
}

func loadJSONReport(filePath string) (bool, []types.CheckReport) {
	f, err := os.Open(filePath)
	if err != nil {
		return false, nil
	}
	defer func() { _ = f.Close() }()

	// Read a small header to see if it even looks like JSON
	header := make([]byte, 512)

	n, err := f.Read(header)
	if err != nil || n == 0 {
		return false, nil
	}

	trimmedHeader := strings.TrimLeft(string(header[:n]), " \t\r\n")
	if len(trimmedHeader) == 0 || (trimmedHeader[0] != '{' && trimmedHeader[0] != '[') {
		return false, nil
	}

	// It's likely JSON, read the whole thing for parsing
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false, nil
	}

	reports, err := parseReports(data)
	if err != nil {
		return false, nil
	}

	return true, reports
}

func parseReports(data []byte) ([]types.CheckReport, error) {
	var reports []types.CheckReport
	if err := json.Unmarshal(data, &reports); err != nil {
		// Try unmarshaling a single report
		var singleReport types.CheckReport
		if err := json.Unmarshal(data, &singleReport); err != nil {
			return nil, err
		}

		reports = append(reports, singleReport)
	}

	return reports, nil
}

func collectCheckData(cmd *cobra.Command, filePath string) (types.CheckReport, error) {
	filenameNoExt := filename.GetBaseName(filePath)

	ui.Println(ui.Banner(".: INTEGRITY VERIFICATION :."))
	ui.Println(ui.LabelValue("Target Name:", filenameNoExt))

	match := filename.Parse(filenameNoExt)

	mi, err := mediainfo.Get(filePath)
	if err != nil {
		return types.CheckReport{}, fmt.Errorf("error getting mediainfo: %w", err)
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
	var allIssues []types.IssueGroup
	appendFailed(&allIssues, "FILENAME", checks.RunFilenameChecks(filenameNoExt, match))
	appendFailed(&allIssues, "MDB", checks.RunMdbChecks(mi, match))
	appendFailed(&allIssues, "MEDIAINFO", checks.RunMediaInfoChecks(mi, match))
	appendFailed(&allIssues, "MATROSKA", checks.RunMatroskaChecks(filePath))

	return types.CheckReport{
		File:          filePath,
		Passed:        len(allIssues) == 0,
		ReleaseName:   filenameNoExt,
		GeneratedName: match.GetReleaseName(),
		Version:       Version,
		Issues:        allIssues,
	}, nil
}

func appendFailed(allIssues *[]types.IssueGroup, category string, results []checks.CheckResult) {
	var failed []checks.CheckResult

	for _, r := range results {
		if !r.Passed {
			failed = append(failed, r)
		}
	}

	if len(failed) > 0 {
		*allIssues = append(*allIssues, types.IssueGroup{Category: category, Results: failed})
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

func printJSONReports(reports []types.CheckReport) {
	data, err := json.MarshalIndent(reports, "", "  ")
	if err != nil {
		ui.PrintError(fmt.Sprintf("Error generating output: %v", err))
		os.Exit(1)
	}

	fmt.Println(string(data))
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
