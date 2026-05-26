/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"codeberg.org/n0ne/parsec/internal/config"
	"codeberg.org/n0ne/parsec/internal/mdb"
	mdbSearch "codeberg.org/n0ne/parsec/internal/mdb/search"
	"codeberg.org/n0ne/parsec/internal/metadata/filename"
	"codeberg.org/n0ne/parsec/internal/metadata/mediainfo"
	"github.com/spf13/cobra"
)

// renameCmd represents the rename command
var renameCmd = &cobra.Command{
	Use:   "rename [file...]",
	Short: "rename files according to metadata and MDB data",
	Long: `Rename files based on information from:
1. MediaInfo (resolution, codecs, etc.)
2. Movie Database (TMDB/TVDB for correct title and episode name)
3. Information already in the filename
4. CLI flags (to override or provide missing info)

The resulting filename follows the project's naming convention.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		for _, filePath := range args {
			renameFile(cmd, filePath)
		}
	},
}

func renameFile(cmd *cobra.Command, filePath string) {
	ext := filepath.Ext(filePath)
	filenameNoExt := filename.GetBaseName(filePath)

	// 1. Parse filename for initial metadata
	meta := filename.Parse(filenameNoExt)

	// 2. Get MediaInfo and merge
	mi, err := mediainfo.Get(filePath)
	if err == nil {
		mediaMeta := mi.GetMetadata()
		meta.Override(mediaMeta, true)
	} else {
		fmt.Printf("Warning: Could not get MediaInfo for %s: %v\n", filePath, err)
	}

	// 3. Override with CLI flags
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
	if episodeTitleFlag != "" {
		meta.EpisodeTitle = episodeTitleFlag
	}
	if cmd.Flags().Changed("tv") {
		meta.IsTV = isTVFlag
	} else if cmd.Flags().Changed("movie") {
		meta.IsTV = !isMovieFlag
	}
	if serviceFlag != "" {
		meta.Service = serviceFlag
	}
	if sourceFlag != "" {
		meta.Source = sourceFlag
	}
	if isRepackFlag {
		meta.Repack = isRepackFlag
	}
	if groupFlag != "" {
		meta.Group = groupFlag
	}

	// 4. MDB Search to get "correct" title and year
	var result *mdb.SearchResult
	imdbID := imdbIDFlag
	if imdbID == "" {
		imdbID = config.GetImdbID()
	}
	tmdbID := tmdbIDFlag
	if tmdbID == 0 {
		tmdbID = config.GetTmdbID()
	}
	tvdbID := tvdbIDFlag
	if tvdbID == 0 {
		tvdbID = config.GetTvdbID()
	}

	if imdbID != "" || tmdbID > 0 || tvdbID > 0 {
		result, _ = mdbSearch.SearchByID(imdbID, tmdbID, tvdbID, meta.IsTV)
	} else {
		searchQuery := filename.DeobfuscateTitle(meta.Title)
		result, _ = mdbSearch.FuzzySearch(searchQuery, meta.Year, meta.IsTV)
	}

	if result != nil {
		meta.Title = result.Title
		if result.Year > 0 {
			meta.Year = result.Year
		}
		if meta.IsTV && (meta.Season > 0 || meta.Episode > 0) {
			episodeResult := mdbSearch.FindEpisode(*result, meta.Season, meta.Episode)
			if episodeResult.Name != "" {
				meta.EpisodeTitle = episodeResult.Name
			}
		}
	}

	// 6. Apply normalization to Title, EpisodeTitle and Service
	meta.Title = filename.NormalizeTitle(meta.Title)
	if meta.EpisodeTitle != "" {
		meta.EpisodeTitle = filename.NormalizeTitle(meta.EpisodeTitle)
	}
	if meta.Service != "" {
		meta.Service = filename.NormalizeService(meta.Service)
	}

	// 7. Set defaults for missing fields (Source, Group)
	meta.SetDefaults()

	// 8. Generate new name
	newName := meta.String() + ext
	newPath := filepath.Join(filepath.Dir(filePath), newName)

	if filepath.Base(filePath) == newName {
		fmt.Printf("File '%s' already has the correct name.\n", filepath.Base(filePath))
		return
	}

	fmt.Printf("Renaming:\n  Old: %s\n  New: %s\n", filepath.Base(filePath), newName)

	if dryRunFlag {
		fmt.Println("Dry run: no changes made.")
		return
	}

	if !unattendedFlag {
		fmt.Print("Proceed with rename? [y/N] ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Skipping...")
			return
		}
	}

	renameErr := os.Rename(filePath, newPath)
	if renameErr != nil {
		fmt.Printf("Error renaming file %s: %v\n", filePath, renameErr)
	} else {
		fmt.Println("File renamed successfully.")
	}

}

func init() {
	rootCmd.AddCommand(renameCmd)
	// Metadata
	renameCmd.Flags().StringVarP(&titleFlag, "title", "t", "", "title of the movie or TV show")
	renameCmd.Flags().IntVarP(&yearFlag, "year", "y", 0, "release year")
	renameCmd.Flags().IntVarP(&seasonFlag, "season", "s", 0, "season number")
	renameCmd.Flags().IntVarP(&episodeFlag, "episode", "e", 0, "episode number")
	renameCmd.Flags().StringVarP(&dateFlag, "date", "D", "", "episode aired date (YYYY-MM-DD)")
	renameCmd.Flags().StringVar(&episodeTitleFlag, "episode-title", "", "episode title")
	// P2P
	renameCmd.Flags().StringVarP(&serviceFlag, "service", "S", "", "streaming service")
	renameCmd.Flags().StringVarP(&sourceFlag, "source", "o", "", "source (WEB-DL, BluRay, etc.)")
	renameCmd.Flags().BoolVarP(&isRepackFlag, "repack", "R", false, "is repack")
	renameCmd.Flags().StringVarP(&groupFlag, "group", "g", "", "release group")
	// MDB ID
	renameCmd.Flags().BoolVarP(&isTVFlag, "tv", "T", false, "identify as TV show")
	renameCmd.Flags().BoolVarP(&isMovieFlag, "movie", "M", false, "identify as movie")
	renameCmd.Flags().StringVar(&imdbIDFlag, "imdb", "", "IMDb ID")
	renameCmd.Flags().IntVar(&tmdbIDFlag, "tmdb", 0, "TMDB ID")
	renameCmd.Flags().IntVar(&tvdbIDFlag, "tvdb", 0, "TVDB ID")
	// Other
	renameCmd.Flags().BoolVarP(&unattendedFlag, "unattended", "u", false, "unattended mode (do not prompt for confirmation)")
	renameCmd.Flags().BoolVarP(&dryRunFlag, "dry-run", "d", false, "only print the new filename without renaming")

	// Group metadata flags
	metadataFlags := []string{"title", "year", "season", "episode", "date", "episode-title"}
	for _, f := range metadataFlags {
		renameCmd.Flags().SetAnnotation(f, "group", []string{"metadata"})
	}
	p2pFlags := []string{"service", "source", "repack", "group"}
	for _, f := range p2pFlags {
		renameCmd.Flags().SetAnnotation(f, "group", []string{"p2p"})
	}

	// Group ID flags
	idFlags := []string{"tv", "movie", "imdb", "tmdb", "tvdb"}
	for _, f := range idFlags {
		renameCmd.Flags().SetAnnotation(f, "group", []string{"id"})
	}

	renameCmd.Flags().SortFlags = false
}
