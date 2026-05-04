package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/esrid/gogen/internal/config"
	"github.com/esrid/gogen/internal/scaffold"
)

var destroyScaffoldCmd = &cobra.Command{
	Use:     "scaffold <ModelName>",
	Aliases: []string{"s"},
	Short:   "Remove a generated scaffold",
	Example: "  gogen d scaffold Post",
	Args:    cobra.ExactArgs(1),
	RunE:    runDestroyScaffold,
}

func init() {
	destroyCmd.AddCommand(destroyScaffoldCmd)
}

func runDestroyScaffold(_ *cobra.Command, args []string) error {
	modelName := scaffold.ToCamel(args[0])

	gogenCfg, err := loadGogenYAML()
	if err != nil {
		return err
	}

	cfg := &config.ProjectConfig{
		DB:         gogenCfg.DB,
		ModulePath: gogenCfg.Module,
	}

	data := scaffold.NewData(modelName, nil, cfg)
	n := strings.ToLower(modelName)

	files := []string{
		"internal/domain/" + n + ".go",
		"internal/domain/" + n + "_port.go",
		"internal/adapters/db/" + n + "_store.go",
		"internal/application/" + n + "_service.go",
		"internal/adapters/api/" + n + "_handler.go",
		"internal/adapters/api/" + n + "_api_handler.go",
		"internal/adapters/web/" + n + "_handler.go",
	}

	fmt.Printf("\nRemoving %s scaffold...\n\n", modelName)

	for _, path := range files {
		if err := removeFile(path); err != nil {
			return err
		}
	}

	if err := removeDir("web/components/" + n); err != nil {
		return err
	}

	// Find and remove matching migration(s)
	migrations, err := findMigrations("internal/adapters/db/migrations", "create_"+data.TableName)
	if err != nil {
		return err
	}

	for _, m := range migrations {
		fmt.Printf("  warning migration %s was deleted — run goose down manually if already applied\n", filepath.Base(m))
		if err := removeFile(m); err != nil {
			return err
		}
	}

	removeScaffoldMeta(gogenCfg, modelName)
	removeWireScaffold(data, gogenCfg)

	fmt.Println("\nDone.")
	return nil
}

func removeScaffoldMeta(cfg *config.GogenYAML, modelName string) {
	if cfg.Scaffolds == nil {
		return
	}
	delete(cfg.Scaffolds, modelName)
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return
	}
	_ = os.WriteFile(".gogen.yaml", data, 0644)
}

func removeDir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Printf("  skip    %s (not found)\n", path)
		return nil
	}
	if flagDryRun {
		fmt.Printf("  dryrun  %s\n", path)
		return nil
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	fmt.Printf("  remove  %s\n", path)
	return nil
}

func removeFile(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Printf("  skip    %s (not found)\n", path)
		return nil
	}
	if flagDryRun {
		fmt.Printf("  dryrun  %s\n", path)
		return nil
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	fmt.Printf("  remove  %s\n", path)
	return nil
}

func findMigrations(dir, suffix string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var matches []string
	for _, e := range entries {
		if !e.IsDir() && strings.Contains(e.Name(), suffix) && filepath.Ext(e.Name()) == ".sql" {
			matches = append(matches, filepath.Join(dir, e.Name()))
		}
	}
	return matches, nil
}
