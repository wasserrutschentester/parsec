package cmd

import (
	"fmt"

	"codeberg.org/n0ne/parsec/internal/config"
	"codeberg.org/n0ne/parsec/internal/mdb"
	mdbSearch "codeberg.org/n0ne/parsec/internal/mdb/search"
	"codeberg.org/n0ne/parsec/internal/metadata"
	"codeberg.org/n0ne/parsec/internal/metadata/filename"
	"codeberg.org/n0ne/parsec/internal/metadata/matroska"
	"codeberg.org/n0ne/parsec/internal/metadata/mediainfo"
	"codeberg.org/n0ne/parsec/internal/ui"

	"github.com/spf13/cobra"
)

var (
	writeTagsFlag bool
)

// identifyCmd represents the identify command
var identifyCmd = &cobra.Command{
	Use:   "identify [filename]",
	Short: "search for a movie or TV show",
	Long: fmt.Sprintf("%s\n%s", ui.Banner(".: CLASSIFY ENTITIES :."),
		`find a movie or TV show on the media databases
check if it exists and return its details.
If a filename is provided, it will be parsed for metadata.
Flags can be used to override or provide missing information.`),
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var meta *metadata.Metadata
		filePath := ""
		if len(args) > 0 {
			filePath = args[0]
			filenameNoExt := filename.GetBaseName(filePath)
			meta = filename.Parse(filenameNoExt)
			ui.Println(ui.Banner(".: ENTITY CLASSIFICATION :."))
			ui.Println(ui.LabelValue("Target Name:", filenameNoExt))
		} else {
			meta = &metadata.Metadata{}
			ui.Println(ui.Banner(".: ENTITY CLASSIFICATION :."))
		}

		applyMetadataFlags(cmd, meta)
		meta.SetDefaults()

		result, err := mdbSearch.InteractiveSearch(meta, unattendedFlag)
		if err != nil {
			ui.PrintError(err.Error())
			return
		}

		warnOnIDMismatch(filePath, result)

		mdb.PrintResult(*result)
		tags := mdb.GetMatroskaTags(*result)

		if meta.IsTV {
			episodeResult := getEpisodeResult(result, meta)
			if episodeResult.Name != "" {
				tags.SetEpisodeTags(episodeResult)
			}
		}
		if !unattendedFlag && !dryRunFlag && filePath != "" && matroska.CheckForMatroska(filePath) == nil {
			fmt.Print(ui.Info.Render("\nDo you want to write the tags to the file? [y/N] "))
			var response string
			fmt.Scanln(&response)
			if response != "y" && response != "Y" {
				ui.Println(ui.Muted.Render("Skipping..."))
				return
			} else {
				writeTagsFlag = true
			}
		}

		if writeTagsFlag {
			err := matroska.SetGlobalTags(filePath, tags)
			if err != nil {
				ui.PrintError(fmt.Sprintf("Error writing tags: %v", err))
			} else {
				ui.Println(ui.Success.Render("All systems nominal! Tags written successfully"))
			}
		}
	},
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
	// Group metadata flags
	metadataFlags := []string{"title", "year", "season", "episode", "date", "episode-title"}
	for _, f := range metadataFlags {
		identifyCmd.Flags().SetAnnotation(f, "group", []string{"metadata"})
	}

	// Group P2P flags
	p2pFlags := []string{"service", "source", "group"}
	for _, f := range p2pFlags {
		identifyCmd.Flags().SetAnnotation(f, "group", []string{"p2p"})
	}

	// Group ID flags
	idFlags := []string{"tv", "movie", "imdb", "tmdb", "tvdb"}
	for _, f := range idFlags {
		identifyCmd.Flags().SetAnnotation(f, "group", []string{"id"})
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
	if (tagImdb != "" && result.ImdbID != "" && tagImdb != result.ImdbID) ||
		(tagTmdb != 0 && result.TmdbID != 0 && tagTmdb != result.TmdbID) ||
		(tagTvdb != 0 && result.TvdbID != 0 && tagTvdb != result.TvdbID) {
		ui.Println("\n" + ui.FormatWarning("Selected result IDs do not match file tags:"))
		if tagImdb != "" && tagImdb != result.ImdbID {
			ui.Println("  " + ui.LabelValue("IMDB (File vs Selected):", fmt.Sprintf("%s / %s", tagImdb, result.ImdbID)))
		}
		if tagTmdb != 0 && tagTmdb != result.TmdbID {
			ui.Println("  " + ui.LabelValue("TMDB (File vs Selected):", fmt.Sprintf("%d / %d", tagTmdb, result.TmdbID)))
		}
		if tagTvdb != 0 && tagTvdb != result.TvdbID {
			ui.Println("  " + ui.LabelValue("TVDB (File vs Selected):", fmt.Sprintf("%d / %d", tagTvdb, result.TvdbID)))
		}
	}
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
