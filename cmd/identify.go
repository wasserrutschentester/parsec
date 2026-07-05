package cmd

import (
	"errors"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

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
	writeTagsFlag bool
	commentFlag   string
	errSearch     = errors.New("search failed")
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
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Println(ui.Banner(".: ENTITY CLASSIFICATION :."))

		if len(args) == 0 {
			_, err := identifyFile(cmd, "", nil)

			return err
		}

		return runIdentifyBatch(cmd, expandArgs(args))
	},
}

// runIdentifyBatch identifies each file independently. A failure on one file
// (including the user answering an interactive disambiguation prompt badly)
// is reported by identifyFile and skipped rather than aborting the rest of
// the batch; only when every file in the batch fails does this return an
// error, so the process still exits non-zero when nothing succeeded.
func runIdentifyBatch(cmd *cobra.Command, filePaths []string) error {
	var prevResult *mdb.SearchResult

	errs := make([]error, 0, len(filePaths))

	for _, filePath := range filePaths {
		result, err := identifyFile(cmd, filePath, prevResult)
		errs = append(errs, err)

		if err == nil {
			prevResult = result
		}
	}

	return batchIdentifyError(errs)
}

func batchIdentifyError(errs []error) error {
	for _, err := range errs {
		if err == nil {
			return nil
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return errSearch
}

func identifyFile(cmd *cobra.Command, filePath string, prevResult *mdb.SearchResult) (*mdb.SearchResult, error) {
	meta := initializeMetadata(cmd, filePath)

	result, err := mdbSearch.InteractiveSearch(meta, unattendedFlag)
	if err != nil {
		ui.PrintError(err.Error())

		return nil, errSearch
	}

	processIdentificationResult(filePath, result, meta, prevResult)

	return result, nil
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

func processIdentificationResult(filePath string, result *mdb.SearchResult, meta *metadata.Metadata, prevResult *mdb.SearchResult) {
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

	maybeWriteTags(filePath, tags)
}

func maybeWriteTags(filePath string, tags []mdb.MatroskaTagSet) {
	if filePath == "" || len(tags) == 0 {
		return
	}

	if dryRunFlag {
		printTagPreview(tags)

		return
	}

	shouldWriteTags := writeTagsFlag || unattendedFlag

	if !unattendedFlag && matroska.CheckForMatroska(filePath) == nil {
		printTagPreview(tags)

		shouldWriteTags = shouldWriteTagsInteractively()
	} else if shouldWriteTags {
		printTagPreview(tags)
	}

	if shouldWriteTags {
		writeTags(filePath, tags)
	}
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

			keys := make([]string, 0, len(tagSet.Fields))
			for k := range tagSet.Fields {
				keys = append(keys, k)
			}

			sort.Slice(keys, func(i, j int) bool {
				rankI, okI := defaultTagOrder[keys[i]]
				if !okI {
					rankI = 1000
				}

				rankJ, okJ := defaultTagOrder[keys[j]]
				if !okJ {
					rankJ = 1000
				}

				if rankI == rankJ {
					return keys[i] < keys[j]
				}

				return rankI < rankJ
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

func writeTags(filePath string, tags []mdb.MatroskaTagSet) {
	err := matroska.SetGlobalTags(filePath, tags)
	if err != nil {
		ui.PrintError(fmt.Sprintf("Error writing tags: %v", err))
	} else {
		ui.Println(ui.Success.Render("All systems nominal! Tags written successfully"))
	}
}

func init() {
	rootCmd.AddCommand(identifyCmd)
	// Title flags
	identifyCmd.Flags().StringVarP(&titleFlag, "title", "t", "", "title of the movie or TV show")
	identifyCmd.Flags().IntVarP(&yearFlag, "year", "y", 0, "release year")
	identifyCmd.Flags().IntVarP(&seasonFlag, "season", "s", 0, "season number")
	identifyCmd.Flags().IntSliceVarP(&episodeFlag, "episode", "e", nil, "episode numbers (comma-separated)")
	identifyCmd.Flags().StringVarP(&dateFlag, "date", "D", "", "episode aired date")
	identifyCmd.Flags().StringVar(&episodeTitleFlag, "episode-title", "", "episode title")
	// Mdb IDs
	identifyCmd.Flags().BoolVarP(&isTVFlag, "tv", "T", false, "identify as TV show")
	identifyCmd.Flags().BoolVarP(&isMovieFlag, "movie", "M", false, "identify as movie")
	identifyCmd.Flags().StringVar(&imdbIDFlag, "imdb", "", "IMDb ID")
	identifyCmd.Flags().IntVar(&tmdbIDFlag, "tmdb", 0, "TMDB ID")
	identifyCmd.Flags().IntVar(&tvdbIDFlag, "tvdb", 0, "TVDB ID")
	// P2P Info
	identifyCmd.Flags().StringVarP(&serviceFlag, "service", "S", "", "streaming service")
	identifyCmd.Flags().StringVarP(&sourceFlag, "source", "O", "", "source (e.g. BluRay, Web-DL)")
	identifyCmd.Flags().StringVarP(&groupFlag, "group", "g", "", "release group")
	// Tag flags
	identifyCmd.Flags().BoolVarP(&dryRunFlag, "dry-run", "d", false, "simulate identification and preview tags without writing to the file")
	identifyCmd.Flags().BoolVar(&writeTagsFlag, "write-tags", false, "write metadata tags to the file")
	_ = identifyCmd.Flags().MarkDeprecated("write-tags", "use --unattended instead")
	identifyCmd.Flags().StringVar(&commentFlag, "comment", "", "comment to expose to tag templates")
	identifyCmd.Flags().BoolVarP(&unattendedFlag, "unattended", "u", false, "run in unattended mode (implies writing tags)")
	identifyCmd.Flags().BoolVarP(&releasesFlag, "releases", "r", false, "search for releases via Prowlarr")
	identifyCmd.Flags().BoolVarP(&bestFlag, "best-release", "b", false, "only show the best release per indexer")
	// Group metadata flags
	metadataFlags := []string{"title", "year", "season", "episode", "date", "episode-title"}
	for _, f := range metadataFlags {
		_ = identifyCmd.Flags().SetAnnotation(f, "group", []string{"metadata"})
	}

	// Group P2P flags
	p2pFlags := []string{"service", "source", "group"}
	for _, f := range p2pFlags {
		_ = identifyCmd.Flags().SetAnnotation(f, "group", []string{"p2p"})
	}

	// Group ID flags
	idFlags := []string{"tv", "movie", "imdb", "tmdb", "tvdb"}
	for _, f := range idFlags {
		_ = identifyCmd.Flags().SetAnnotation(f, "group", []string{"id"})
	}

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
