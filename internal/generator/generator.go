package generator

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/esrid/gogen/internal/config"
	"github.com/esrid/gogen/internal/render"
)

type Config struct {
	Force  bool
	DryRun bool
	Skip   bool
}

type Generator struct {
	cfg      Config
	reporter *Reporter
}

func New(cfg Config) *Generator {
	return &Generator{
		cfg:      cfg,
		reporter: NewReporter(cfg.DryRun),
	}
}

type FileSpec struct {
	TemplatePath string
	OutputPath   string
}

func (g *Generator) GenerateProject(cfg *config.ProjectConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	outDir := cfg.ProjectName
	specs := fileSpecs(cfg)

	fmt.Printf("\nGenerating %s...\n\n", cfg.ProjectName)

	for _, spec := range specs {
		outPath := filepath.Join(outDir, spec.OutputPath)

		if !g.cfg.DryRun && !g.cfg.Force {
			if _, err := os.Stat(outPath); err == nil {
				if g.cfg.Skip {
					g.reporter.Skipped(spec.OutputPath)
					continue
				}
				g.reporter.Conflict(spec.OutputPath)
				continue
			}
		}

		content, err := render.File(spec.TemplatePath, cfg)
		if err != nil {
			return fmt.Errorf("render %s: %w", spec.TemplatePath, err)
		}

		if g.cfg.DryRun {
			g.reporter.DryRun(spec.OutputPath)
			continue
		}

		if err := writeFile(outPath, content); err != nil {
			return fmt.Errorf("write %s: %w", outPath, err)
		}
		g.reporter.Created(spec.OutputPath)
	}

	if g.cfg.DryRun {
		fmt.Println("\nDry run complete. No files written.")
		return nil
	}

	return PostProcess(outDir)
}

func writeFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0644)
}

func S(tmpl, out string) FileSpec {
	return FileSpec{TemplatePath: tmpl, OutputPath: out}
}

func s(tmpl, out string) FileSpec { return S(tmpl, out) }

func fileSpecs(cfg *config.ProjectConfig) []FileSpec {
	specs := []FileSpec{
		s("new/base/go.mod.tmpl", "go.mod"),
		s("new/base/main.go.tmpl", "main.go"),
		s("new/base/dot_gitignore.tmpl", ".gitignore"),
		s("new/base/dot_env.tmpl", ".env"),
		s("new/base/dot_air.toml.tmpl", ".air.toml"),
		s("new/base/Makefile.tmpl", "Makefile"),
		s("new/base/Dockerfile.tmpl", "Dockerfile"),
		s("new/base/docker-compose.yml.tmpl", "docker-compose.yml"),
		s("new/base/dot_dockerignore.tmpl", ".dockerignore"),
		s("new/base/dot_gogen.yaml.tmpl", ".gogen.yaml"),
		s("new/base/bootstrap/app.go.tmpl", "bootstrap/app.go"),
		s("new/base/bootstrap/config.go.tmpl", "bootstrap/config.go"),
		s("new/base/bootstrap/server.go.tmpl", "bootstrap/server.go"),
		s("new/base/bootstrap/router.go.tmpl", "bootstrap/router.go"),
		s("new/base/bootstrap/wire_gen.go.tmpl", "bootstrap/wire_gen.go"),
		s("new/base/internal/adapters/api/controller.go.tmpl", "internal/adapters/api/controller.go"),
		s("new/base/internal/adapters/api/middleware.go.tmpl", "internal/adapters/api/middleware.go"),
		s("new/base/internal/domain/errors.go.tmpl", "internal/domain/errors.go"),
		s("new/base/internal/domain/session_port.go.tmpl", "internal/domain/session_port.go"),
		s("new/base/internal/utils/http_utils.go.tmpl", "internal/utils/http_utils.go"),
		// DB adapter
		s("new/db/"+cfg.DB+"/internal/adapters/db/store.go.tmpl", "internal/adapters/db/store.go"),
		s("new/db/"+cfg.DB+"/internal/adapters/db/migrations.go.tmpl", "internal/adapters/db/migrations.go"),
	}

	if cfg.Auth {
		specs = append(specs,
			s("new/auth/internal/domain/user.go.tmpl", "internal/domain/user.go"),
			s("new/auth/internal/domain/auth_port.go.tmpl", "internal/domain/auth_port.go"),
			s("new/auth/internal/domain/email_port.go.tmpl", "internal/domain/email_port.go"),
			s("new/auth/internal/application/auth_service.go.tmpl", "internal/application/auth_service.go"),
			s("new/auth/internal/application/session_service.go.tmpl", "internal/application/session_service.go"),
			s("new/auth/internal/utils/validation.go.tmpl", "internal/utils/validation.go"),
			s("new/auth/internal/adapters/api/auth_handler.go.tmpl", "internal/adapters/api/auth_handler.go"),
			s("new/auth/internal/adapters/api/middleware_auth.go.tmpl", "internal/adapters/api/middleware_auth.go"),
			s("new/auth/internal/adapters/external/email/noop.go.tmpl", "internal/adapters/external/email/noop.go"),
		)
		if cfg.IsSQLite() {
			specs = append(specs,
				s("new/auth_sqlite/internal/adapters/db/auth_store.go.tmpl", "internal/adapters/db/auth_store.go"),
				s("new/auth_sqlite/internal/adapters/store/migrations/00001_init.sql", "internal/adapters/db/migrations/00001_init.sql"),
			)
		} else {
			specs = append(specs,
				s("new/auth_postgres/internal/adapters/db/auth_store.go.tmpl", "internal/adapters/db/auth_store.go"),
				s("new/auth_postgres/internal/adapters/store/migrations/00001_init.sql", "internal/adapters/db/migrations/00001_init.sql"),
			)
		}
	} else {
		specs = append(specs,
			s("new/base/internal/adapters/store/migrations/00001_init.sql", "internal/adapters/db/migrations/00001_init.sql"),
		)
	}

	if cfg.IsSSR() {
		specs = append(specs,
			s("new/ssr/web/renderer.go.tmpl", "web/renderer.go"),
			s("new/ssr/web/static.go.tmpl", "web/static.go"),
			s("new/ssr/web/static/robots.txt", "web/static/robots.txt"),
			s("new/ssr/web/layouts/layout.templ.tmpl", "web/layouts/layout.templ"),
			s("new/ssr/web/components/components.templ.tmpl", "web/components/components.templ"),
			s("new/ssr/web/components/landing.templ.tmpl", "web/components/landing.templ"),
			s("new/ssr/web/components/error.templ.tmpl", "web/components/error.templ"),
		)
		if cfg.Auth {
			specs = append(specs,
				s("new/ssr_auth/web/components/auth/login.templ.tmpl", "web/components/auth/login.templ"),
				s("new/ssr_auth/web/components/auth/signup.templ.tmpl", "web/components/auth/signup.templ"),
				s("new/ssr_auth/web/components/auth/forgot-password.templ.tmpl", "web/components/auth/forgot-password.templ"),
				s("new/ssr_auth/web/components/auth/reset-password.templ.tmpl", "web/components/auth/reset-password.templ"),
				s("new/ssr_auth/web/components/auth/settings.templ.tmpl", "web/components/auth/settings.templ"),
			)
		}
	}

	if cfg.IsAPI() {
		specs = append(specs,
			s("new/api/internal/adapters/api/response.go.tmpl", "internal/adapters/api/response.go"),
		)
	}

	return specs
}
