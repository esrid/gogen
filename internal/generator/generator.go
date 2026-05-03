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
		s("new/base/internal/server/config.go.tmpl", "internal/server/config.go"),
		s("new/base/internal/server/server.go.tmpl", "internal/server/server.go"),
		s("new/base/internal/server/routes.go.tmpl", "internal/server/routes.go"),
		s("new/base/internal/server/wire_gen.go.tmpl", "internal/server/wire_gen.go"),
		s("new/base/internal/adapters/http/middleware.go.tmpl", "internal/adapters/http/middleware.go"),
		s("new/base/internal/core/domains/errors.go.tmpl", "internal/core/domains/errors.go"),
		s("new/base/internal/core/ports/session_port.go.tmpl", "internal/core/ports/session_port.go"),
		s("new/base/internal/core/utils/http_utils.go.tmpl", "internal/core/utils/http_utils.go"),
		// DB store
		s("new/db/"+cfg.DB+"/internal/adapters/store/store.go.tmpl", "internal/adapters/store/store.go"),
		s("new/db/"+cfg.DB+"/internal/adapters/store/migrations.go.tmpl", "internal/adapters/store/migrations.go"),
	}

	if cfg.Auth {
		specs = append(specs,
			s("new/auth/internal/core/domains/user.go.tmpl", "internal/core/domains/user.go"),
			s("new/auth/internal/core/ports/auth_port.go.tmpl", "internal/core/ports/auth_port.go"),
			s("new/auth/internal/core/ports/email_port.go.tmpl", "internal/core/ports/email_port.go"),
			s("new/auth/internal/core/services/auth_service.go.tmpl", "internal/core/services/auth_service.go"),
			s("new/auth/internal/core/services/session_service.go.tmpl", "internal/core/services/session_service.go"),
			s("new/auth/internal/core/utils/validation.go.tmpl", "internal/core/utils/validation.go"),
			s("new/auth/internal/adapters/http/auth_handler.go.tmpl", "internal/adapters/http/auth_handler.go"),
			s("new/auth/internal/adapters/http/middleware_auth.go.tmpl", "internal/adapters/http/middleware_auth.go"),
			s("new/auth/internal/adapters/external/email/noop.go.tmpl", "internal/adapters/external/email/noop.go"),
		)
		if cfg.IsSQLite() {
			specs = append(specs,
				s("new/auth_sqlite/internal/adapters/store/auth_store.go.tmpl", "internal/adapters/store/auth_store.go"),
				s("new/auth_sqlite/internal/adapters/store/migrations/00001_init.sql", "internal/adapters/store/migrations/00001_init.sql"),
			)
		} else {
			specs = append(specs,
				s("new/auth_postgres/internal/adapters/store/auth_store.go.tmpl", "internal/adapters/store/auth_store.go"),
				s("new/auth_postgres/internal/adapters/store/migrations/00001_init.sql", "internal/adapters/store/migrations/00001_init.sql"),
			)
		}
	} else {
		specs = append(specs,
			s("new/base/internal/adapters/store/migrations/00001_init.sql", "internal/adapters/store/migrations/00001_init.sql"),
		)
	}

	if cfg.IsSSR() {
		specs = append(specs,
			s("new/ssr/web/renderer.go.tmpl", "web/renderer.go"),
			s("new/ssr/web/static.go.tmpl", "web/static.go"),
			s("new/ssr/web/static/robots.txt", "web/static/robots.txt"),
			s("new/ssr/web/templates/layout.html", "web/templates/layout.html"),
			s("new/ssr/web/templates/components/components.html", "web/templates/components/components.html"),
			s("new/ssr/web/templates/pages/landing.html", "web/templates/pages/landing.html"),
			s("new/ssr/web/templates/pages/error.html", "web/templates/pages/error.html"),
		)
		if cfg.Auth {
			specs = append(specs,
				s("new/ssr_auth/web/templates/pages/login.html", "web/templates/pages/login.html"),
				s("new/ssr_auth/web/templates/pages/signup.html", "web/templates/pages/signup.html"),
				s("new/ssr_auth/web/templates/pages/forgot-password.html", "web/templates/pages/forgot-password.html"),
				s("new/ssr_auth/web/templates/pages/reset-password.html", "web/templates/pages/reset-password.html"),
				s("new/ssr_auth/web/templates/pages/settings.html", "web/templates/pages/settings.html"),
			)
		}
	}

	if cfg.IsAPI() {
		specs = append(specs,
			s("new/api/internal/adapters/http/response.go.tmpl", "internal/adapters/http/response.go"),
		)
	}

	return specs
}
