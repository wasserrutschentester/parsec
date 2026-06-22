// Package cmd provides the command line interface for parsec.
package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/mdb"
	mdbSearch "codeberg.org/upPollo/parsec/internal/mdb/search"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/filename"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/metadata/mediainfo"
	"codeberg.org/upPollo/parsec/internal/ui"
)

var (
	errRename           = errors.New("rename failed")
	errMediaInfoParsing = errors.New("mediainfo parsing failed")
	seasonPackFlag      bool
)

// renameCmd represents the rename command
var renameCmd = &cobra.Command{
	Use:   "rename [path...]",
	Short: "rename files according to metadata and MDB data",
	Long: fmt.Sprintf("%s\n%s", ui.Banner(".: ALIGNING THE SHIP :."),
		`Rename files based on information from:
  1. MediaInfo (resolution, codecs, etc.)
  2. Media Databases (TMDB/TVDB for correct title and episode name)
  3. Information already in the filename
  4. CLI flags (to override or provide missing info)

You can pass files or directories. Directories are scanned recursively for Matroska files.
The resulting filename is generated according to the configured template.`),
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Println(ui.Banner(".: VECTOR REALIGNMENT :."))

		expandedArgs := expandArgs(args)
		for _, filePath := range expandedArgs {
			err := renameFile(cmd, filePath)
			if err != nil {
				return err
			}
		}

		return nil
	},
}

func renameFile(cmd *cobra.Command, filePath string) error {
	meta, ext, err := gatherRenameMetadata(cmd, filePath)
	if err != nil {
		return err
	}

	return commitRenameFile(filePath, meta, ext)
}

// gatherRenameMetadata runs the full metadata pipeline (filename, MediaInfo,
// CLI flags and MDB search) and returns the enriched metadata and file extension.
func gatherRenameMetadata(cmd *cobra.Command, filePath string) (*metadata.Metadata, string, error) {
	ext := filepath.Ext(filePath)
	filenameNoExt := filename.GetBaseName(filePath)

	// 1. Parse filename for initial metadata
	meta := filename.Parse(filenameNoExt)

	// 2. Get MediaInfo/EBML and merge
	mi, err := renameGetMediaMetadata(filePath, meta)
	if err != nil {
		return nil, "", err
	}

	// 2.3 Apply MDB IDs from file tags
	renameApplyMdbIDs(cmd, meta, mi)

	// 3. Override with CLI flags
	applyMetadataFlags(cmd, meta)

	// 4. MDB Search to get "correct" title and year
	renameApplyMdbSearch(meta)

	// 6. Apply normalization
	renameApplyNormalization(meta)

	if meta.Service != "" {
		meta.Service = filename.NormalizeService(meta.Service)
	}

	// 7. Set defaults for missing fields (Source, Group)
	meta.SetDefaults()

	return meta, ext, nil
}

// commitRenameFile generates the target name and renames the file unless it is
// already correctly named.
func commitRenameFile(filePath string, meta *metadata.Metadata, ext string) error {
	newNameBase := meta.GetReleaseName()
	newNameBase = filename.ApplyReplacements(newNameBase, config.GetOutputReplacements())
	newName := newNameBase + ext

	destDir := filepath.Dir(filePath)

	outputPath := outputPathFlag
	if outputPath == "" {
		outputPath = config.GetOutputPath()
	}

	if outputPath != "" {
		destDir = outputPath
	}

	if seasonPackFlag && meta.IsTV && meta.Season >= 0 {
		seasonPackName := meta.GetSeasonPackName()

		absDestDir, _ := filepath.Abs(destDir)
		if filepath.Base(absDestDir) != seasonPackName {
			destDir = filepath.Join(destDir, seasonPackName)
		}
	}

	newPath := filepath.Join(destDir, newName)

	if filePath == newPath {
		ui.Println(ui.Success.Render(fmt.Sprintf("NOMINAL: File '%s' already has the correct name.", filepath.Base(filePath))))

		return nil
	}

	return renameCommit(filePath, newPath, newName)
}

func renameCommit(filePath, newPath, newName string) error {
	ui.Println()

	oldDir := filepath.Dir(filePath)
	newDir := filepath.Dir(newPath)

	if oldDir != newDir {
		ui.Println(ui.FormatStringDiffAligned("Current Path", ui.AnonymizePath(oldDir), "Target Folder", ui.AnonymizePath(newDir)))
	}

	ui.Println(ui.FormatStringDiffAligned("Current Name", filepath.Base(filePath), "Proposed Name", newName))
	ui.Println()

	if dryRunFlag {
		ui.Println(ui.Muted.Render("Dry run: no changes made."))

		return nil
	}

	if !unattendedFlag {
		response := ui.Prompt(ui.Info.Render("Proceed with rename? [y/N] "))
		if response != "y" && response != "Y" {
			ui.Println(ui.Muted.Render("Skipping..."))

			return nil
		}
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		ui.PrintError(fmt.Sprintf("Error creating directory %s: %v", ui.AnonymizePath(newDir), err))

		return errRename
	}

	renameErr := os.Rename(filePath, newPath)
	if renameErr != nil {
		ui.PrintError(fmt.Sprintf("Error renaming file %s: %v", ui.AnonymizePath(filePath), renameErr))

		return errRename
	}

	ui.Println(ui.Success.Render("All systems nominal! File renamed successfully."))

	return nil
}

func renameGetMediaMetadata(filePath string, meta *metadata.Metadata) (*mediainfo.MediaInfo, error) {
	// 2.1 Get MediaInfo and merge
	mi, err := mediainfo.Get(filePath)
	if err == nil {
		mediaMeta := mi.GetMetadata()
		meta.Override(mediaMeta)
	} else {
		ui.PrintError(fmt.Sprintf("Could not get MediaInfo for %s: %v\n", ui.AnonymizePath(filePath), err))

		return nil, errMediaInfoParsing
	}

	// 2.2 Get EBML Metadata for Visual Impaired flag
	ebml, err := matroska.GetEbmlMetadata(filePath)
	if err == nil {
		if ebml.HasVisualImpairedAudio() {
			meta.HasAudioDesc = true
		}
	}

	return mi, nil
}

func renameApplyMdbSearch(meta *metadata.Metadata) {
	meta.SetDefaults()
	result, _ := mdbSearch.InteractiveSearch(meta, true)

	if result != nil {
		mdb.PrintCompactResult(*result)

		meta.Title = result.Title
		if result.Year > 0 {
			meta.Year = result.Year
		}

		if meta.IsTV {
			episodeResult := renameGetEpisodeInfo(result, meta)
			mdb.PrintCompactEpisodeResult(episodeResult)
		}
	} else {
		ui.PrintWarning("Could not find matching Result on TMDB or TVDB")
	}

	ui.PrintDebug(fmt.Sprintf("search result: %+v", result))
}

func renameApplyNormalization(meta *metadata.Metadata) {
	// Apply normalization to Title, EpisodeTitle and Service
	meta.Title = filename.NormalizeTitle(meta.Title)
	if meta.EpisodeTitle != "" {
		meta.EpisodeTitle = filename.NormalizeTitle(meta.EpisodeTitle)
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
	if (meta.Season >= 0 && len(meta.Episodes) > 0) || meta.EpisodeTitle != "" || meta.Date != "" {
		episodeResult = mdbSearch.FindEpisode(*result, meta, config.GetAllowSpecials())

		if episodeResult.Name != "" {
			meta.EpisodeTitle = episodeResult.Name
			meta.Season = episodeResult.Season
			// Note: this overrides episodes with just the FIRST found episode's ID if we only found one,
			// wait, mdbSearch.FindEpisode should probably return all episodes if there are multiple.
			// I'll fix this in the next replacement. Let's just leave it for a sec.
			// Actually we will handle this in FindEpisode by returning a combined EpisodeResult.
			// For now, assume it returns the unified object.
			// However, since we matched them, we should probably just keep meta.Episodes intact unless we only searched by title/date.
			if len(meta.Episodes) == 0 {
				meta.Episodes = []int{episodeResult.Episode}
			}
		}
	}

	// Correct a date-based release's date from the authoritative aired date.
	if meta.Date != "" && episodeResult.Airdate != "" {
		meta.Date = episodeResult.Airdate
	}

	return episodeResult
}

func init() {
	rootCmd.AddCommand(renameCmd)
	registerRenameFlags(renameCmd)
}

// registerRenameFlags registers rename's metadata, P2P and ID override flags,
// including their usage groups.
func registerRenameFlags(cmd *cobra.Command) {
	// Metadata
	cmd.Flags().StringVarP(&titleFlag, "title", "t", "", "title of the movie or TV show")
	cmd.Flags().IntVarP(&yearFlag, "year", "y", 0, "release year")
	cmd.Flags().IntVarP(&seasonFlag, "season", "s", 0, "season number")
	cmd.Flags().IntVarP(&episodeFlag, "episode", "e", 0, "episode number")
	cmd.Flags().StringVarP(&dateFlag, "date", "D", "", "episode aired date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&episodeTitleFlag, "episode-title", "", "episode title")
	cmd.Flags().StringVar(&cutEditionFlag, "cut-edition", "", "special edition or cut")
	cmd.Flags().StringVar(&hdrFlag, "hdr", "", "HDR format")
	// P2P
	cmd.Flags().StringVarP(&serviceFlag, "service", "S", "", "streaming service")
	cmd.Flags().StringVarP(&sourceFlag, "source", "o", "", "source (WEB-DL, BluRay, etc.)")
	cmd.Flags().BoolVarP(&isRepackFlag, "repack", "R", false, "is repack")
	cmd.Flags().BoolVar(&isSubbedFlag, "subbed", false, "has subtitles in the preferred language")
	cmd.Flags().BoolVar(&isAudioDescFlag, "audio-description", false, "add audio description tag")
	cmd.Flags().StringVarP(&groupFlag, "group", "g", "", "release group")
	// MDB ID
	cmd.Flags().BoolVarP(&isTVFlag, "tv", "T", false, "identify as TV show")
	cmd.Flags().BoolVarP(&isMovieFlag, "movie", "M", false, "identify as movie")
	cmd.Flags().StringVar(&imdbIDFlag, "imdb", "", "IMDb ID")
	cmd.Flags().IntVar(&tmdbIDFlag, "tmdb", 0, "TMDB ID")
	cmd.Flags().IntVar(&tvdbIDFlag, "tvdb", 0, "TVDB ID")
	// Other
	cmd.Flags().BoolVarP(&unattendedFlag, "unattended", "u", false, "unattended mode (do not prompt for confirmation)")
	cmd.Flags().BoolVarP(&dryRunFlag, "dry-run", "d", false, "only print the new filename without renaming")
	cmd.Flags().BoolVarP(&seasonPackFlag, "season-pack", "P", false, "move episodes into a correctly named season pack folder")
	cmd.Flags().StringVarP(&outputPathFlag, "output", "O", "", "output path where to move the files after renaming")

	// Group metadata flags
	metadataFlags := []string{"title", "year", "season", "episode", "date", "episode-title"}
	for _, f := range metadataFlags {
		_ = cmd.Flags().SetAnnotation(f, "group", []string{"metadata"})
	}

	p2pFlags := []string{"service", "source", "repack", "group"}
	for _, f := range p2pFlags {
		_ = cmd.Flags().SetAnnotation(f, "group", []string{"p2p"})
	}

	// Group ID flags
	idFlags := []string{"tv", "movie", "imdb", "tmdb", "tvdb"}
	for _, f := range idFlags {
		_ = cmd.Flags().SetAnnotation(f, "group", []string{"id"})
	}

	cmd.Flags().SortFlags = false
}
