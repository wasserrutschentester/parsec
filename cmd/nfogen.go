package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/mdb"
	"codeberg.org/upPollo/parsec/internal/mdb/search"
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
)

func init() {
	rootCmd.AddCommand(nfogenCmd)
	nfogenCmd.Flags().StringVarP(&nfogenTemplateFlag, "template", "t", "", "NFO template to use (builtin: default; or name of .tmpl in config dir, or default via config)")
	nfogenCmd.Flags().StringVar(&notesFlag, "notes", "", "custom notes to embed in the NFO")
	nfogenCmd.Flags().BoolVar(&nfogenFullDiffFlag, "full-diff", false, "show full context for NFO diffs instead of just changed lines")
	nfogenCmd.Flags().StringSliceVar(&sourcesFlag, "source", []string{}, "add a source release name (can be used multiple times)")
	nfogenCmd.Flags().StringSliceVar(&sourceMapFlag, "source-map", []string{}, "map tracks to a source (e.g., 'v1,a1-3:Release-Name')")
	nfogenCmd.Flags().BoolVarP(&unattendedFlag, "unattended", "u", false, "run in unattended mode")
	nfogenCmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "print NFO output to console without writing to disk")
	nfogenCmd.Flags().BoolVar(&dumpContextFlag, "dump-context", false, "dump the template context data as JSON (hides raw fields)")
	nfogenCmd.Flags().BoolVar(&dumpContextRawFlag, "dump-context-raw", false, "dump the template context data as JSON including all raw provider data")

	_ = nfogenCmd.RegisterFlagCompletionFunc("template", completeTemplates)
}

var nfogenCmd = &cobra.Command{
	Use:   "nfogen <file>",
	Short: "Generate an NFO file for a media file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if !cmd.Flags().Changed("template") {
			nfogenTemplateFlag = config.GetNfogenTemplate()
		}

		file := args[0]

		filenameNoExt := filename.GetBaseName(file)
		meta := filename.Parse(filenameNoExt)

		info, err := mediainfo.Get(file)
		if err != nil {
			ui.PrintWarning("Could not parse mediainfo: " + err.Error() + ". Generating minimal NFO.")
		}

		var searchResult *mdb.SearchResult

		var episodeResults []mdb.EpisodeResult

		res, err := search.InteractiveSearch(meta, unattendedFlag)
		if err == nil && res != nil {
			searchResult = res
			if searchResult.IsTV && len(meta.Episodes) > 0 {
				episodeResults = search.FindEpisodes(*searchResult, meta, true)
			}
		} else if err != nil {
			ui.PrintWarning("Search failed: " + err.Error() + ". Generating minimal NFO.")
		}

		var ebmlMeta *matroska.EbmlMetadata
		if strings.ToLower(filepath.Ext(file)) == ".mkv" {
			ebmlMeta, _ = matroska.GetEbmlMetadata(file)
		}

		ctx := nfo.BuildContext(meta, info, file, notesFlag, searchResult, episodeResults, ebmlMeta, Version)
		ctx.Sources = sourcesFlag

		if err := nfo.ApplySourceMap(ctx, sourceMapFlag); err != nil {
			ui.PrintError("Failed to apply source map: " + err.Error())

			return
		}

		if dumpContextFlag || dumpContextRawFlag {
			dumpCtx := *ctx
			if !dumpContextRawFlag {
				dumpCtx.RawSearchResult = nil
				dumpCtx.RawEpisodeResults = nil
				dumpCtx.RawMediaInfo = nil
				dumpCtx.RawEbmlMetadata = nil
			}

			//nolint:musttag // debugging output doesn't need strict json tags on the entire tree
			b, err := json.MarshalIndent(dumpCtx, "", "  ")
			if err != nil {
				ui.PrintError("Failed to dump context: " + err.Error())

				return
			}

			ui.Println(string(b))

			return
		}

		out, err := nfo.Render(ctx, nfogenTemplateFlag)
		if err != nil {
			ui.PrintError("Failed to render NFO: " + err.Error())

			return
		}

		if dryRunFlag {
			ui.Println("Dry run. NFO Output:")
			ui.Println(out)

			return
		}

		nfoFile := strings.TrimSuffix(file, filepath.Ext(file)) + ".nfo"

		// Check if it already exists
		if existing, err := os.ReadFile(nfoFile); err == nil {
			existingNFO := string(existing)

			if existingNFO == out {
				ui.PrintSuccess("NFO file is already up to date.")

				return
			}

			ui.PrintWarning("NFO file already exists and differs from generated output.")
			ui.Println("Diff:")
			printNFODiff(existingNFO, out, nfogenFullDiffFlag)
			ui.Println("\nUse --force to overwrite the existing NFO file.")

			return
		}

		if !unattendedFlag {
			ui.Println("Generated NFO Preview:")
			ui.Println(out)
			fmt.Print(ui.Info.Render("Proceed with generating NFO? [y/N] "))

			var response string

			_, _ = fmt.Scanln(&response)
			if response != "y" && response != "Y" {
				ui.Println(ui.Muted.Render("Skipping..."))

				return
			}
		}

		err = os.WriteFile(nfoFile, []byte(out), 0o644)
		if err != nil {
			ui.PrintError("Failed to write NFO file: " + err.Error())

			return
		}

		ui.PrintSuccess("Successfully generated NFO: " + nfoFile)
	},
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
