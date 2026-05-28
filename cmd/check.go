package cmd

import (
	"fmt"

	"codeberg.org/n0ne/parsec/internal/checks"
	"codeberg.org/n0ne/parsec/internal/config"
	"codeberg.org/n0ne/parsec/internal/metadata/filename"
	"codeberg.org/n0ne/parsec/internal/metadata/matroska"
	"codeberg.org/n0ne/parsec/internal/metadata/mediainfo"
	"codeberg.org/n0ne/parsec/internal/ui"

	"github.com/spf13/cobra"
)

type issueGroup struct {
	Category string
	Issues   []string
}

var checkCmd = &cobra.Command{
	Use:   "check [file]",
	Short: "Check if the file fits the specification",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		filePath := args[0]
		filenameNoExt := filename.GetBaseName(filePath)

		ui.Println(ui.Header.Render("Parsec File Check"))
		ui.Println(ui.LabelValue("Current Name:", filenameNoExt))

		var allIssues []issueGroup

		// 1. Filename Basic Checks
		var fnIssues []string
		if config.IsCheckEnabled("filename_characters") {
			if msg := filename.CheckAllowedCharacters(filenameNoExt); msg != "" {
				fnIssues = append(fnIssues, msg)
			}
		}
		if config.IsCheckEnabled("filename_sequences") {
			if msg := filename.CheckCharacterSequences(filenameNoExt); msg != "" {
				fnIssues = append(fnIssues, msg)
			}
		}

		match := filename.Parse(filenameNoExt)
		if match.String() != filenameNoExt {
			fnIssues = append(fnIssues, ui.Warning.Render("Some tags weren't parsed correctly from the filename"))
			fnIssues = append(fnIssues, ui.LabelValue("Parsed Name:", match.String()))
		}
		if len(fnIssues) > 0 {
			allIssues = append(allIssues, issueGroup{"FILENAME", fnIssues})
		}

		// 2. MediaInfo & EBML Checks
		mi, err := mediainfo.Get(filePath)
		if err != nil {
			ui.PrintError(fmt.Sprintf("Error getting mediainfo: %v", err))
			return
		}
		mediaMeta := mi.GetMetadata()
		updated := match.Override(mediaMeta, false)

		ebml, ebmlErr := matroska.GetEbmlMetadata(filePath)
		if ebmlErr == nil {
			if ebml.HasVisualImpairedAudio() && !match.HasAudioDesc {
				match.HasAudioDesc = true
				updated = true
			}
		}

		if updated {
			ui.Println(ui.Info.Render("\nUpdates applied from MediaInfo/EBML:"))
			ui.Println(ui.LabelValue("Generated Name:", match.String()))
		}

		// Collect MediaInfo issues
		miIssues := checks.RunMediaInfoChecks(mi, match)
		if len(miIssues) > 0 {
			allIssues = append(allIssues, issueGroup{"MEDIAINFO", miIssues})
		}

		// Collect EBML issues
		var ebmlIssues []string
		if ebmlErr == nil {
			ebmlIssues = append(ebmlIssues, matroska.VerifyTrackOrder(ebml.Tracks)...)
			if config.IsCheckEnabled("matroska_default_flags") {
				ebmlIssues = append(ebmlIssues, matroska.CheckDefaultFlags(ebml.Tracks)...)
			}
			if config.IsCheckEnabled("matroska_subtitle_format") {
				ebmlIssues = append(ebmlIssues, matroska.CheckSubtitleFormat(ebml.Tracks)...)
			}
		} else {
			ebmlIssues = append(ebmlIssues, fmt.Sprintf("Error getting EBML metadata: %v", ebmlErr))
		}
		if len(ebmlIssues) > 0 {
			allIssues = append(allIssues, issueGroup{"MATROSKA", ebmlIssues})
		}

		// 3. Generic Checks
		genericIssues := checks.RunGenericChecks(match)
		if len(genericIssues) > 0 {
			allIssues = append(allIssues, issueGroup{"GENERIC", genericIssues})
		}

		// 4. MDB Checks
		tagImdb, tagTmdb, tagTvdb, tagIsTV := mi.GetMdbIDs()
		if match.ImdbID == "" {
			match.ImdbID = tagImdb
		}
		if match.TmdbID == 0 {
			match.TmdbID = tagTmdb
		}
		if match.TvdbID == 0 {
			match.TvdbID = tagTvdb
		}
		if !cmd.Flags().Changed("tv") && !cmd.Flags().Changed("movie") && tagIsTV {
			match.IsTV = true
		}

		if imdbIDFlag != "" {
			match.ImdbID = imdbIDFlag
		}
		if tmdbIDFlag != 0 {
			match.TmdbID = tmdbIDFlag
		}
		if tvdbIDFlag != 0 {
			match.TvdbID = tvdbIDFlag
		}

		mdbIssues := checks.RunMdbChecks(mi, match)
		if len(mdbIssues) > 0 {
			allIssues = append(allIssues, issueGroup{"MDB", mdbIssues})
		}

		// Final Output
		if len(allIssues) == 0 {
			ui.Println("\n" + ui.IconCheck + ui.Success.Render(" All checks passed! The file fits the specification."))
		} else {
			ui.Println("\n" + ui.IconCross + ui.Error.Render(fmt.Sprintf(" %d issues found:", countIssues(allIssues))))
			for _, group := range allIssues {
				ui.Println(ui.ReportSection(group.Category))
				for _, issue := range group.Issues {
					ui.Println("  " + ui.IconWarn + " " + issue)
				}
			}
		}
		ui.Println()
	},
}

func countIssues(groups []issueGroup) int {
	count := 0
	for _, g := range groups {
		count += len(g.Issues)
	}
	return count
}

func init() {
	rootCmd.AddCommand(checkCmd)
	checkCmd.Flags().IntVar(&tmdbIDFlag, "tmdb", 0, "TMDB ID")
	checkCmd.Flags().IntVar(&tvdbIDFlag, "tvdb", 0, "TVDB ID")
	checkCmd.Flags().StringVar(&imdbIDFlag, "imdb", "", "IMDb ID")

	idFlags := []string{"imdb", "tmdb", "tvdb"}
	for _, f := range idFlags {
		checkCmd.Flags().SetAnnotation(f, "group", []string{"id"})
	}
}
