/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
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
	titleFlag   string
	yearFlag    int
	seasonFlag  int
	episodeFlag int
	isTVFlag    bool
	isMovieFlag bool
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
		if len(args) > 0 {
			filePath := args[0]
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

		if cmd.Flags().Changed("tv") {
			meta.IsTV = isTVFlag
		} else if cmd.Flags().Changed("movie") {
			meta.IsTV = !isMovieFlag
		}

		if meta.Title == "" {
			fmt.Println("Error: Title is required (either from filename or --title flag)")
			return
		}

		// Replace dots with spaces for the search query
		searchQuery := filename.DeobfuscateTitle(meta.Title)

		mediaType := "movie"
		if meta.IsTV {
			mediaType = "tv"
		}
		fmt.Printf("Searching for %s (%d) [%s]...\n", searchQuery, meta.Year, mediaType)

		result, err := mdbSearch.FuzzySearch(searchQuery, meta.Year, meta.IsTV)
		if err != nil {
			fmt.Printf("Error searching TMDB: %v\n", err)
			return
		}

		if result == nil {
			fmt.Println("No results found on TMDB.")
			return
		}

		mdb.PrintResult(*result)

		if meta.IsTV && (meta.Season > 0 || meta.Episode > 0) {
			data := mdbSearch.FindEpisode(*result, meta.Season, meta.Episode)
			mdb.PrintEpisodeResult(data)
		}
	},
}

func init() {
	rootCmd.AddCommand(identifyCmd)

	identifyCmd.Flags().StringVarP(&titleFlag, "title", "t", "", "title of the movie or TV show")
	identifyCmd.Flags().IntVarP(&yearFlag, "year", "y", 0, "release year")
	identifyCmd.Flags().IntVarP(&seasonFlag, "season", "s", 0, "season number")
	identifyCmd.Flags().IntVarP(&episodeFlag, "episode", "e", 0, "episode number")
	identifyCmd.Flags().BoolVar(&isTVFlag, "tv", false, "identify as TV show")
	identifyCmd.Flags().BoolVar(&isMovieFlag, "movie", false, "identify as movie")
}
