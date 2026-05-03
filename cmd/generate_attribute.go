package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/esrid/gogen/internal/config"
	"github.com/esrid/gogen/internal/render"
	"github.com/esrid/gogen/internal/scaffold"
)

var attributeCmd = &cobra.Command{
	Use:     "attribute <ModelName> [field:type ...]",
	Aliases: []string{"a", "attr"},
	Short:   "Add fields to an existing scaffold",
	Example: "  gogen g attribute Post published:bool views:int",
	Args:    cobra.MinimumNArgs(2),
	RunE:    runGenerateAttribute,
}

func init() {
	generateCmd.AddCommand(attributeCmd)
}

func runGenerateAttribute(_ *cobra.Command, args []string) error {
	modelName := scaffold.ToCamel(args[0])
	newFieldArgs := args[1:]

	gogenCfg, err := loadGogenYAML()
	if err != nil {
		return err
	}

	meta, ok := gogenCfg.Scaffolds[modelName]
	if !ok {
		return fmt.Errorf("%s not found in .gogen.yaml — run gogen g scaffold first", modelName)
	}

	newFields, err := scaffold.ParseFields(newFieldArgs)
	if err != nil {
		return err
	}

	// Merge existing + new fields, avoiding duplicates
	existingFields, err := scaffold.ParseFields(meta.Fields)
	if err != nil {
		return fmt.Errorf("parse existing fields: %w", err)
	}
	existingNames := make(map[string]bool, len(existingFields))
	for _, f := range existingFields {
		existingNames[f.Name] = true
	}
	for _, f := range newFields {
		if existingNames[f.Name] {
			return fmt.Errorf("field %q already exists on %s", f.Name, modelName)
		}
	}
	allFields := append(existingFields, newFields...)

	cfg := &config.ProjectConfig{
		ModulePath: gogenCfg.Module,
		DB:         gogenCfg.DB,
		RenderMode: gogenCfg.RenderMode,
		Auth:       gogenCfg.Auth,
	}

	data := scaffold.NewData(modelName, allFields, cfg)
	data.Protected = meta.Protected

	fmt.Printf("\nAdding attributes to %s...\n\n", modelName)

	// Create ALTER TABLE migration
	if !flagDryRun {
		if err := createAttributeMigration(data, newFields, gogenCfg.DB); err != nil {
			return err
		}
	} else {
		fmt.Printf("  dryrun  internal/adapters/store/migrations/NNNNN_add_cols_to_%s.sql\n", data.TableName)
	}

	// Regenerate domain, store, handler (force overwrite)
	n := strings.ToLower(modelName)
	regenSpecs := []scaffoldSpec{
		{"scaffold/domain.go.tmpl", "internal/core/domains/" + n + ".go"},
		{attributeStoreTemplate(gogenCfg.DB), "internal/adapters/store/" + n + "_store.go"},
		{attributeHandlerTemplate(cfg), "internal/adapters/http/" + n + "_handler.go"},
	}

	for _, spec := range regenSpecs {
		if err := regenFile(spec.templatePath, spec.outputPath, data); err != nil {
			return err
		}
	}

	if !flagDryRun {
		// Update .gogen.yaml
		meta.Fields = append(meta.Fields, newFieldArgs...)
		yamlData, err := yaml.Marshal(gogenCfg)
		if err != nil {
			return err
		}
		if err := os.WriteFile(".gogen.yaml", yamlData, 0644); err != nil {
			return err
		}

		if cfg.IsSSR() {
			fmt.Println("\n  note    SSR views not regenerated — update web/templates/pages/" + data.TableName + "_*.html manually")
		}
	}

	fmt.Println("\nDone.")
	return nil
}

func attributeStoreTemplate(db string) string {
	if db == "postgres" {
		return "scaffold/store_postgres.go.tmpl"
	}
	return "scaffold/store_sqlite.go.tmpl"
}

func attributeHandlerTemplate(cfg *config.ProjectConfig) string {
	if cfg.IsSSR() {
		return "scaffold/handler_ssr.go.tmpl"
	}
	return "scaffold/handler.go.tmpl"
}

func regenFile(tmplPath, outPath string, data *scaffold.Data) error {
	content, err := render.File(tmplPath, data)
	if err != nil {
		return fmt.Errorf("render %s: %w", tmplPath, err)
	}
	if flagDryRun {
		fmt.Printf("  dryrun  %s\n", outPath)
		return nil
	}
	if err := writeFile(outPath, content); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	fmt.Printf("  update  %s\n", outPath)
	return nil
}

func createAttributeMigration(data *scaffold.Data, newFields []scaffold.Field, db string) error {
	migrationsDir := filepath.Join("internal", "adapters", "store", "migrations")
	next, err := nextMigrationNumber(migrationsDir)
	if err != nil {
		return err
	}

	var upLines, downLines []string
	for _, f := range newFields {
		colDef := f.SQLiteCol
		if db == "postgres" {
			colDef = f.PGCol
		}
		upLines = append(upLines, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s;", data.TableName, f.Name, colDef))
		downLines = append(downLines, fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", data.TableName, f.Name))
		if f.IsRef {
			upLines = append(upLines, fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_%s ON %s (%s);", data.TableName, f.Name, data.TableName, f.Name))
		}
	}

	var sql string
	if db == "sqlite" {
		sql = fmt.Sprintf("-- +goose Up\n-- +goose StatementBegin\n%s\n-- +goose StatementEnd\n\n-- +goose Down\n-- +goose StatementBegin\n%s\n-- +goose StatementEnd\n",
			strings.Join(upLines, "\n"),
			strings.Join(downLines, "\n"),
		)
	} else {
		sql = fmt.Sprintf("-- +goose Up\n%s\n\n-- +goose Down\n%s\n",
			strings.Join(upLines, "\n"),
			strings.Join(downLines, "\n"),
		)
	}

	// Derive migration name from new field names
	fieldNames := make([]string, len(newFields))
	for i, f := range newFields {
		fieldNames[i] = f.Name
	}
	migName := "add_" + strings.Join(fieldNames, "_") + "_to_" + data.TableName

	filename := filepath.Join(migrationsDir, fmt.Sprintf("%05d_%s.sql", next, migName))
	if err := os.WriteFile(filename, []byte(sql), 0644); err != nil {
		return err
	}
	fmt.Printf("  create  %s\n", filename)
	return nil
}
