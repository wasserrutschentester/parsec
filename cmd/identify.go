package cmd

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/mdb"
	"codeberg.org/upPollo/parsec/internal/mdb/prowlarr"
	mdbSearch "codeberg.org/upPollo/parsec/internal/mdb/search"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/filename"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/metadata/mediainfo"
	"codeberg.org/upPollo/parsec/internal/ui"
)

var (
	writeTagsFlag  bool
	commentFlag    string
	moveTaggedFlag string
	errSearch      = errors.New("search failed")
)

// identifyCmd represents the identify command
var identifyCmd = &cobra.Command{
	Use:   "identify [path...]",
	Short: "search for a movie or TV show",
	Long: fmt.Sprintf("%s\n%s", ui.Banner(".: CLASSIFY ENTITIES :."),
		`find a movie or TV show on the media databases
check if it exists and return its details.
If filenames or directories are provided, they will be parsed for metadata.
Directories are scanned recursively for Matroska files.
Flags can be used to override or provide missing information.`),
	Args: cobra.ArbitraryArgs,
	ValidArgsFunction: func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeFiles(toComplete, "mkv")
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Println(ui.Banner(".: ENTITY CLASSIFICATION :."))

		if len(args) == 0 {
			_, _, err := identifyFile(cmd, "", nil)

			return err
		}

		return runIdentifyBatch(cmd, expandArgs(args))
	},
}

func runIdentifyBatch(cmd *cobra.Command, filePaths []string) error {
	var (
		prevResult  *mdb.SearchResult
		taggedFiles []string
	)

	errs := make([]error, 0, len(filePaths))

	for _, filePath := range filePaths {
		result, tagged, err := identifyFile(cmd, filePath, prevResult)
		errs = append(errs, err)

		if err == nil {
			prevResult = result
		}

		if tagged {
			taggedFiles = append(taggedFiles, filePath)
		}
	}

	if err := moveTaggedFiles(taggedFiles, moveTaggedFlag); err != nil {
		return err
	}

	return batchIdentifyError(errs)
}

func batchIdentifyError(errs []error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	// if all errs were nil
	return nil
}

func identifyFile(cmd *cobra.Command, filePath string, prevResult *mdb.SearchResult) (*mdb.SearchResult, bool, error) {
	meta := initializeMetadata(cmd, filePath)

	result, err := mdbSearch.InteractiveSearch(meta, unattendedFlag, false)
	if err != nil {
		ui.PrintError(err.Error())

		return nil, false, errSearch
	}

	tagged := processIdentificationResult(filePath, result, meta, prevResult)

	return result, tagged, nil
}

func initializeMetadata(cmd *cobra.Command, filePath string) *metadata.Metadata {
	var meta *metadata.Metadata

	if filePath != "" {
		filenameNoExt := filename.GetBaseName(filePath)
		meta = filename.Parse(filenameNoExt)
		ui.Println(ui.LabelValue("Target Name:", filenameNoExt))
	} else {
		meta = &metadata.Metadata{}
	}

	applyMetadataFlags(cmd, meta)
	meta.SetDefaults()

	return meta
}

func isSameSeries(result, prevResult *mdb.SearchResult) bool {
	return prevResult != nil &&
		result.TmdbID == prevResult.TmdbID &&
		result.ImdbID == prevResult.ImdbID &&
		result.TvdbID == prevResult.TvdbID
}

func processIdentificationResult(filePath string, result *mdb.SearchResult, meta *metadata.Metadata, prevResult *mdb.SearchResult) bool {
	warnOnIDMismatch(filePath, result)

	if !isSameSeries(result, prevResult) {
		mdb.PrintResult(*result)
	}

	var relName string
	if filePath != "" {
		relName = filename.GetBaseName(filePath)
	}

	ctx := mdb.TagTemplateContext{
		Media:       *result,
		Comment:     commentFlag,
		ReleaseName: relName,
	}
	if meta.IsTV {
		episodes := getEpisodeResults(result, meta)
		if len(episodes) > 0 {
			ctx.Episodes = episodes
			ctx.Episode = &episodes[0]
		}
	}

	tags, err := mdb.GetMatroskaTags(ctx)
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to generate tags: %v", err))
		// We could still continue or return, but let's just proceed without tags or return early.
		tags = []mdb.MatroskaTagSet{}
	}

	if releasesFlag {
		prowlarr.PrintReleases(result, meta, bestFlag)
	}

	return maybeWriteTags(filePath, tags)
}

func maybeWriteTags(filePath string, tags []mdb.MatroskaTagSet) bool {
	if filePath == "" || len(tags) == 0 {
		return false
	}

	if dryRunFlag {
		printTagPreview(tags)

		return true
	}

	shouldWriteTags := writeTagsFlag || unattendedFlag

	if !unattendedFlag && matroska.CheckForMatroska(filePath) == nil {
		printTagPreview(tags)

		shouldWriteTags = shouldWriteTagsInteractively()
	} else if shouldWriteTags {
		printTagPreview(tags)
	}

	if shouldWriteTags {
		return writeTags(filePath, tags)
	}

	return false
}

var defaultTagOrder = map[string]int{
	"TITLE":         1,
	"PART_NUMBER":   2,
	"TOTAL_PARTS":   3,
	"DATE_RELEASED": 4,
	"IMDB":          5,
	"TMDB":          6,
	"TVDB":          7,
	"TVDB2":         8,
	"COMMENT":       9,
}

func printTagPreview(tags []mdb.MatroskaTagSet) {
	if config.GetTagPreview() && len(tags) > 0 {
		fmt.Println(ui.Info.Render("\nTag Preview:"))

		for _, tagSet := range tags {
			if len(tagSet.Fields) == 0 {
				continue
			}

			fmt.Println(ui.Muted.Render(fmt.Sprintf("  TargetTypeValue: %d", tagSet.TargetTypeValue)))

			keys := slices.Collect(maps.Keys(tagSet.Fields))

			slices.SortFunc(keys, func(a, b string) int {
				rankA, okA := defaultTagOrder[a]
				if !okA {
					rankA = 1000
				}

				rankB, okB := defaultTagOrder[b]
				if !okB {
					rankB = 1000
				}

				if rankA != rankB {
					return cmp.Compare(rankA, rankB)
				}

				return cmp.Compare(a, b)
			})

			for _, k := range keys {
				v := tagSet.Fields[k]
				fmt.Printf("    %s %s\n", ui.LabelStyle.Render(k+":"), ui.ValueStyle.Render(v))
			}
		}
	}
}

func shouldWriteTagsInteractively() bool {
	fmt.Print(ui.Info.Render("\nDo you want to write the tags to the file? [y/N] "))

	var response string

	_, _ = fmt.Scanln(&response)

	if response == "y" || response == "Y" {
		return true
	}

	ui.Println(ui.Muted.Render("Skipping..."))

	return false
}

func writeTags(filePath string, tags []mdb.MatroskaTagSet) bool {
	err := matroska.SetGlobalTags(filePath, tags)
	if err != nil {
		ui.PrintError(fmt.Sprintf("Error writing tags: %v", err))

		return false
	}

	ui.Println(ui.Success.Render("All systems nominal! Tags written successfully"))

	return true
}

//nolint:cyclop // file relocation logic with interactive prompt and dry-run handling
func moveTaggedFiles(taggedFiles []string, destDir string) error {
	if destDir == "" || len(taggedFiles) == 0 {
		return nil
	}

	ui.Println("\n" + ui.Header.Render("MOVING TAGGED FILES"))

	if dryRunFlag {
		for _, f := range taggedFiles {
			dest := filepath.Join(destDir, filepath.Base(f))
			ui.Println(ui.Muted.Render(fmt.Sprintf("Dry run: would move %s -> %s", f, dest)))
		}

		return nil
	}

	if !unattendedFlag {
		fmt.Printf("Move %d tagged file(s) to %s? [y/N] ", len(taggedFiles), destDir)

		var response string

		_, _ = fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			ui.Println(ui.Muted.Render("Skipping move."))

			return nil
		}
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	for _, f := range taggedFiles {
		dest := filepath.Join(destDir, filepath.Base(f))
		// ponytail: os.Rename fails across filesystems (EXDEV); add io.Copy fallback if cross-device move needed
		if err := os.Rename(f, dest); err != nil {
			ui.PrintError(fmt.Sprintf("Failed to move %s: %v", f, err))

			continue
		}

		ui.Println(ui.Success.Render(fmt.Sprintf("Moved: %s -> %s", filepath.Base(f), destDir)))
	}

	return nil
}

func init() {
	rootCmd.AddCommand(identifyCmd)
	// Title flags
	identifyCmd.Flags().StringVarP(&titleFlag, "title", "t", "", "title of the movie or TV show")
	identifyCmd.Flags().IntVarP(&yearFlag, "year", "y", 0, "release year")
	identifyCmd.Flags().IntVarP(&seasonFlag, "season", "s", 0, "season number")
	identifyCmd.Flags().IntSliceVarP(&episodeFlag, "episode", "e", nil, "episode numbers (comma-separated)")
	identifyCmd.Flags().StringVarP(&dateFlag, "date", "D", "", "episode aired date")
	identifyCmd.Flags().StringVarP(&episodeTitleFlag, "episode-title", "E", "", "episode title")
	// Mdb IDs
	identifyCmd.Flags().BoolVarP(&isTVFlag, "tv", "T", false, "identify as TV show")
	identifyCmd.Flags().BoolVarP(&isMovieFlag, "movie", "M", false, "identify as movie")
	identifyCmd.Flags().StringVar(&imdbIDFlag, "imdb", "", "IMDb ID")
	identifyCmd.Flags().IntVar(&tmdbIDFlag, "tmdb", 0, "TMDB ID")
	identifyCmd.Flags().IntVar(&tvdbIDFlag, "tvdb", 0, "TVDB ID")
	identifyCmd.Flags().String("tvdb-order", "", "TVDB episode ordering for identification (default, official, dvd, absolute, alternate, regional)")
	// Output
	identifyCmd.Flags().BoolVarP(&releasesFlag, "releases", "r", false, "search for releases via Prowlarr")
	identifyCmd.Flags().BoolVarP(&bestFlag, "best-release", "b", false, "only show the best release per indexer")
	// Tag flags
	identifyCmd.Flags().StringVar(&commentFlag, "comment", "", "comment to expose to tag templates")
	identifyCmd.Flags().BoolVarP(&dryRunFlag, "dry-run", "d", false, "simulate identification and preview tags without writing to the file")
	identifyCmd.Flags().BoolVarP(&unattendedFlag, "unattended", "u", false, "run in unattended mode (implies writing tags)")
	identifyCmd.Flags().StringVar(&moveTaggedFlag, "move-tagged", "", "move files that were tagged to destination folder")
	// Deprecated
	identifyCmd.Flags().BoolVar(&writeTagsFlag, "write-tags", false, "write metadata tags to the file")
	identifyCmd.Flags().StringVarP(&serviceFlag, "service", "S", "", "streaming service")
	identifyCmd.Flags().StringVarP(&sourceFlag, "source", "O", "", "source (e.g. BluRay, Web-DL)")
	identifyCmd.Flags().StringVarP(&groupFlag, "group", "g", "", "release group")
	_ = identifyCmd.Flags().MarkDeprecated("write-tags", "use --unattended instead")
	_ = identifyCmd.Flags().MarkDeprecated("source", "source is not used by identify")
	_ = identifyCmd.Flags().MarkDeprecated("service", "service is not used by identify")
	_ = identifyCmd.Flags().MarkDeprecated("group", "group is not used by identify")
	// Group metadata flags
	metadataFlags := []string{"title", "year", "season", "episode", "date", "episode-title"}
	for _, f := range metadataFlags {
		_ = identifyCmd.Flags().SetAnnotation(f, "group", []string{"metadata"})
	}

	// Group ID flags
	idFlags := []string{"tv", "movie", "imdb", "tmdb", "tvdb", "tvdb-order"}
	for _, f := range idFlags {
		_ = identifyCmd.Flags().SetAnnotation(f, "group", []string{"id"})
	}

	// Group Output flags
	outputFlags := []string{"releases", "best-release"}
	for _, f := range outputFlags {
		_ = identifyCmd.Flags().SetAnnotation(f, "group", []string{"output"})
	}

	_ = identifyCmd.RegisterFlagCompletionFunc("tvdb-order", completeTvdbOrders)
	_ = viper.BindPFlag("tvdb_order", identifyCmd.Flags().Lookup("tvdb-order"))

	_ = identifyCmd.MarkFlagDirname("move-tagged")

	// Disable sorting to keep the defined order
	identifyCmd.Flags().SortFlags = false
}

func warnOnIDMismatch(filePath string, result *mdb.SearchResult) {
	if filePath == "" {
		return
	}

	mi, err := mediainfo.Get(filePath)
	if err != nil {
		return
	}

	tagImdb, tagTmdb, tagTvdb, _ := mi.GetMdbIDs()
	mismatches := collectMismatches(tagImdb, result.ImdbID, tagTmdb, result.TmdbID, tagTvdb, result.TvdbID)

	if len(mismatches) > 0 {
		ui.Println("\n" + ui.FormatWarning("Selected result IDs do not match file tags:"))

		for _, m := range mismatches {
			ui.Println("  " + m)
		}
	}
}

func collectMismatches(tagImdb, resImdb string, tagTmdb, resTmdb, tagTvdb, resTvdb int) []string {
	var mismatches []string
	if tagImdb != "" && resImdb != "" && tagImdb != resImdb {
		mismatches = append(mismatches, ui.LabelValue("IMDB (File vs Selected):", fmt.Sprintf("%s / %s", tagImdb, resImdb)))
	}

	if tagTmdb != 0 && resTmdb != 0 && tagTmdb != resTmdb {
		mismatches = append(mismatches, ui.LabelValue("TMDB (File vs Selected):", fmt.Sprintf("%d / %d", tagTmdb, resTmdb)))
	}

	if tagTvdb != 0 && resTvdb != 0 && tagTvdb != resTvdb {
		mismatches = append(mismatches, ui.LabelValue("TVDB (File vs Selected):", fmt.Sprintf("%d / %d", tagTvdb, resTvdb)))
	}

	return mismatches
}

func hasEpisodeMetadata(meta *metadata.Metadata) bool {
	return (meta.Season >= 0 && len(meta.Episodes) > 0) || len(meta.EpisodeTitles) > 0 || meta.Date != ""
}

func getEpisodeResults(result *mdb.SearchResult, meta *metadata.Metadata) []mdb.EpisodeResult {
	if !hasEpisodeMetadata(meta) {
		return nil
	}

	ui.Println(ui.Info.Render("Identifying episode..."))

	episodes := mdbSearch.FindEpisodes(*result, meta, config.GetAllowSpecials())
	if len(episodes) == 0 {
		ui.Println(ui.FormatWarning("Could not identify episode metadata"))

		return nil
	}

	ui.PrintSuccess("Episode identified successfully.")

	epNums := mdb.ExtractEpisodeNumbers(episodes)

	titles := make([]string, 0, len(episodes))
	for _, ep := range episodes {
		mdb.PrintEpisodeResult(ep)
		titles = append(titles, ep.Name)
	}

	meta.Season = episodes[0].Season
	meta.Episodes = epNums
	meta.EpisodeTitles = titles

	ui.PrintDebug(fmt.Sprintf("%+v\n", meta))
	ui.PrintDebug(fmt.Sprintf("%+v\n", episodes))

	return episodes
}
