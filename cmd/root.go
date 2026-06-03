package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/ui"
	"codeberg.org/upPollo/parsec/internal/update"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	Version = "v0.0.0"
)

var rootCmd = &cobra.Command{
	Use:   "parsec",
	Short: "parsec allows you to parse, check and create releases",
	Long:  ui.Banner(".: FIRST STEPS? :."),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		ui.IsSilent = jsonOutputFlag // make sure only json is printed
		if jsonOutputFlag {
			ui.DisableColors()
		}
		ui.IsDebug = debugFlag
		ui.PrintDebug("Debug output enabled")
		config.NoCache = noCacheFlag
		update.CheckForUpdateBackground(Version)
	},
}

func Execute() {
	// Version is now injected via ldflags during build
	rootCmd.Version = Version

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/parsec/config.toml)")
	rootCmd.PersistentFlags().StringVarP(&presetFlag, "preset", "p", "", "configuration preset to use")
	rootCmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "enable debug output")
	rootCmd.PersistentFlags().BoolVar(&noCacheFlag, "no-cache", false, "bypass the API cache and fetch fresh data")

	config.InitDefaults()

	// Add template functions for flag grouping
	cobra.AddTemplateFunc("filterFlags", func(fs *pflag.FlagSet, key, value string) *pflag.FlagSet {
		newFs := pflag.NewFlagSet(value, pflag.ContinueOnError)
		newFs.SortFlags = false
		fs.VisitAll(func(f *pflag.Flag) {
			if v, ok := f.Annotations[key]; ok {
				for _, s := range v {
					if s == value {
						newFs.AddFlag(f)
						break
					}
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
		fs.VisitAll(func(f *pflag.Flag) {
			has = true
		})
		return has
	})

	// Apply global usage template to support flag grouping
	rootCmd.SetUsageTemplate(`Usage:{{if .Runnable}}
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
`)
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
		if err := viper.ReadInConfig(); err != nil {
			ui.PrintError(fmt.Sprintf("Error reading config file: %v", err))
		}
	} else {
		// Set search paths
		confDir, err := os.UserConfigDir()
		if err == nil {
			viper.AddConfigPath(filepath.Join(confDir, "parsec"))
		}
		viper.AddConfigPath(".")

		// Set preferred type to TOML
		viper.SetConfigType("toml")

		// Try 'config' first
		viper.SetConfigName("config")
		if err := viper.ReadInConfig(); err != nil {
			// Fallback to '.parsec' (can still be TOML if extension matches or forced)
			viper.SetConfigName(".parsec")
			_ = viper.ReadInConfig()
		}
	}

	viper.AutomaticEnv()

	if presetFlag != "" {
		config.SetPreset(presetFlag)
		if !config.PresetExists(presetFlag) {
			ui.PrintWarning(fmt.Sprintf("Preset '%s' does not exist in your configuration", presetFlag))
		}
	}
}
