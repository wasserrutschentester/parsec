package cmd

import (
	"fmt"
	"path/filepath"

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

		name := filepath.Base(filePath)
		filenameNoExt := name
		if ext := filepath.Ext(name); ext != "" {
			filenameNoExt = name[:len(name)-len(ext)]
		}
		fmt.Printf("Current Name:\t%s\n", filenameNoExt)
		filename.CheckAllowedCharacters(filenameNoExt)
		filename.CheckCharacterSequences(filenameNoExt)

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
		updated := match.Override(mediaMeta) // keep only the fields that can't be parsed from MediaInfo
		if updated {
			fmt.Println("After Applying those updates the name looks like this:")
			fmt.Printf("Generated Name:\t%s\n", match)
		} else {
			fmt.Println("The Parsed name fits the specification")
		}

		// Verify Track Order
		ebml, err := metadata.GetEbmlMetadata(filePath)
		if err == nil {
			if err := metadata.VerifyTrackOrder(ebml.Tracks); err != nil {
				fmt.Printf("Track Order Error: %v\n", err)
			} else {
				fmt.Println("Track order and languages are correct")
			}
		} else {
			fmt.Printf("Error getting EBML metadata: %v\n", err)
		}

	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
}
