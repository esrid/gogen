package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"os"

	"github.com/esrid/gogen/internal/scaffold"
)

var destroyControllerCmd = &cobra.Command{
	Use:     "controller <Name>",
	Aliases: []string{"c", "ctrl"},
	Short:   "Remove a generated controller",
	Example: "  gogen d controller Contact",
	Args:    cobra.ExactArgs(1),
	RunE:    runDestroyController,
}

func init() {
	destroyCmd.AddCommand(destroyControllerCmd)
}

func runDestroyController(_ *cobra.Command, args []string) error {
	name := scaffold.ToCamel(args[0])

	gogenCfg, err := loadGogenYAML()
	if err != nil {
		return err
	}

	if gogenCfg.Controllers == nil || gogenCfg.Controllers[name] == nil {
		return fmt.Errorf("controller %q not found in .gogen.yaml", name)
	}

	n := strings.ToLower(name[:1]) + name[1:]

	files := []string{
		"internal/adapters/web/" + n + "_handler.go",
		"internal/adapters/api/" + n + "_handler.go",
	}

	fmt.Printf("\nRemoving %s controller...\n\n", name)

	for _, path := range files {
		if err := removeFile(path); err != nil {
			return err
		}
	}

	if err := removeDir("web/components/" + n); err != nil {
		return err
	}

	if !flagDryRun {
		delete(gogenCfg.Controllers, name)
		if len(gogenCfg.Controllers) == 0 {
			gogenCfg.Controllers = nil
		}
		yamlData, err := yaml.Marshal(gogenCfg)
		if err != nil {
			return err
		}
		if err := os.WriteFile(".gogen.yaml", yamlData, 0644); err != nil {
			return err
		}
		if err := regenerateWireGen(gogenCfg); err != nil {
			fmt.Printf("  hint    update wire_gen.go manually (%v)\n", err)
		}
	}

	fmt.Println("\nDone.")
	return nil
}
