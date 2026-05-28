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

var checkCmd = &cobra.Command{
	Use:   "check [file]",
	Short: "Check if the file fits the specification",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		filePath := args[0]
		filenameNoExt := filename.GetBaseName(filePath)

		ui.Println(ui.Header.Render("Parsec File Check"))
		ui.Println(ui.LabelValue("Current Name:", filenameNoExt))

		if config.IsCheckEnabled("filename_characters") {
			filename.CheckAllowedCharacters(filenameNoExt)
		}
		if config.IsCheckEnabled("filename_sequences") {
			filename.CheckCharacterSequences(filenameNoExt)
		}

		// match filename against spec
		match := filename.Parse(filenameNoExt)
		matchName := match.String()
		if matchName != filenameNoExt {
			ui.Println(ui.Warning.Render("\nSome Tags weren't parsed correctly from the filename"))
			ui.Println(ui.LabelValue("Parsed Name:", match.String()))
		}

		// Generate Name from mediainfo
		mi, err := mediainfo.Get(filePath)
		if err != nil {
			ui.PrintError(fmt.Sprintf("Error getting mediainfo: %v", err))
			return
		}
		mediaMeta := mi.GetMetadata()
		updated := match.Override(mediaMeta, false) // keep only the fields that can't be parsed from MediaInfo

		// 3. Get EBML Metadata for Visual Impaired flag
		ebml, err := matroska.GetEbmlMetadata(filePath)
		if err == nil {
			if ebml.HasVisualImpairedAudio() {
				if !match.HasAudioDesc {
					match.HasAudioDesc = true
					updated = true
				}
			}
		}

		if updated {
			ui.Println(ui.Info.Render("\nUpdates applied from MediaInfo/EBML:"))
			ui.Println(ui.LabelValue("Generated Name:", match.String()))
			ui.Println()
		} else {
			ui.Println(ui.Success.Render("\nThe parsed name fits the specification"))
		}

		checks.RunMediaInfoChecks(mi, match)

		// 4. Run EBML specific checks
		if err == nil {
			if err := matroska.VerifyTrackOrder(ebml.Tracks); err != nil {
				ui.PrintWarning(fmt.Sprintf("Track Order Error: %v", err))
			}

			if config.IsCheckEnabled("matroska_default_flags") {
				if err := matroska.CheckDefaultFlags(ebml.Tracks); err != nil {
					ui.PrintWarning(fmt.Sprintf("Default Flag Error: %v", err))
				}
			}

			if config.IsCheckEnabled("matroska_subtitle_format") {
				if err := matroska.CheckSubtitleFormat(ebml.Tracks); err != nil {
					ui.PrintWarning(fmt.Sprintf("Subtitle Format Error: %v", err))
				}
			}

		} else {
			ui.PrintWarning(fmt.Sprintf("Error getting EBML metadata: %v", err))
		}

		checks.RunGenericChecks(match)

		// Get IDs from file tags
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

		// checks.RunMdbChecks(mi, match)
		var test string
		fmt.Scanln(&test)
		fmt.Println(test)
	},
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
