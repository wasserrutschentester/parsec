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
	"codeberg.org/n0ne/parsec/internal/metadata"
	"codeberg.org/n0ne/parsec/internal/metadata/filename"
	"codeberg.org/n0ne/parsec/internal/metadata/matroska"
	"codeberg.org/n0ne/parsec/internal/metadata/mediainfo"
	"codeberg.org/n0ne/parsec/internal/ui"
	"github.com/spf13/cobra"
)

// renameCmd represents the rename command
var renameCmd = &cobra.Command{
	Use:   "rename [file...]",
	Short: "rename files according to metadata and MDB data",
	Long: fmt.Sprintf("%s\n%s", ui.Banner(".: ALIGNING THE SHIP :."),
		`Rename files based on information from:
  1. MediaInfo (resolution, codecs, etc.)
  2. Media Databases (TMDB/TVDB for correct title and episode name)
  3. Information already in the filename
  4. CLI flags (to override or provide missing info)

The resulting filename is generated according to the configured template.`),
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, filePath := range args {
			err := renameFile(cmd, filePath)
			if err != nil {
				return err
			}
		}
		return nil
	},
}

func renameFile(cmd *cobra.Command, filePath string) error {
	ext := filepath.Ext(filePath)
	filenameNoExt := filename.GetBaseName(filePath)

	// 1. Parse filename for initial metadata
	meta := filename.Parse(filenameNoExt)

	// 2.1 Get MediaInfo and merge
	mi, err := mediainfo.Get(filePath)
	if err == nil {
		mediaMeta := mi.GetMetadata()
		meta.Override(mediaMeta)
	} else {
		ui.PrintError(fmt.Sprintf("Could not get MediaInfo for %s: %v\n", filePath, err))
		return fmt.Errorf("mediainfo parsing failed")
	}

	// 2.2 Get EBML Metadata for Visual Impaired flag
	ebml, err := matroska.GetEbmlMetadata(filePath)
	if err == nil {
		if ebml.HasVisualImpairedAudio() {
			meta.HasAudioDesc = true
		}
	}

	// 2.3 Apply MDB IDs from file tags
	renameApplyMdbIDs(cmd, meta, mi)

	// 3. Override with CLI flags
	applyMetadataFlags(cmd, meta)

	// 4. MDB Search to get "correct" title and year
	meta.SetDefaults()
	result, _ := mdbSearch.InteractiveSearch(meta, true)

	if result != nil {
		meta.Title = result.Title
		if result.Year > 0 {
			meta.Year = result.Year
		}
		if meta.IsTV {
			renameGetEpisodeInfo(result, meta)
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
	newName := meta.GetReleaseName() + ext
	newPath := filepath.Join(filepath.Dir(filePath), newName)

	if filepath.Base(filePath) == newName {
		ui.Println(ui.Success.Render(fmt.Sprintf("NOMINAL: File '%s' already has the correct name.", filepath.Base(filePath))))
		return nil
	}

	ui.Println(ui.Banner(".: VECTOR REALIGNMENT :."))
	ui.Println(ui.FormatStringDiffAligned("Current Heading", filepath.Base(filePath), "Proposed Vector", newName))
	ui.Println()

	if dryRunFlag {
		ui.Println(ui.Muted.Render("Dry run: no changes made."))
		return nil
	}

	if !unattendedFlag {
		fmt.Print(ui.Info.Render("Proceed with rename? [y/N] "))
		var response string
		_, _ = fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			ui.Println(ui.Muted.Render("Skipping..."))
			return nil
		}
	}

	renameErr := os.Rename(filePath, newPath)
	if renameErr != nil {
		ui.PrintError(fmt.Sprintf("Error renaming file %s: %v", filePath, renameErr))
		return fmt.Errorf("rename failed")
	} else {
		ui.Println(ui.Success.Render("All systems nominal! File renamed successfully."))
		return nil
	}
}

func renameApplyMdbIDs(cmd *cobra.Command, meta *metadata.Metadata, mi *mediainfo.MediaInfo) {
	tagImdb, tagTmdb, tagTvdb, tagIsTV := mi.GetMdbIDs()
	if meta.ImdbID == "" {
		meta.ImdbID = tagImdb
	}
	if meta.TmdbID == 0 {
		meta.TmdbID = tagTmdb
	}
	if meta.TvdbID == 0 {
		meta.TvdbID = tagTvdb
	}
	if !cmd.Flags().Changed("tv") && !cmd.Flags().Changed("movie") && tagIsTV {
		meta.IsTV = true
	}
}

func renameGetEpisodeInfo(result *mdb.SearchResult, meta *metadata.Metadata) mdb.EpisodeResult {
	var episodeResult mdb.EpisodeResult
	if (meta.Season > 0 && meta.Episode > 0) || meta.EpisodeTitle != "" || meta.Date != "" {
		episodeResult = mdbSearch.FindEpisode(*result, meta, config.GetAllowSpecials())
	}
	if episodeResult.Name != "" {
		meta.EpisodeTitle = episodeResult.Name
		meta.Season = episodeResult.Season
		meta.Episode = episodeResult.Episode
	}
	return episodeResult
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
	renameCmd.Flags().StringVar(&cutEditionFlag, "cut-edition", "", "special edition or cut")
	renameCmd.Flags().StringVar(&hdrFlag, "hdr", "", "HDR format")
	// P2P
	renameCmd.Flags().StringVarP(&serviceFlag, "service", "S", "", "streaming service")
	renameCmd.Flags().StringVarP(&sourceFlag, "source", "o", "", "source (WEB-DL, BluRay, etc.)")
	renameCmd.Flags().BoolVarP(&isRepackFlag, "repack", "R", false, "is repack")
	renameCmd.Flags().BoolVar(&isSubbedFlag, "subbed", false, "has subtitles in the preferred language")
	renameCmd.Flags().BoolVar(&isAudioDescFlag, "audio-description", false, "add audio description tag")
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
		_ = renameCmd.Flags().SetAnnotation(f, "group", []string{"metadata"})
	}
	p2pFlags := []string{"service", "source", "repack", "group"}
	for _, f := range p2pFlags {
		_ = renameCmd.Flags().SetAnnotation(f, "group", []string{"p2p"})
	}

	// Group ID flags
	idFlags := []string{"tv", "movie", "imdb", "tmdb", "tvdb"}
	for _, f := range idFlags {
		_ = renameCmd.Flags().SetAnnotation(f, "group", []string{"id"})
	}

	renameCmd.Flags().SortFlags = false
}
