package cmd

import (
	"fmt"

	"codeberg.org/n0ne/parsec/internal/mdb"
	mdbSearch "codeberg.org/n0ne/parsec/internal/mdb/search"
	"codeberg.org/n0ne/parsec/internal/metadata"
	"codeberg.org/n0ne/parsec/internal/metadata/filename"
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

		if meta.Title == "" && imdbIDFlag == "" && tmdbIDFlag == 0 && tvdbIDFlag == 0 {
			fmt.Println("Error: Title is required (either from filename or --title flag) OR an ID (--imdb, --tmdb, --tvdb)")
			return
		}

		mediaType := "movie"
		if meta.IsTV {
			mediaType = "tv"
		}

		meta.SetDefaults()

		var result *mdb.SearchResult
		var err error

		if imdbIDFlag != "" || tmdbIDFlag > 0 || tvdbIDFlag > 0 {
			fmt.Printf("Searching by ID: IMDB:%s TMDB:%d TVDB:%d [%s]...\n", imdbIDFlag, tmdbIDFlag, tvdbIDFlag, mediaType)
			result, err = mdbSearch.SearchByID(imdbIDFlag, tmdbIDFlag, tvdbIDFlag, meta.IsTV)
		} else {
			// Replace dots with spaces for the search query
			searchQuery := filename.DeobfuscateTitle(meta.Title)
			fmt.Printf("Searching for %s (%d) [%s]...\n", searchQuery, meta.Year, mediaType)
			result, err = mdbSearch.FuzzySearch(searchQuery, meta.Year, meta.IsTV)
		}

		if err != nil {
			fmt.Printf("Error searching: %v\n", err)
			return
		}

		if result == nil {
			fmt.Println("No results found")
			return
		}

		mdb.PrintResult(*result)
		tags := mdb.GetMatroskaTags(*result)

		if meta.IsTV && (meta.Season > 0 || meta.Episode > 0) {
			episodeResult := mdbSearch.FindEpisode(*result, meta.Season, meta.Episode)
			mdb.PrintEpisodeResult(episodeResult)
			tags.SetEpisodeTags(episodeResult)
		}

		if !unattendedFlag && !dryRunFlag {
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
