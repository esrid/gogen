package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/esrid/gogen/internal/config"
	"github.com/esrid/gogen/internal/scaffold"
)

var generateAPICmd = &cobra.Command{
	Use:     "api <ModelName>",
	Short:   "Enable JSON API endpoint for an existing SSR scaffold",
	Example: "  gogen g api Post",
	Args:    cobra.ExactArgs(1),
	RunE:    runGenerateAPI,
}

func init() {
	generateCmd.AddCommand(generateAPICmd)
}

func runGenerateAPI(_ *cobra.Command, args []string) error {
	modelName := scaffold.ToCamel(args[0])

	gogenCfg, err := loadGogenYAML()
	if err != nil {
		return err
	}

	if gogenCfg.RenderMode != "ssr" {
		return fmt.Errorf("gogen g api only applies to SSR projects (current mode: %s)", gogenCfg.RenderMode)
	}

	meta, ok := gogenCfg.Scaffolds[modelName]
	if !ok {
		return fmt.Errorf("%s not found in .gogen.yaml — run gogen g scaffold first", modelName)
	}

	if meta.API && !flagForce {
		return fmt.Errorf("%s already has an API handler (use --force to regenerate)", modelName)
	}

	fields, err := scaffold.ParseFields(meta.Fields)
	if err != nil {
		return fmt.Errorf("parse fields: %w", err)
	}

	cfg := &config.ProjectConfig{
		ModulePath: gogenCfg.Module,
		DB:         gogenCfg.DB,
		RenderMode: "both", // IsBoth() = true → handler named *APIHandler, routes /api/*
		Auth:       gogenCfg.Auth,
	}

	data := scaffold.NewData(modelName, fields, cfg)
	data.Protected = meta.Protected

	n := strings.ToLower(modelName)
	outPath := "internal/adapters/api/" + n + "_api_handler.go"

	fmt.Printf("\nEnabling API for %s...\n\n", modelName)

	if err := writeScaffoldFile("scaffold/handler.go.tmpl", outPath, data); err != nil {
		return err
	}

	if !flagDryRun {
		meta.API = true
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
