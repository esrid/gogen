package cmd

import "github.com/spf13/cobra"

var destroyCmd = &cobra.Command{
	Use:     "destroy",
	Aliases: []string{"d"},
	Short:   "Remove previously generated code",
}

func init() {
	rootCmd.AddCommand(destroyCmd)
}
