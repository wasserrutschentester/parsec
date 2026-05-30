package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"codeberg.org/n0ne/parsec/internal/ui"
	"codeberg.org/n0ne/parsec/internal/update"
	"github.com/spf13/cobra"
)

var forceUpdate bool

func init() {
	updateCmd.Flags().BoolVarP(&forceUpdate, "force", "f", false, "force update even if version is the same or lower")
	rootCmd.AddCommand(updateCmd)
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update parsec to the latest version",
	Run: func(cmd *cobra.Command, args []string) {
		runUpdate()
	},
}

func runUpdate() {
	ui.PrintInfo("Checking for updates...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	rel, err := update.FetchLatestRelease(ctx)
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to check for updates: %v", err))
		return
	}

	ui.PrintDebug(fmt.Sprintf("Latest version: %s (Current: %s)", rel.TagName, Version))

	if !forceUpdate && !update.IsNewer(rel.TagName, Version) {
		ui.PrintSuccess(fmt.Sprintf("You are already on the latest version (%s)", Version))
		return
	}

	ui.PrintInfo(fmt.Sprintf("Updating to %s...", rel.TagName))

	asset := rel.GetMatchingAsset()
	if asset == nil {
		ui.PrintError("No prebuilt binary found for your platform")
		ui.PrintInfo("Available assets:")
		for _, a := range rel.Assets {
			ui.PrintInfo("- " + a.Name)
		}
		return
	}

	// 1. Download
	ui.PrintInfo("Downloading update...")
	tempFile, err := update.DownloadAsset(ctx, asset.BrowserDownloadURL)
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to download update: %v", err))
		return
	}
	defer func() { _ = os.Remove(tempFile) }() // Cleanup if we return early (e.g. checksum fail)

	// 2. Verify Checksum
	checksumAsset := rel.GetChecksumsAsset()
	if checksumAsset != nil {
		ui.PrintInfo("Verifying checksum...")
		if err := update.VerifyChecksum(ctx, asset.Name, tempFile, checksumAsset.BrowserDownloadURL); err != nil {
			ui.PrintError(fmt.Sprintf("Security check failed: %v", err))
			return
		}
		ui.PrintSuccess("Checksum verified")
	} else {
		ui.PrintWarning("No checksums.txt found in release, skipping verification")
	}

	// 3. Finalize Replacement
	ui.PrintInfo("Finalizing update...")
	if err := update.ReplaceExecutable(tempFile); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to replace binary: %v", err))
		return
	}

	ui.PrintSuccess(fmt.Sprintf("Successfully updated to %s", rel.TagName))
}
