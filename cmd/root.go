package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/ui"
	"codeberg.org/upPollo/parsec/internal/update"
)

var (
	cfgFile string
	// Version is the current version of parsec, set during build via ldflags.
	Version = "v0.0.0"
)

var rootCmd = &cobra.Command{
	Use:   "parsec",
	Short: "parsec allows you to parse, check and create releases",
	Long:  ui.Banner(".: FIRST STEPS? :."),
	PersistentPreRun: func(cmd *cobra.Command, _ []string) {
		ui.IsSilent = jsonOutputFlag // make sure only json is printed
		ui.IsJSON = jsonOutputFlag

		if ui.IsSilent {
			ui.DisableColors()
		}

		ui.IsDebug = debugFlag

		ui.PrintDebug("Debug output enabled")

		config.NoCache = noCacheFlag

		initConfig()
		config.InitDefaults()

		// Suppress update check for certain commands
		if !isExcludedFromUpdateCheck(cmd) {
			update.CheckForUpdateBackground(Version)
		}
	},
}

func isExcludedFromUpdateCheck(cmd *cobra.Command) bool {
	switch cmd.Name() {
	case "__complete", "__completeNoDesc", "completion", "update":
		return true
	default:
		// checks for "parsec completion bash" and similar commands
		return cmd.HasParent() && cmd.Parent().Name() == "completion"
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// Version is now injected via ldflags during build
	rootCmd.Version = Version

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/parsec/config.toml)")
	rootCmd.PersistentFlags().StringVarP(&presetFlag, "preset", "p", "", "configuration preset to use")
	rootCmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "enable debug output")
	rootCmd.PersistentFlags().BoolVar(&noCacheFlag, "no-cache", false, "bypass the API cache and fetch fresh data")

	// Provide dynamic shell completions for --preset / -p.
	// initConfig() is called explicitly here because completion runs before
	// PersistentPreRun, so viper has not yet loaded the configuration file.
	_ = rootCmd.RegisterFlagCompletionFunc("preset", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		initConfig()

		return config.ListPresets(), cobra.ShellCompDirectiveNoFileComp
	})

	// Add template functions for flag grouping
	cobra.AddTemplateFunc("filterFlags", func(fs *pflag.FlagSet, key, value string) *pflag.FlagSet {
		newFs := pflag.NewFlagSet(value, pflag.ContinueOnError)
		newFs.SortFlags = false

		fs.VisitAll(func(f *pflag.Flag) {
			if v, ok := f.Annotations[key]; ok {
				if slices.Contains(v, value) {
					newFs.AddFlag(f)
				}
			}
		})

		return newFs
	})
	cobra.AddTemplateFunc("ungroupedFlags", func(fs *pflag.FlagSet) *pflag.FlagSet {
		newFs := pflag.NewFlagSet("other", pflag.ContinueOnError)
		newFs.SortFlags = false

		fs.VisitAll(func(f *pflag.Flag) {
			if _, ok := f.Annotations["group"]; !ok {
				newFs.AddFlag(f)
			}
		})

		return newFs
	})
	cobra.AddTemplateFunc("hasFlags", func(fs *pflag.FlagSet) bool {
		has := false

		fs.VisitAll(func(_ *pflag.Flag) {
			has = true
		})

		return has
	})

	// Apply global usage template to support flag grouping
	rootCmd.SetUsageTemplate(usageTemplate)
}

const usageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}
{{$metadataFlags := (filterFlags .LocalFlags "group" "metadata") -}}
{{$p2pInfoFlags := (filterFlags .LocalFlags "group" "p2p") -}}
{{$idFlags := (filterFlags .LocalFlags "group" "id") -}}
{{$otherFlags := (ungroupedFlags .LocalFlags) -}}
{{if or (hasFlags $metadataFlags) (hasFlags $idFlags) }}
{{- if (hasFlags $metadataFlags)}}

Metadata Flags:
{{$metadataFlags.FlagUsages | trimTrailingWhitespaces}}
{{- end}}
{{- if (hasFlags $p2pInfoFlags)}}

P2P Info Flags:
{{$p2pInfoFlags.FlagUsages | trimTrailingWhitespaces}}
{{- end}}

{{- if (hasFlags $idFlags)}}

ID Flags:
{{$idFlags.FlagUsages | trimTrailingWhitespaces}}
{{- end}}
{{- if (hasFlags $otherFlags)}}

Other Flags:
{{$otherFlags.FlagUsages | trimTrailingWhitespaces}}
{{- end}}
{{- else}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}
{{- end}}
{{- end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

// tryLoadConfig attempts to load the configuration using the provided names.
// It returns true if a config was successfully loaded or if a parsing error occurred
// (in which case it prints the error and stops further searching).
func tryLoadConfig(names ...string) bool {
	for _, name := range names {
		viper.SetConfigName(name)

		if err := viper.ReadInConfig(); err != nil {
			var configFileNotFoundError viper.ConfigFileNotFoundError
			if !errors.As(err, &configFileNotFoundError) {
				configPath := viper.ConfigFileUsed()
				if configPath == "" {
					configPath = name
				}

				ui.PrintError(fmt.Sprintf("Error reading config file %s: %v", ui.AnonymizePath(configPath), err))

				return true // Stop trying if we found a file but it's broken
			}
			// If it's just not found, continue to the next name
		} else {
			return true // Successfully read a config, stop trying
		}
	}

	return false
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)

		if err := viper.ReadInConfig(); err != nil {
			ui.PrintError(fmt.Sprintf("Error reading config file %s: %v", ui.AnonymizePath(cfgFile), err))
		}
	} else {
		// Set search paths (only system dir initially)
		confDir, err := os.UserConfigDir()
		if err == nil {
			viper.AddConfigPath(filepath.Join(confDir, "parsec"))
		}

		// Set preferred type to TOML
		viper.SetConfigType("toml")

		// 1. Try 'config' (only in system config dir)
		if !tryLoadConfig("config") {
			// 2. If not found, allow searching in the current directory for parsec specific names
			viper.AddConfigPath(".")
			tryLoadConfig("parsec.toml", ".parsec")
		}
	}

	viper.AutomaticEnv()

	if viper.ConfigFileUsed() != "" {
		ui.PrintDebug("Using config file: " + ui.AnonymizePath(viper.ConfigFileUsed()))
	} else if cfgFile == "" {
		ui.PrintWarning("No config file found, using defaults")
	}

	if presetFlag != "" {
		config.SetPreset(presetFlag)

		if !config.PresetExists(presetFlag) {
			ui.PrintWarning(fmt.Sprintf("Preset '%s' does not exist in your configuration", presetFlag))
		} else {
			ui.PrintDebug("Using configuration preset: " + presetFlag)
		}
	}
}
