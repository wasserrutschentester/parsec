package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"codeberg.org/n0ne/parsec/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "parsec",
	Short: "parsec allows you to parse, check and create releases",
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $XDG_CONFIG_HOME/parsec/config.toml)")
	
	config.InitDefaults()
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

	if viper.ConfigFileUsed() != "" {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
