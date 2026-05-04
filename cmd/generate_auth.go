package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/esrid/gogen/internal/config"
	"github.com/esrid/gogen/internal/generator"
	"github.com/esrid/gogen/internal/render"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Add authentication to an existing project",
	Args:  cobra.NoArgs,
	RunE:  runGenerateAuth,
}

func init() {
	generateCmd.AddCommand(authCmd)
}

func runGenerateAuth(_ *cobra.Command, _ []string) error {
	gogenCfg, err := loadGogenYAML()
	if err != nil {
		return err
	}
	if gogenCfg.Auth {
		return fmt.Errorf("auth is already enabled in this project")
	}

	cfg := &config.ProjectConfig{
		ModulePath: gogenCfg.Module,
		DB:         gogenCfg.DB,
		RenderMode: gogenCfg.RenderMode,
		Auth:       true,
	}

	// infra = always overwrite (wiring files that change when auth is added)
	infra, new := authFileSpecs(cfg)

	fmt.Println("\nAdding auth...")
	fmt.Println()

	for _, spec := range infra {
		if err := writeAuthFile(spec, cfg, true); err != nil {
			return err
		}
	}
	for _, spec := range new {
		if err := writeAuthFile(spec, cfg, false); err != nil {
			return err
		}
	}

	if !flagDryRun {
		if err := createAuthMigration(cfg.DB); err != nil {
			return err
		}
		if err := setAuthInGogenYAML(gogenCfg); err != nil {
			return fmt.Errorf("update .gogen.yaml: %w", err)
		}
		reWireAllScaffolds(gogenCfg)
		return generator.PostProcess(".", cfg.IsSSR())
	}

	fmt.Println("\nDry run complete. No files written.")
	return nil
}

func writeAuthFile(spec generator.FileSpec, cfg *config.ProjectConfig, force bool) error {
	outPath := spec.OutputPath

	exists := false
	if _, err := os.Stat(outPath); err == nil {
		exists = true
	}

	if exists && !force && !flagForce {
		if flagSkip || flagDryRun {
			fmt.Printf("  skip    %s\n", outPath)
		} else {
			fmt.Printf("  conflict %s\n", outPath)
		}
		return nil
	}

	content, err := render.File(spec.TemplatePath, cfg)
	if err != nil {
		return fmt.Errorf("render %s: %w", spec.TemplatePath, err)
	}

	if flagDryRun {
		fmt.Printf("  dryrun  %s\n", outPath)
		return nil
	}

	if err := writeFile(outPath, content); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	if exists {
		fmt.Printf("  update  %s\n", outPath)
	} else {
		fmt.Printf("  create  %s\n", outPath)
	}
	return nil
}

// authFileSpecs returns (infra specs to always overwrite, new specs to create only)
func authFileSpecs(cfg *config.ProjectConfig) (infra, new []generator.FileSpec) {
	infra = []generator.FileSpec{
		generator.S("new/base/main.go.tmpl", "main.go"),
		generator.S("new/base/bootstrap/router.go.tmpl", "bootstrap/router.go"),
		generator.S("new/base/internal/domain/errors.go.tmpl", "internal/domain/errors.go"),
	}

	if cfg.IsSSR() {
		infra = append(infra,
			generator.S("new/ssr/web/renderer.go.tmpl", "web/renderer.go"),
		)
	}

	new = []generator.FileSpec{
		generator.S("new/auth/internal/domain/user.go.tmpl", "internal/domain/user.go"),
		generator.S("new/auth/internal/domain/auth_port.go.tmpl", "internal/domain/auth_port.go"),
		generator.S("new/auth/internal/domain/email_port.go.tmpl", "internal/domain/email_port.go"),
		generator.S("new/auth/internal/application/auth_service.go.tmpl", "internal/application/auth_service.go"),
		generator.S("new/auth/internal/application/session_service.go.tmpl", "internal/application/session_service.go"),
		generator.S("new/auth/internal/utils/validation.go.tmpl", "internal/utils/validation.go"),
		generator.S("new/auth/internal/adapters/api/auth_handler.go.tmpl", "internal/adapters/api/auth_handler.go"),
		generator.S("new/auth/internal/adapters/api/middleware_auth.go.tmpl", "internal/adapters/api/middleware_auth.go"),
		generator.S("new/auth/internal/adapters/external/email/noop.go.tmpl", "internal/adapters/external/email/noop.go"),
	}

	if cfg.IsSQLite() {
		new = append(new, generator.S("new/auth_sqlite/internal/adapters/db/auth_store.go.tmpl", "internal/adapters/db/auth_store.go"))
	} else {
		new = append(new, generator.S("new/auth_postgres/internal/adapters/db/auth_store.go.tmpl", "internal/adapters/db/auth_store.go"))
	}

	if cfg.IsSSR() {
		new = append(new,
			generator.S("new/ssr_auth/web/components/auth/login.templ.tmpl", "web/components/auth/login.templ"),
			generator.S("new/ssr_auth/web/components/auth/signup.templ.tmpl", "web/components/auth/signup.templ"),
			generator.S("new/ssr_auth/web/components/auth/forgot-password.templ.tmpl", "web/components/auth/forgot-password.templ"),
			generator.S("new/ssr_auth/web/components/auth/reset-password.templ.tmpl", "web/components/auth/reset-password.templ"),
			generator.S("new/ssr_auth/web/components/auth/settings.templ.tmpl", "web/components/auth/settings.templ"),
			generator.S("new/ssr/web/components/dashboard.templ.tmpl", "web/components/dashboard.templ"),
		)
	}

	return infra, new
}

func createAuthMigration(db string) error {
	migrationsDir := filepath.Join("internal", "adapters", "db", "migrations")
	next, err := nextMigrationNumber(migrationsDir)
	if err != nil {
		return err
	}

	tmplPath := "new/auth_" + db + "/internal/adapters/store/migrations/add_auth.sql"
	content, err := render.File(tmplPath, nil)
	if err != nil {
		return fmt.Errorf("render auth migration: %w", err)
	}

	filename := filepath.Join(migrationsDir, fmt.Sprintf("%05d_add_auth.sql", next))
	if err := os.WriteFile(filename, content, 0644); err != nil {
		return err
	}
	fmt.Printf("  create  %s\n", filename)
	return nil
}

func setAuthInGogenYAML(cfg *config.GogenYAML) error {
	cfg.Auth = true
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(".gogen.yaml", data, 0644)
}

func writeFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0644)
}
