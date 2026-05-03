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
		fmt.Printf("  dryrun  internal/adapters/db/migrations/NNNNN_add_cols_to_%s.sql\n", data.TableName)
	}

	// Regenerate domain, port, service, store, handler(s) (force overwrite)
	n := strings.ToLower(modelName)
	regenSpecs := []scaffoldSpec{
		{"scaffold/domain.go.tmpl", "internal/domain/" + n + ".go"},
		{"scaffold/port.go.tmpl", "internal/domain/" + n + "_port.go"},
		{"scaffold/service.go.tmpl", "internal/application/" + n + "_service.go"},
		{attributeStoreTemplate(gogenCfg.DB), "internal/adapters/db/" + n + "_store.go"},
	}

	switch {
	case cfg.IsBoth():
		regenSpecs = append(regenSpecs,
			scaffoldSpec{"scaffold/handler_ssr.go.tmpl", "internal/adapters/web/" + n + "_handler.go"},
			scaffoldSpec{"scaffold/handler.go.tmpl", "internal/adapters/api/" + n + "_api_handler.go"},
		)
	case cfg.IsSSR():
		regenSpecs = append(regenSpecs, scaffoldSpec{"scaffold/handler_ssr.go.tmpl", "internal/adapters/web/" + n + "_handler.go"})
	default:
		regenSpecs = append(regenSpecs, scaffoldSpec{"scaffold/handler.go.tmpl", "internal/adapters/api/" + n + "_handler.go"})
	}

	if cfg.IsSSR() {
		t := data.TableName
		regenSpecs = append(regenSpecs,
			scaffoldSpec{"scaffold/pages/index.html.tmpl", "web/templates/pages/" + t + "_index.html"},
			scaffoldSpec{"scaffold/pages/show.html.tmpl", "web/templates/pages/" + t + "_show.html"},
			scaffoldSpec{"scaffold/pages/new.html.tmpl", "web/templates/pages/" + t + "_new.html"},
			scaffoldSpec{"scaffold/pages/edit.html.tmpl", "web/templates/pages/" + t + "_edit.html"},
		)
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
	migrationsDir := filepath.Join("internal", "adapters", "db", "migrations")
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
