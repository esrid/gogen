package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	flagForce  bool
	flagDryRun bool
	flagSkip   bool
)

var rootCmd = &cobra.Command{
	Use:   "gogen",
	Short: "Rails-style Go project generator",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&flagForce, "force", "f", false, "overwrite existing files")
	rootCmd.PersistentFlags().BoolVarP(&flagDryRun, "dry-run", "p", false, "preview without writing files")
	rootCmd.PersistentFlags().BoolVarP(&flagSkip, "skip", "s", false, "skip existing files")
}
