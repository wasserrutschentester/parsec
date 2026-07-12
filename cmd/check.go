package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"codeberg.org/upPollo/parsec/internal/checks"
	mdbSearch "codeberg.org/upPollo/parsec/internal/mdb/search"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/filename"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/metadata/mediainfo"
	"codeberg.org/upPollo/parsec/internal/types"
	"codeberg.org/upPollo/parsec/internal/ui"
)

var (
	jsonOutputFlag        bool
	individualReportsFlag bool
	jobsFlag              int
	originalLanguageFlag  string
)

type seasonKey struct {
	tvdbID int
	tmdbID int
	season int
}

var errCheckDataCollection = errors.New("collecting check data failed")

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
	ValidArgsFunction: func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// mkv for media files, json for existing check report files
		return completeFiles(toComplete, "mkv", "json")
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.IsSilent = jsonOutputFlag
		ui.IsJSON = jsonOutputFlag

		if originalLanguageFlag != "" {
			viper.Set("original_language", originalLanguageFlag)
		}

		var allReports []types.CheckReport

		expandedArgs := expandArgs(args)
		batchMode := !individualReportsFlag && len(expandedArgs) > 1

		if len(expandedArgs) == 1 {
			isJSON, jsonReports := loadJSONReport(expandedArgs[0])
			if isJSON && len(jsonReports) > 1 {
				batchMode = !individualReportsFlag
			}
		}

		seasonEpisodes := make(map[seasonKey][]int)
		seasonMetas := make(map[seasonKey]*metadata.Metadata)

		ui.Println(ui.Banner(".: INTEGRITY VERIFICATION :."))

		numJobs := getNumJobs(len(expandedArgs))
		results := runChecksInParallel(cmd, expandedArgs, numJobs, batchMode)

		for i, filePath := range expandedArgs {
			res := results[i]

			if res.err != nil {
				ui.PrintError(res.err.Error())

				return fmt.Errorf("%w for %s", errCheckDataCollection, filePath)
			}

			if !batchMode && !jsonOutputFlag {
				filenameNoExt := filename.GetBaseName(filePath)

				ui.Println("\n" + ui.Header.Render("VERIFYING NEW TARGET"))
				ui.Println(filenameNoExt)
			}

			allReports = append(allReports, res.reports...)
			meta := res.meta

			if meta != nil && meta.IsTV && (meta.TvdbID > 0 || meta.TmdbID > 0) && meta.Season > 0 {
				key := seasonKey{tvdbID: meta.TvdbID, tmdbID: meta.TmdbID, season: meta.Season}

				if len(meta.Episodes) > 0 {
					seasonEpisodes[key] = append(seasonEpisodes[key], meta.Episodes...)
				}

				seasonMetas[key] = meta
			}

			if !jsonOutputFlag && !batchMode {
				for _, r := range res.reports {
					ui.PrintInteractiveReport(r, unattendedFlag)
				}
			}
		}

		if !jsonOutputFlag && batchMode {
			ui.PrintAggregatedSummary(allReports, unattendedFlag)
		}

		// Run aggregate season checks
		if !jsonOutputFlag {
			runSeasonCompletenessChecks(seasonEpisodes, seasonMetas)
		}

		if jsonOutputFlag {
			printJSONReports(allReports)
		}

		return nil
	},
}

func runSeasonCompletenessChecks(seasonEpisodes map[seasonKey][]int, seasonMetas map[seasonKey]*metadata.Metadata) {
	for key, episodes := range seasonEpisodes {
		// 1. Skip Season 0 (Specials)
		if key.season == 0 {
			continue
		}

		// 2. Skip if only one episode was provided
		if len(episodes) <= 1 {
			continue
		}

		ui.Println("\n" + ui.Header.Render("AGGREGATE CHECK: SEASON COMPLETENESS"))

		meta := seasonMetas[key]
		res, err := mdbSearch.InteractiveSearch(meta, true)

		if err == nil && res != nil {
			completenessResults := checks.RunSeasonCompletenessCheck(res, key.season, episodes)
			for _, r := range completenessResults {
				if !r.Passed {
					ui.Println(ui.FormatWarning(r.Warning))
				} else {
					ui.Println(ui.Success.Render(fmt.Sprintf("Season %d is complete (%d episodes).", key.season, len(episodes))))
				}
			}
		}
	}
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
			return nil, fmt.Errorf("failed to unmarshal single report: %w", err)
		}

		reports = append(reports, singleReport)
	}

	return reports, nil
}

func collectCheckData(cmd *cobra.Command, filePath string, showIndividual bool) (types.CheckReport, *metadata.Metadata, error) {
	filenameNoExt := filename.GetBaseName(filePath)

	if showIndividual {
		ui.Println("\n" + ui.Header.Render("VERIFYING NEW TARGET"))
		ui.Println(filenameNoExt)
	} else {
		ui.Println(fmt.Sprintf("Checking %s...", filenameNoExt))
	}

	match := filename.Parse(filenameNoExt)

	mi, err := mediainfo.Get(filePath)
	if err != nil {
		return types.CheckReport{}, nil, fmt.Errorf("error getting mediainfo: %w", err)
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
	appendFailed(&allIssues, "MATROSKA", checks.RunMatroskaChecks(filePath, ebml, ebmlErr, match))

	return types.CheckReport{
		File:          filePath,
		Passed:        len(allIssues) == 0,
		ReleaseName:   filenameNoExt,
		GeneratedName: match.GetReleaseName(),
		Version:       Version,
		Issues:        allIssues,
	}, match, nil
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
	checkCmd.Flags().StringVar(&originalLanguageFlag, "original-language", "", "Override original language")
	checkCmd.Flags().BoolVarP(&jsonOutputFlag, "json", "j", false, "Output check results in JSON")
	checkCmd.Flags().BoolVarP(&unattendedFlag, "unattended", "u", false, "Do not prompt for confirmation")
	checkCmd.Flags().BoolVar(&verboseFlag, "verbose", false, "Verbose output")
	checkCmd.Flags().BoolVarP(&individualReportsFlag, "individual", "i", false, "Display the full individual reports for each file in the batch")
	checkCmd.Flags().IntVar(&jobsFlag, "jobs", 0, "Number of parallel jobs to run (default is number of CPUs)")

	idFlags := []string{"imdb", "tmdb", "tvdb"}
	for _, f := range idFlags {
		_ = checkCmd.Flags().SetAnnotation(f, "group", []string{"id"})
	}
}

type checkJobResult struct {
	reports []types.CheckReport
	meta    *metadata.Metadata
	err     error
}

func runChecksInParallel(cmd *cobra.Command, filePaths []string, numJobs int, batchMode bool) []checkJobResult {
	results := make([]checkJobResult, len(filePaths))

	type workItem struct {
		index    int
		filePath string
	}

	workChan := make(chan workItem, len(filePaths))

	for i, filePath := range filePaths {
		workChan <- workItem{index: i, filePath: filePath}
	}

	close(workChan)

	var (
		printMu        sync.Mutex
		completedCount atomic.Int32
	)

	oldIsSilent := ui.IsSilent
	ui.IsSilent = true

	var wg sync.WaitGroup

	for range numJobs {
		wg.Go(func() {
			for item := range workChan {
				results[item.index] = checkSingleFile(cmd, item.filePath)
				completed := completedCount.Add(1)

				if batchMode && !jsonOutputFlag {
					filenameNoExt := filename.GetBaseName(item.filePath)

					printMu.Lock()

					ui.IsSilent = false

					ui.Println(fmt.Sprintf("Checking (%d/%d): %s", completed, len(filePaths), filenameNoExt))

					ui.IsSilent = true

					printMu.Unlock()
				}
			}
		})
	}

	wg.Wait()

	ui.IsSilent = oldIsSilent

	return results
}

func checkSingleFile(cmd *cobra.Command, filePath string) checkJobResult {
	isJSON, jsonReports := loadJSONReport(filePath)

	if isJSON {
		return checkJobResult{
			reports: jsonReports,
		}
	}

	report, m, err := collectCheckData(cmd, filePath, false)

	return checkJobResult{
		reports: []types.CheckReport{report},
		meta:    m,
		err:     err,
	}
}

func getNumJobs(argCount int) int {
	numJobs := jobsFlag

	if numJobs <= 0 {
		numJobs = runtime.NumCPU()
	}

	if numJobs < 1 {
		numJobs = 1
	}

	if numJobs > argCount {
		numJobs = argCount
	}

	return numJobs
}
