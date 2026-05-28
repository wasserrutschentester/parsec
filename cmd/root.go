package cmd

import (
	"os"
	"path/filepath"

	"codeberg.org/n0ne/parsec/internal/config"
	"codeberg.org/n0ne/parsec/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "parsec",
	Short: "parsec allows you to parse, check and create releases",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		ui.IsDebug = debugFlag
		ui.PrintDebug("Debug output enabled")
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/parsec/config.toml)")
	rootCmd.PersistentFlags().StringVar(&presetFlag, "preset", "", "configuration preset to use")
	rootCmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "enable debug output")

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
	}
}
