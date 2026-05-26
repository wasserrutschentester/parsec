package cmd

import (
	"fmt"

	"codeberg.org/n0ne/parsec/internal/checks"
	"codeberg.org/n0ne/parsec/internal/config"
	"codeberg.org/n0ne/parsec/internal/metadata"
	"codeberg.org/n0ne/parsec/internal/metadata/filename"
	"codeberg.org/n0ne/parsec/internal/metadata/mediainfo"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check [file]",
	Short: "Check if the file fits the specification",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		filePath := args[0]
		filenameNoExt := filename.GetBaseName(filePath)
		fmt.Printf("Current Name:\t%s\n", filenameNoExt)

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
			fmt.Println("Some Tags weren't parsed correctly from the filename")
			fmt.Printf("Parsed Name:\t%s\n", match)
		}

		// Generate Name from mediainfo
		mi, err := mediainfo.Get(filePath)
		if err != nil {
			fmt.Printf("Error getting mediainfo: %v\n", err)
			return
		}
		mediaMeta := mi.GetMetadata()
		updated := match.Override(mediaMeta, false) // keep only the fields that can't be parsed from MediaInfo
		if updated {
			fmt.Println("After Applying those updates the name looks like this:")
			fmt.Printf("Generated Name:\t%s\n\n", match)
		} else {
			fmt.Println("The Parsed name fits the specification")
		}

		checks.RunMediaInfoChecks(mi, match)

		// Verify Track Order
		ebml, err := metadata.GetEbmlMetadata(filePath)
		if err == nil {
			if err := metadata.VerifyTrackOrder(ebml.Tracks); err != nil {
				fmt.Printf("Track Order Error: %v\n", err)
			}

			if config.IsCheckEnabled("matroska_default_flags") {
				if err := metadata.CheckDefaultFlags(ebml.Tracks); err != nil {
					fmt.Printf("Default Flag Error: %v\n", err)
				}
			}

			if config.IsCheckEnabled("matroska_subtitle_format") {
				if err := metadata.CheckSubtitleFormat(ebml.Tracks); err != nil {
					fmt.Printf("Subtitle Format Error: %v\n", err)
				}
			}

		} else {
			fmt.Printf("Error getting EBML metadata: %v\n", err)
		}

		checks.RunGenericChecks(match)

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
		checks.RunMdbChecks(match, imdbID, tmdbID, tvdbID)
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
