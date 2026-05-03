package cmd

import "github.com/spf13/cobra"

var generateCmd = &cobra.Command{
	Use:     "generate",
	Aliases: []string{"g"},
	Short:   "Generate code for an existing project",
}

func init() {
	rootCmd.AddCommand(generateCmd)
}
