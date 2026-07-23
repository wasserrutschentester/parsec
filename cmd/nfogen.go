package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/mdb"
	"codeberg.org/upPollo/parsec/internal/mdb/search"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/filename"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/metadata/mediainfo"
	"codeberg.org/upPollo/parsec/internal/nfo"
	"codeberg.org/upPollo/parsec/internal/ui"
)

var (
	nfogenTemplateFlag string
	notesFlag          string
	nfogenFullDiffFlag bool
	sourcesFlag        []string
	sourceMapFlag      []string
	dumpContextFlag    bool
	dumpContextRawFlag bool
	errNoMediaFiles    = errors.New("no media files found in directory")
)

func init() {
	rootCmd.AddCommand(nfogenCmd)
	// P2P info
	nfogenCmd.Flags().StringVar(&notesFlag, "notes", "", "custom notes to embed in the NFO")
	nfogenCmd.Flags().StringSliceVar(&sourcesFlag, "source", []string{}, "add a source release name (can be used multiple times)")
	nfogenCmd.Flags().StringSliceVar(&sourceMapFlag, "source-map", []string{}, "map tracks to a source (e.g., 'v1,a1-3:Release-Name')")
	// Output
	nfogenCmd.Flags().BoolVar(&dumpContextFlag, "dump-context", false, "dump the template context data as JSON (hides raw fields)")
	nfogenCmd.Flags().BoolVar(&dumpContextRawFlag, "dump-context-raw", false, "dump the template context data as JSON including all raw provider data")
	nfogenCmd.Flags().BoolVar(&nfogenFullDiffFlag, "full-diff", false, "show full context for NFO diffs instead of just changed lines")
	// interaction
	nfogenCmd.Flags().BoolVarP(&unattendedFlag, "unattended", "u", false, "run in unattended mode")
	nfogenCmd.Flags().BoolVarP(&dryRunFlag, "dry-run", "d", false, "print NFO output to console without writing to disk")
	// other
	nfogenCmd.Flags().StringVarP(&nfogenTemplateFlag, "template", "t", "", "NFO template to use (builtin: default; or name of .tmpl in config dir, or default via config)")
	_ = nfogenCmd.RegisterFlagCompletionFunc("template", completeTemplates)

	// group Flags
	p2pFlags := []string{"notes", "source", "source-map"}
	for _, f := range p2pFlags {
		_ = nfogenCmd.Flags().SetAnnotation(f, "group", []string{"p2p"})
	}

	outputFlags := []string{"full-diff", "dump-context", "dump-context-raw"}
	for _, f := range outputFlags {
		_ = nfogenCmd.Flags().SetAnnotation(f, "group", []string{"output"})
	}
}

var nfogenCmd = &cobra.Command{
	Use:   "nfogen <file|directory>",
	Short: "Generate an NFO file for a media file or pack",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if !cmd.Flags().Changed("template") {
			nfogenTemplateFlag = config.GetNfogenTemplate()
		}

		targetPath := args[0]

		mediaFiles, releaseName, nfoFile, err := getTargetFiles(targetPath)
		if err != nil {
			ui.PrintError(err.Error())

			return
		}

		baseMeta := filename.Parse(releaseName)
		searchResult := performSearch(baseMeta, unattendedFlag)
		fileInputs := parseMediaFiles(mediaFiles, searchResult)

		ctx := nfo.BuildContext(releaseName, baseMeta, fileInputs, notesFlag, searchResult, Version)
		ctx.Sources = sourcesFlag

		if err := nfo.ApplySourceMap(ctx, sourceMapFlag); err != nil {
			ui.PrintError("Failed to apply source map: " + err.Error())

			return
		}

		if dumpContextFlag || dumpContextRawFlag {
			dumpNfoContext(ctx, dumpContextRawFlag)

			return
		}

		out, err := nfo.Render(ctx, nfogenTemplateFlag)
		if err != nil {
			ui.PrintError("Failed to render NFO: " + err.Error())

			return
		}

		if err := handleNfoOutput(nfoFile, out, dryRunFlag, unattendedFlag, nfogenFullDiffFlag); err != nil {
			ui.PrintError(err.Error())
		}
	},
}

func getTargetFiles(targetPath string) (mediaFiles []string, releaseName string, nfoFile string, err error) {
	stat, err := os.Stat(targetPath)
	if err != nil {
		return nil, "", "", fmt.Errorf("cannot access target: %w", err)
	}

	if stat.IsDir() {
		return getTargetFilesFromDir(targetPath)
	}

	releaseName = filename.GetBaseName(targetPath)
	nfoFile = strings.TrimSuffix(targetPath, filepath.Ext(targetPath)) + ".nfo"
	mediaFiles = []string{targetPath}

	return mediaFiles, releaseName, nfoFile, nil
}

func getTargetFilesFromDir(targetPath string) (mediaFiles []string, releaseName string, nfoFile string, err error) {
	releaseName = filepath.Base(targetPath)
	nfoFile = filepath.Join(targetPath, releaseName+".nfo")

	entries, err := os.ReadDir(targetPath)
	if err != nil {
		return nil, "", "", fmt.Errorf("cannot read directory: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext == ".mkv" || ext == ".mp4" || ext == ".avi" {
			mediaFiles = append(mediaFiles, filepath.Join(targetPath, e.Name()))
		}
	}

	if len(mediaFiles) == 0 {
		return nil, "", "", errNoMediaFiles
	}

	sort.Strings(mediaFiles)

	return mediaFiles, releaseName, nfoFile, nil
}

func performSearch(baseMeta *metadata.Metadata, unattended bool) *mdb.SearchResult {
	var searchResult *mdb.SearchResult

	res, err := search.InteractiveSearch(baseMeta, unattended)
	if err == nil && res != nil {
		searchResult = res
	} else if err != nil {
		ui.PrintWarning("Search failed: " + err.Error() + ". Generating minimal NFO.")
	}

	return searchResult
}

func parseMediaFiles(mediaFiles []string, searchResult *mdb.SearchResult) []nfo.FileInput {
	fileInputs := make([]nfo.FileInput, 0, len(mediaFiles))

	for _, file := range mediaFiles {
		fileMeta := filename.Parse(filename.GetBaseName(file))

		info, err := mediainfo.Get(file)
		if err != nil {
			ui.PrintWarning(fmt.Sprintf("Could not parse mediainfo for %s: %s", filepath.Base(file), err.Error()))
		}

		var episodeResults []mdb.EpisodeResult
		if searchResult != nil && searchResult.IsTV && len(fileMeta.Episodes) > 0 {
			episodeResults = search.FindEpisodes(*searchResult, fileMeta, true)
		}

		var ebmlMeta *matroska.EbmlMetadata
		if strings.ToLower(filepath.Ext(file)) == ".mkv" {
			ebmlMeta, _ = matroska.GetEbmlMetadata(file)
		}

		fileInputs = append(fileInputs, nfo.FileInput{
			Path:           file,
			Meta:           fileMeta,
			Info:           info,
			Ebml:           ebmlMeta,
			EpisodeResults: episodeResults,
		})
	}

	return fileInputs
}

func dumpNfoContext(ctx *nfo.Context, raw bool) {
	dumpCtx := *ctx
	if !raw {
		dumpCtx.RawSearchResult = nil
		dumpCtx.RawEpisodeResults = nil
		dumpCtx.RawMediaInfo = nil

		dumpCtx.RawEbmlMetadata = nil
		for i := range dumpCtx.Files {
			dumpCtx.Files[i].RawEpisodeResults = nil
			dumpCtx.Files[i].RawMediaInfo = nil
			dumpCtx.Files[i].RawEbmlMetadata = nil
		}
	}

	//nolint:musttag // debugging output doesn't need strict json tags on the entire tree
	b, err := json.MarshalIndent(dumpCtx, "", "  ")
	if err != nil {
		ui.PrintError("Failed to dump context: " + err.Error())

		return
	}

	ui.Println(string(b))
}

func handleNfoOutput(nfoFile string, out string, dryRun bool, unattended bool, fullDiff bool) error {
	if dryRun {
		ui.Println("Dry run. NFO Output:")
		ui.Println(out)

		return nil
	}

	// Check if it already exists
	if existing, err := os.ReadFile(nfoFile); err == nil {
		existingNFO := string(existing)

		if existingNFO == out {
			ui.PrintSuccess("NFO file is already up to date.")

			return nil
		}

		ui.PrintWarning("NFO file already exists and differs from generated output.")
		ui.Println("Diff:")
		printNFODiff(existingNFO, out, fullDiff)

		return nil
	}

	if !unattended {
		ui.Println("Generated NFO Preview:")
		ui.Println(out)
		fmt.Print(ui.Info.Render("Proceed with generating NFO? [y/N] "))

		var response string

		_, _ = fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			ui.Println(ui.Muted.Render("Skipping..."))

			return nil
		}
	}

	err := os.WriteFile(nfoFile, []byte(out), 0o644)
	if err != nil {
		return fmt.Errorf("failed to write NFO file: %w", err)
	}

	ui.PrintSuccess("Successfully generated NFO: " + nfoFile)

	return nil
}

func printNFODiff(oldStr, newStr string, showFull bool) {
	oldStr = strings.ReplaceAll(oldStr, "\r\n", "\n")
	newStr = strings.ReplaceAll(newStr, "\r\n", "\n")

	oldLines := strings.Split(strings.TrimSpace(oldStr), "\n")
	newLines := strings.Split(strings.TrimSpace(newStr), "\n")

	// Basic line-by-line comparison (not a full Myers diff, but good enough for static templates)
	maxLines := max(len(newLines), len(oldLines))

	for i := range maxLines {
		var oLine, nLine string
		if i < len(oldLines) {
			oLine = oldLines[i]
		}

		if i < len(newLines) {
			nLine = newLines[i]
		}

		if oLine != nLine {
			if i < len(oldLines) {
				ui.Println("\033[31m- " + oLine + "\033[0m")
			}

			if i < len(newLines) {
				ui.Println("\033[32m+ " + nLine + "\033[0m")
			}
		} else if showFull {
			if i < len(oldLines) {
				ui.Println("  " + oLine)
			}
		}
	}
}
