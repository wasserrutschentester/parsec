package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/ui"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage parsec configuration",
	Long:  ui.Banner(".: CONFIGURATION :."),
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a default configuration file",
	Long:  ui.Banner(".: WARP CORE LOADING PROTO :."),
	RunE: func(_ *cobra.Command, _ []string) error {
		ui.Println(ui.Banner(".: LOADING WARP CORE :."))

		confDir, err := os.UserConfigDir()
		if err != nil {
			ui.PrintError(fmt.Sprintf("Could not determine user config directory: %v", err))
			return fmt.Errorf("config dir determination failed")
		}

		targetDir := filepath.Join(confDir, "parsec")
		targetFile := filepath.Join(targetDir, "config.toml")

		if _, err := os.Stat(targetFile); err == nil {
			if !ui.ConfirmContinue(fmt.Sprintf("Configuration file already exists at %s. Overwrite and backup old one?", ui.AnonymizePath(targetFile))) {
				return nil
			}

			backupFile := targetFile + "." + time.Now().Format("2006-01-02_15-04-05") + ".bak"
			if err := os.Rename(targetFile, backupFile); err != nil {
				ui.PrintError(fmt.Sprintf("Could not backup existing config file: %v", err))
				return fmt.Errorf("config backup failed")
			}

			ui.PrintInfo(fmt.Sprintf("Existing configuration backed up to %s", ui.AnonymizePath(backupFile)))
		}

		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			ui.PrintError(fmt.Sprintf("Could not create config directory: %v", err))
			return fmt.Errorf("config dir creation failed")
		}

		if err := os.WriteFile(targetFile, []byte(config.GetDefaultConfig()), 0o644); err != nil {
			ui.PrintError(fmt.Sprintf("Could not write config file: %v", err))
			return fmt.Errorf("config write failed")
		}

		ui.PrintSuccess(fmt.Sprintf("Created default configuration at %s", ui.AnonymizePath(targetFile)))

		return nil
	},
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Verify the current configuration",
	Long:  ui.Banner(".: STABILITY ASSESSMENT :."),
	Run: func(_ *cobra.Command, _ []string) {
		ui.Println(ui.Banner(".: ASSESSING STABILITY :."))
		config.Validate()
		ui.PrintSuccess("Configuration validation complete.")
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configValidateCmd)
}
