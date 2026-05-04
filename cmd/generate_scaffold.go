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

var scaffoldCmd = &cobra.Command{
	Use:     "scaffold <ModelName> [field:type ...]",
	Aliases: []string{"s"},
	Short:   "Generate CRUD scaffold for a model",
	Example: "  gogen g scaffold Post title:string body:text user:references published:bool",
	Args:    cobra.MinimumNArgs(1),
	RunE:    runScaffold,
}

func init() {
	generateCmd.AddCommand(scaffoldCmd)
	scaffoldCmd.Flags().Bool("protected", false, "Mount routes inside the auth-protected group")
}

func runScaffold(cmd *cobra.Command, args []string) error {
	modelName := scaffold.ToCamel(args[0])
	fieldArgs := args[1:]

	protected, _ := cmd.Flags().GetBool("protected")

	gogenCfg, err := loadGogenYAML()
	if err != nil {
		return err
	}

	if protected && !gogenCfg.Auth {
		return fmt.Errorf("--protected requires auth (run: gogen g auth)")
	}

	fields, err := scaffold.ParseFields(fieldArgs)
	if err != nil {
		return err
	}

	cfg := &config.ProjectConfig{
		ModulePath: gogenCfg.Module,
		DB:         gogenCfg.DB,
		RenderMode: gogenCfg.RenderMode,
		Auth:       gogenCfg.Auth,
	}

	data := scaffold.NewData(modelName, fields, cfg)
	data.Protected = protected

	fmt.Printf("\nGenerating %s scaffold...\n\n", modelName)

	specs := scaffoldSpecs(data)
	for _, spec := range specs {
		if err := writeScaffoldFile(spec.templatePath, spec.outputPath, data); err != nil {
			return err
		}
	}

	// Migration
	if !flagDryRun {
		if err := createScaffoldMigration(data); err != nil {
			return err
		}
	} else {
		fmt.Printf("  dryrun  internal/adapters/db/migrations/NNNNN_create_%s.sql\n", data.TableName)
	}

	if !flagDryRun {
		if err := saveScaffoldMeta(gogenCfg, modelName, fieldArgs, protected); err != nil {
			fmt.Printf("  warn    could not save scaffold metadata: %v\n", err)
		}
		autoWireScaffold(data, gogenCfg)
	}

	fmt.Println("\nDone.")
	return nil
}

func saveScaffoldMeta(cfg *config.GogenYAML, modelName string, fieldArgs []string, protected bool) error {
	if cfg.Scaffolds == nil {
		cfg.Scaffolds = make(map[string]*config.ScaffoldMeta)
	}
	// Preserve existing flags (e.g. API: true set by `gogen g api`) when re-scaffolding with --force.
	existing := cfg.Scaffolds[modelName]
	meta := &config.ScaffoldMeta{
		Fields:    fieldArgs,
		Protected: protected,
	}
	if existing != nil {
		meta.API = existing.API
	}
	cfg.Scaffolds[modelName] = meta
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(".gogen.yaml", data, 0644)
}

type scaffoldSpec struct {
	templatePath string
	outputPath   string
}

func scaffoldSpecs(data *scaffold.Data) []scaffoldSpec {
	n := strings.ToLower(data.ModelName)

	storeTemplate := "scaffold/store_sqlite.go.tmpl"
	if data.DB == "postgres" {
		storeTemplate = "scaffold/store_postgres.go.tmpl"
	}

	specs := []scaffoldSpec{
		{"scaffold/domain.go.tmpl", "internal/domain/" + n + ".go"},
		{"scaffold/port.go.tmpl", "internal/domain/" + n + "_port.go"},
		{storeTemplate, "internal/adapters/db/" + n + "_store.go"},
		{"scaffold/service.go.tmpl", "internal/application/" + n + "_service.go"},
	}

	switch {
	case data.IsBoth():
		specs = append(specs,
			scaffoldSpec{"scaffold/handler_ssr.go.tmpl", "internal/adapters/web/" + n + "_handler.go"},
			scaffoldSpec{"scaffold/handler.go.tmpl", "internal/adapters/api/" + n + "_api_handler.go"},
		)
	case data.IsSSR():
		specs = append(specs, scaffoldSpec{"scaffold/handler_ssr.go.tmpl", "internal/adapters/web/" + n + "_handler.go"})
	default:
		specs = append(specs, scaffoldSpec{"scaffold/handler.go.tmpl", "internal/adapters/api/" + n + "_handler.go"})
	}

	if data.IsSSR() {
		specs = append(specs,
			scaffoldSpec{"scaffold/components/index.templ.tmpl", "web/components/" + n + "/index.templ"},
			scaffoldSpec{"scaffold/components/show.templ.tmpl", "web/components/" + n + "/show.templ"},
			scaffoldSpec{"scaffold/components/new.templ.tmpl", "web/components/" + n + "/new.templ"},
			scaffoldSpec{"scaffold/components/edit.templ.tmpl", "web/components/" + n + "/edit.templ"},
		)
	}

	return specs
}

func writeScaffoldFile(tmplPath, outPath string, data *scaffold.Data) error {
	exists := false
	if _, err := os.Stat(outPath); err == nil {
		exists = true
	}

	if exists && !flagForce {
		if flagSkip || flagDryRun {
			fmt.Printf("  skip    %s\n", outPath)
		} else {
			fmt.Printf("  conflict %s\n", outPath)
		}
		return nil
	}

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
	fmt.Printf("  create  %s\n", outPath)
	return nil
}

func createScaffoldMigration(data *scaffold.Data) error {
	migrationsDir := filepath.Join("internal", "adapters", "db", "migrations")
	next, err := nextMigrationNumber(migrationsDir)
	if err != nil {
		return err
	}

	tmplPath := "scaffold/migration_sqlite.sql.tmpl"
	if data.DB == "postgres" {
		tmplPath = "scaffold/migration_postgres.sql.tmpl"
	}

	content, err := render.File(tmplPath, data)
	if err != nil {
		return fmt.Errorf("render migration: %w", err)
	}

	filename := filepath.Join(migrationsDir, fmt.Sprintf("%05d_create_%s.sql", next, data.TableName))
	if err := os.WriteFile(filename, content, 0644); err != nil {
		return err
	}
	fmt.Printf("  create  %s\n", filename)
	return nil
}

