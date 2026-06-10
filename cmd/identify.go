package cmd

import (
	"errors"
	"fmt"

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
			return identifyFile(cmd, "")
		}

		expandedArgs := expandArgs(args)
		for _, filePath := range expandedArgs {
			err := identifyFile(cmd, filePath)
			if err != nil {
				return err
			}
		}

		return nil
	},
}

func identifyFile(cmd *cobra.Command, filePath string) error {
	meta := initializeMetadata(cmd, filePath)

	result, err := mdbSearch.InteractiveSearch(meta, unattendedFlag)
	if err != nil {
		ui.PrintError(err.Error())

		return errSearch
	}

	processIdentificationResult(filePath, result, meta)

	return nil
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

func processIdentificationResult(filePath string, result *mdb.SearchResult, meta *metadata.Metadata) {
	warnOnIDMismatch(filePath, result)

	mdb.PrintResult(*result)
	tags := mdb.GetMatroskaTags(*result)

	if meta.IsTV {
		handleTVEpisode(result, meta, &tags)
	}

	if releasesFlag {
		prowlarr.PrintReleases(result, meta, bestFlag)
	}

	shouldWriteTags := writeTagsFlag
	if !unattendedFlag && !dryRunFlag && filePath != "" && matroska.CheckForMatroska(filePath) == nil {
		shouldWriteTags = shouldWriteTagsInteractively()
	}

	if shouldWriteTags && filePath != "" {
		writeTags(filePath, tags)
	}
}

func handleTVEpisode(result *mdb.SearchResult, meta *metadata.Metadata, tags *mdb.MatroskaTags) {
	episodeResult := getEpisodeResult(result, meta)
	if episodeResult.Name != "" {
		tags.SetEpisodeTags(episodeResult)
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

func writeTags(filePath string, tags mdb.MatroskaTags) {
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
	identifyCmd.Flags().IntVarP(&episodeFlag, "episode", "e", 0, "episode number")
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
	// Other
	identifyCmd.Flags().BoolVar(&writeTagsFlag, "write-tags", false, "write metadata tags to the file")
	identifyCmd.Flags().BoolVarP(&unattendedFlag, "unattended", "u", false, "run in unattended mode")
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

func getEpisodeResult(result *mdb.SearchResult, meta *metadata.Metadata) mdb.EpisodeResult {
	var episodeResult mdb.EpisodeResult

	if meta.Season > 0 && meta.Episode > 0 || meta.EpisodeTitle != "" || meta.Date != "" {
		ui.Println(ui.Info.Render("Identifying episode..."))

		episodeResult = mdbSearch.FindEpisode(*result, meta, config.GetAllowSpecials())
	}

	if episodeResult.Name != "" {
		mdb.PrintEpisodeResult(episodeResult)
		// Back-fill metadata
		meta.Season = episodeResult.Season
		meta.Episode = episodeResult.Episode
		meta.EpisodeTitle = episodeResult.Name
	} else if meta.Season > 0 || meta.Episode > 0 || meta.EpisodeTitle != "" || meta.Date != "" {
		ui.Println(ui.FormatWarning("Could not identify episode metadata"))
	}

	ui.PrintDebug(fmt.Sprintf("%+v\n", meta))
	ui.PrintDebug(fmt.Sprintf("%+v\n", episodeResult))

	return episodeResult
}
