package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"codeberg.org/upPollo/parsec/internal/ui"
	"codeberg.org/upPollo/parsec/internal/update"
)

var forceUpdate bool

func init() {
	updateCmd.Flags().BoolVarP(&forceUpdate, "force", "f", false, "force update even if version is the same or lower")
	rootCmd.AddCommand(updateCmd)
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update parsec to the latest version",
	RunE: func(_ *cobra.Command, _ []string) error {
		return runUpdate()
	},
}

func runUpdate() error {
	ui.PrintInfo("Checking for updates...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	rel, err := update.FetchLatestRelease(ctx)
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to check for updates: %v", err))
		return fmt.Errorf("fetching release info failed")
	}

	ui.PrintDebug(fmt.Sprintf("Latest version: %s (Current: %s)", rel.TagName, Version))

	if !forceUpdate && !update.IsNewer(rel.TagName, Version) {
		ui.PrintSuccess(fmt.Sprintf("You are already on the latest version (%s)", Version))
		return nil
	}

	ui.PrintInfo(fmt.Sprintf("Updating to %s...", rel.TagName))

	tempFile, err := downloadAndVerify(ctx, rel)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tempFile) }()

	return finalizeUpdate(tempFile, rel.TagName)
}

func downloadAndVerify(ctx context.Context, rel *update.Release) (string, error) {
	asset := rel.GetMatchingAsset()
	if asset == nil {
		ui.PrintError("No prebuilt binary found for your platform")
		ui.PrintInfo("Available assets:")

		for _, a := range rel.Assets {
			ui.PrintInfo("- " + a.Name)
		}

		return "", fmt.Errorf("no matching asset found")
	}

	// 1. Download
	ui.PrintInfo("Downloading update...")

	tempFile, err := update.DownloadAsset(ctx, asset.BrowserDownloadURL)
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to download update: %v", err))
		return "", fmt.Errorf("download failed")
	}

	// 2. Verify Checksum
	checksumAsset := rel.GetChecksumsAsset()
	if checksumAsset != nil {
		ui.PrintInfo("Verifying checksum...")

		if err := update.VerifyChecksum(ctx, asset.Name, tempFile, checksumAsset.BrowserDownloadURL); err != nil {
			ui.PrintError(fmt.Sprintf("Security check failed: %v", err))
			return tempFile, fmt.Errorf("checksum verification failed")
		}

		ui.PrintSuccess("Checksum verified")
	} else {
		ui.PrintWarning("No checksums.txt found in release, skipping verification")
	}

	return tempFile, nil
}

func finalizeUpdate(tempFile, tagName string) error {
	// 3. Finalize Replacement
	ui.PrintInfo("Finalizing update...")

	if err := update.ReplaceExecutable(tempFile); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to replace binary: %v", err))
		return fmt.Errorf("update failed")
	}

	ui.PrintSuccess(fmt.Sprintf("Successfully updated to %s", tagName))

	return nil
}
