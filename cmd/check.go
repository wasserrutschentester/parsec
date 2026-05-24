package cmd

import (
	"fmt"
	"path/filepath"

	"codeberg.org/n0ne/parsec/internal/metadata"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check [file]",
	Short: "Check if the file fits the specification",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		filePath := args[0]

		filename := filepath.Base(filePath)
		filenameNoExt := filename
		if ext := filepath.Ext(filename); ext != "" {
			filenameNoExt = filename[:len(filename)-len(ext)]
		}
		fmt.Printf("Current Name:\t%s\n", filenameNoExt)
		metadata.CheckAllowedCharacters(filenameNoExt)
		metadata.CheckCharacterSequences(filenameNoExt)

		// match filename against spec
		match := metadata.ParseFilename(filenameNoExt)
		matchName := match.String()
		if matchName != filenameNoExt {
			fmt.Println("Some Tags weren't parsed correctly from the filename")
			fmt.Printf("Parsed Name:\t%s\n", match)
		}

		// Generate Name from mediainfo
		mediainfo, err := metadata.GetMediaInfo(filePath)
		if err != nil {
			fmt.Printf("Error getting mediainfo: %v\n", err)
			return
		}
		mediaMeta := mediainfo.GetMediaMetadata()
		updated := match.Override(mediaMeta) // keep only the fields that can't be parsed from MediaInfo
		if updated {
			fmt.Println("After Applying those updates the name looks like this:")
			fmt.Printf("Generated Name:\t%s\n", match)
		} else {
			fmt.Println("The Parsed name fits the specification")
		}

	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
}
