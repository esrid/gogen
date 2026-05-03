package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/esrid/gogen/internal/render"
	"github.com/esrid/gogen/internal/scaffold"
	"github.com/esrid/gogen/internal/config"
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
		fmt.Printf("  dryrun  internal/adapters/store/migrations/NNNNN_create_%s.sql\n", data.TableName)
	}

	printScaffoldHint(data)
	return nil
}

type scaffoldSpec struct {
	templatePath string
	outputPath   string
}

func scaffoldSpecs(data *scaffold.Data) []scaffoldSpec {
	n := strings.ToLower(data.ModelName)
	db := data.DB

	storeTemplate := "scaffold/store_sqlite.go.tmpl"
	if db == "postgres" {
		storeTemplate = "scaffold/store_postgres.go.tmpl"
	}

	return []scaffoldSpec{
		{"scaffold/domain.go.tmpl", "internal/core/domains/" + n + ".go"},
		{"scaffold/port.go.tmpl", "internal/core/ports/" + n + "_port.go"},
		{storeTemplate, "internal/adapters/store/" + n + "_store.go"},
		{"scaffold/service.go.tmpl", "internal/core/services/" + n + "_service.go"},
		{"scaffold/handler.go.tmpl", "internal/adapters/http/" + n + "_handler.go"},
	}
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
	migrationsDir := filepath.Join("internal", "adapters", "store", "migrations")
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

func printScaffoldHint(data *scaffold.Data) {
	n := strings.ToLower(data.ModelName)

	var mountHint string
	if data.Protected {
		mountHint = fmt.Sprintf(`       // inside the protected r.Group (with RequireAuth middleware)
       if h.%s != nil {
           r.Mount("%s", h.%s.Route())
       }`, data.ModelName, data.RoutePrefix, data.ModelName)
	} else {
		mountHint = fmt.Sprintf(`       if h.%s != nil {
           r.Mount("%s", h.%s.Route())
       }`, data.ModelName, data.RoutePrefix, data.ModelName)
	}

	fmt.Printf(`
Done! Wire it up in your project:

1. internal/server/routes.go — add to Handler struct:
       %s *api.%sHandler

2. internal/server/routes.go — add to NewRouter:
%s

3. main.go — add after store init:
       %sService := services.New%sService(dbStore)
       handlers.%s = api.New%sHandler(%sService)
`,
		n, data.ModelName,
		mountHint,
		n, data.ModelName, data.ModelName, data.ModelName, n,
	)
}
