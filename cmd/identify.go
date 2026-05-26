package cmd

import (
	"fmt"

	"codeberg.org/n0ne/parsec/internal/mdb"
	mdbSearch "codeberg.org/n0ne/parsec/internal/mdb/search"
	"codeberg.org/n0ne/parsec/internal/metadata"
	"codeberg.org/n0ne/parsec/internal/metadata/filename"
	"codeberg.org/n0ne/parsec/internal/metadata/mediainfo"
	"github.com/spf13/cobra"
)

var (
	writeTagsFlag bool
)

// identifyCmd represents the identify command
var identifyCmd = &cobra.Command{
	Use:   "identify [filename]",
	Short: "search for a movie or TV show",
	Long: `find a movie or TV show on the media databases
	check if it exists and return its details.
	If a filename is provided, it will be parsed for metadata.
	Flags can be used to override or provide missing information.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var meta *metadata.Metadata
		filePath := ""
		if len(args) > 0 {
			filePath = args[0]
			filenameNoExt := filename.GetBaseName(filePath)
			meta = filename.Parse(filenameNoExt)
		} else {
			meta = &metadata.Metadata{}
		}

		if titleFlag != "" {
			meta.Title = titleFlag
		}
		if yearFlag != 0 {
			meta.Year = yearFlag
		}
		if seasonFlag != 0 {
			meta.Season = seasonFlag
		}
		if episodeFlag != 0 {
			meta.Episode = episodeFlag
		}
		if serviceFlag != "" {
			meta.Service = serviceFlag
		}
		if sourceFlag != "" {
			meta.Source = sourceFlag
		}
		if groupFlag != "" {
			meta.Group = groupFlag
		}

		if cmd.Flags().Changed("tv") {
			meta.IsTV = isTVFlag
		} else if cmd.Flags().Changed("movie") {
			meta.IsTV = !isMovieFlag
		}

		if imdbIDFlag != "" {
			meta.ImdbID = imdbIDFlag
		}
		if tmdbIDFlag != 0 {
			meta.TmdbID = tmdbIDFlag
		}
		if tvdbIDFlag != 0 {
			meta.TvdbID = tvdbIDFlag
		}

		meta.SetDefaults()

		result, err := mdbSearch.InteractiveSearch(meta, unattendedFlag)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		warnOnIDMismatch(filePath, result)

		mdb.PrintResult(*result)
		tags := mdb.GetMatroskaTags(*result)

		if meta.IsTV && (meta.Season > 0 || meta.Episode > 0) {
			episodeResult := mdbSearch.FindEpisode(*result, meta.Season, meta.Episode)
			mdb.PrintEpisodeResult(episodeResult)
			tags.SetEpisodeTags(episodeResult)
		}

		if !unattendedFlag && !dryRunFlag && filePath != "" && metadata.CheckForMatroska(filePath) == nil {
			fmt.Print("Do you want to write the tags to the file? [y/N] ")
			var response string
			fmt.Scanln(&response)
			if response != "y" && response != "Y" {
				fmt.Println("Skipping...")
				return
			} else {
				writeTagsFlag = true
			}
		}

		if writeTagsFlag {
			err := metadata.SetGlobalTags(filePath, tags)
			if err != nil {
				fmt.Printf("Error writing tags: %v\n", err)
			} else {
				fmt.Println("Tags written successfully")
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
	// Other
	identifyCmd.Flags().BoolVar(&writeTagsFlag, "write-tags", false, "write metadata tags to the file")
	identifyCmd.Flags().BoolVarP(&unattendedFlag, "unattended", "u", false, "run in unattended mode")
	// Group metadata flags
	metadataFlags := []string{"title", "year", "season", "episode", "date", "episode-title"}
	for _, f := range metadataFlags {
		identifyCmd.Flags().SetAnnotation(f, "group", []string{"metadata"})
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
		fmt.Printf("\nWARNING: Selected result IDs do not match file tags:\n")
		if tagImdb != "" && tagImdb != result.ImdbID {
			fmt.Printf("  IMDB: File=%s, Selected=%s\n", tagImdb, result.ImdbID)
		}
		if tagTmdb != 0 && tagTmdb != result.TmdbID {
			fmt.Printf("  TMDB: File=%d, Selected=%d\n", tagTmdb, result.TmdbID)
		}
		if tagTvdb != 0 && tagTvdb != result.TvdbID {
			fmt.Printf("  TVDB: File=%d, Selected=%d\n", tagTvdb, result.TvdbID)
		}
	}
}
