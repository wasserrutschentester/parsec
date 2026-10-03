package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"codeberg.org/upPollo/parsec/internal/checks"
	"codeberg.org/upPollo/parsec/internal/config"
	mdbSearch "codeberg.org/upPollo/parsec/internal/mdb/search"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/filename"
	"codeberg.org/upPollo/parsec/internal/metadata/resolve"
	"codeberg.org/upPollo/parsec/internal/types"
	"codeberg.org/upPollo/parsec/internal/ui"
)

var (
	jsonOutputFlag        bool
	individualReportsFlag bool
	jobsFlag              int
	moveFailedFlag        string
	failingCheckFlag      string
)

type seasonKey struct {
	tvdbID int
	tmdbID int
	season int
}

var (
	errCheckDataCollection          = errors.New("collecting check data failed")
	errFailingCheckRequiresMoveFlag = errors.New("--failing-check requires --move-failed")
	errUnknownCheckIdentifier       = errors.New("unknown check identifier")
)

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

		if failingCheckFlag != "" && moveFailedFlag == "" {
			return errFailingCheckRequiresMoveFlag
		}

		if failingCheckFlag != "" && !slices.Contains(config.AllChecks, failingCheckFlag) {
			return fmt.Errorf("%w: %s", errUnknownCheckIdentifier, failingCheckFlag)
		}

		if originalLanguageFlag != "" {
			viper.Set("original_language", originalLanguageFlag)
		}

		expandedArgs := expandArgs(args)
		batchMode := !individualReportsFlag && len(expandedArgs) > 1

		if len(expandedArgs) == 1 {
			isJSON, jsonReports := loadJSONReport(expandedArgs[0])
			if isJSON && len(jsonReports) > 1 {
				batchMode = !individualReportsFlag
			}
		}

		ui.Println(ui.Banner(".: INTEGRITY VERIFICATION :."))

		allReports, seasonEpisodes, seasonMetas, err := runBatchChecks(cmd, expandedArgs, batchMode)
		if err != nil {
			return err
		}

		if !jsonOutputFlag && batchMode {
			ui.PrintAggregatedSummary(allReports, unattendedFlag)
		}

		// Run aggregate season checks
		if !jsonOutputFlag {
			runSeasonCompletenessChecks(seasonEpisodes, seasonMetas)
		}

		if moveFailedFlag != "" {
			if err := moveFailedFiles(allReports, moveFailedFlag, failingCheckFlag); err != nil {
				return err
			}
		}

		if jsonOutputFlag {
			printJSONReports(allReports)
		}

		return nil
	},
}

func runBatchChecks(cmd *cobra.Command, expandedArgs []string, batchMode bool) ([]types.CheckReport, map[seasonKey][]int, map[seasonKey]*metadata.Metadata, error) {
	var allReports []types.CheckReport

	seasonEpisodes := make(map[seasonKey][]int)
	seasonMetas := make(map[seasonKey]*metadata.Metadata)

	numJobs := getNumJobs(len(expandedArgs))
	results := runChecksInParallel(cmd, expandedArgs, numJobs, batchMode)

	for i, filePath := range expandedArgs {
		if err := processCheckResult(filePath, results[i], batchMode, &allReports, seasonEpisodes, seasonMetas); err != nil {
			return nil, nil, nil, err
		}
	}

	return allReports, seasonEpisodes, seasonMetas, nil
}

func processCheckResult(filePath string, res checkJobResult, batchMode bool, allReports *[]types.CheckReport, seasonEpisodes map[seasonKey][]int, seasonMetas map[seasonKey]*metadata.Metadata) error {
	if res.err != nil {
		ui.PrintError(res.err.Error())

		return fmt.Errorf("%w for %s", errCheckDataCollection, filePath)
	}

	if !batchMode && !jsonOutputFlag {
		filenameNoExt := filename.GetBaseName(filePath)

		ui.Println("\n" + ui.Header.Render("VERIFYING NEW TARGET"))
		ui.Println(filenameNoExt)
	}

	*allReports = append(*allReports, res.reports...)

	updateSeasonEpisodes(res.meta, seasonEpisodes, seasonMetas)

	if !jsonOutputFlag && !batchMode {
		for _, r := range res.reports {
			ui.PrintInteractiveReport(r, unattendedFlag)
		}
	}

	return nil
}

func updateSeasonEpisodes(meta *metadata.Metadata, seasonEpisodes map[seasonKey][]int, seasonMetas map[seasonKey]*metadata.Metadata) {
	if meta != nil && meta.IsTV && (meta.TvdbID > 0 || meta.TmdbID > 0) && meta.Season > 0 {
		key := seasonKey{tvdbID: meta.TvdbID, tmdbID: meta.TmdbID, season: meta.Season}

		if len(meta.Episodes) > 0 {
			seasonEpisodes[key] = append(seasonEpisodes[key], meta.Episodes...)
		}

		seasonMetas[key] = meta
	}
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
		res, err := mdbSearch.InteractiveSearch(meta, true, true)

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

	res, err := resolve.Metadata(resolve.Options{
		FilePath:   filePath,
		ImdbID:     imdbIDFlag,
		TmdbID:     tmdbIDFlag,
		TvdbID:     tvdbIDFlag,
		IsTVSet:    cmd.Flags().Changed("tv"),
		IsMovieSet: cmd.Flags().Changed("movie"),
		ParseEBML:  true,
	})
	if err != nil {
		return types.CheckReport{}, nil, fmt.Errorf("error getting mediainfo: %w", err)
	}

	match := res.Meta
	mi := res.MediaInfo
	ebml := res.Ebml
	ebmlErr := res.EbmlErr

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

func filterFailedReports(reports []types.CheckReport, targetCheck string) []string {
	var matchedFiles []string

	for _, r := range reports {
		for _, group := range r.Issues {
			matched := slices.ContainsFunc(group.Results, func(res types.CheckResult) bool {
				return !res.Passed && (targetCheck == "" || res.Identifier == targetCheck)
			})
			if matched {
				matchedFiles = append(matchedFiles, r.File)

				break
			}
		}
	}

	return matchedFiles
}

//nolint:cyclop // file relocation logic with interactive prompt and dry-run handling
func moveFailedFiles(reports []types.CheckReport, destDir, targetCheck string) error {
	matchedFiles := filterFailedReports(reports, targetCheck)
	if len(matchedFiles) == 0 {
		return nil
	}

	if !jsonOutputFlag {
		ui.Println("\n" + ui.Header.Render("MOVING FAILED FILES"))
	}

	if dryRunFlag {
		for _, f := range matchedFiles {
			dest := filepath.Join(destDir, filepath.Base(f))
			if !jsonOutputFlag {
				ui.Println(ui.Muted.Render(fmt.Sprintf("Dry run: would move %s -> %s", f, dest)))
			}
		}

		return nil
	}

	if !unattendedFlag && !jsonOutputFlag {
		fmt.Printf("Move %d failing file(s) to %s? [y/N] ", len(matchedFiles), destDir)

		var response string

		_, _ = fmt.Scanln(&response)
		if strings.ToLower(strings.TrimSpace(response)) != "y" {
			ui.Println(ui.Muted.Render("Skipping move."))

			return nil
		}
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	for _, f := range matchedFiles {
		dest := filepath.Join(destDir, filepath.Base(f))
		// ponytail: os.Rename fails across filesystems (EXDEV); add io.Copy fallback if cross-device quarantine needed
		if err := os.Rename(f, dest); err != nil {
			ui.PrintError(fmt.Sprintf("Failed to move %s: %v", f, err))

			continue
		}

		if !jsonOutputFlag {
			ui.Println(ui.Success.Render(fmt.Sprintf("Moved: %s -> %s", filepath.Base(f), destDir)))
		}
	}

	return nil
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
	// ID
	checkCmd.Flags().IntVar(&tmdbIDFlag, "tmdb", 0, "TMDB ID")
	checkCmd.Flags().IntVar(&tvdbIDFlag, "tvdb", 0, "TVDB ID")
	checkCmd.Flags().StringVar(&imdbIDFlag, "imdb", "", "IMDb ID")
	checkCmd.Flags().String("tvdb-order", "", "TVDB episode ordering for completeness checks (default, official, dvd, absolute, alternate, regional)")
	// output
	checkCmd.Flags().BoolVarP(&individualReportsFlag, "individual", "i", false, "Display the full individual reports for each file in the batch")
	checkCmd.Flags().BoolVarP(&jsonOutputFlag, "json", "j", false, "Output check results in JSON")
	checkCmd.Flags().BoolVar(&verboseFlag, "verbose", false, "Verbose output")
	// other
	checkCmd.Flags().StringVar(&originalLanguageFlag, "original-language", "", "Override original language")
	checkCmd.Flags().BoolVarP(&unattendedFlag, "unattended", "u", false, "Do not prompt for confirmation")
	checkCmd.Flags().IntVar(&jobsFlag, "jobs", 0, "Number of parallel jobs to run (default is number of CPUs)")
	checkCmd.Flags().StringVar(&moveFailedFlag, "move-failed", "", "move files failing checks to destination folder")
	checkCmd.Flags().StringVar(&failingCheckFlag, "failing-check", "", "only move files failing this specific check identifier")
	checkCmd.Flags().BoolVarP(&dryRunFlag, "dry-run", "d", false, "simulate actions without modifying or moving files")

	idFlags := []string{"imdb", "tmdb", "tvdb", "tvdb-order"}
	for _, f := range idFlags {
		_ = checkCmd.Flags().SetAnnotation(f, "group", []string{"id"})
	}

	outputFlags := []string{"individual", "json", "verbose"}
	for _, f := range outputFlags {
		_ = checkCmd.Flags().SetAnnotation(f, "group", []string{"output"})
	}

	_ = checkCmd.MarkFlagDirname("move-failed")
	_ = checkCmd.RegisterFlagCompletionFunc("failing-check", completeCheckIdentifiers)
	_ = checkCmd.RegisterFlagCompletionFunc("original-language", completeLanguages)
	_ = checkCmd.RegisterFlagCompletionFunc("tvdb-order", completeTvdbOrders)
	_ = viper.BindPFlag("tvdb_order", checkCmd.Flags().Lookup("tvdb-order"))
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
