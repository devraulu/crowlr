package main

import (
	"os"

	"github.com/spf13/cobra"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:          "crowlr",
	Short:        "A fast, polite web crawler",
	SilenceUsage: true,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "config.toml", "config file path")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
