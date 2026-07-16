//go:build no_update

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(updateCmd)
}

var updateCmd = &cobra.Command{
	Use:    "update",
	Short:  "Update parsec to the latest version (disabled)",
	Hidden: true,
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Println("Self-updating is disabled in this build of parsec.")
		fmt.Println("Please use your system package manager or pull the latest container image to update.")
	},
}
