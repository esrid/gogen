package generator

import (
	"sort"
	"strings"

	"github.com/esrid/gogen/internal/config"
)

func WireGenContent(cfg *config.GogenYAML) []byte {
	modulePath := cfg.Module

	names := make([]string, 0, len(cfg.Scaffolds))
	for name := range cfg.Scaffolds {
		names = append(names, name)
	}
	sort.Strings(names)

	hasScaffolds := len(names) > 0
	isSSR := cfg.RenderMode == "ssr" || cfg.RenderMode == "both"
	needsWeb := isSSR && hasScaffolds

	var b strings.Builder
	b.WriteString("package bootstrap\n\n")
	b.WriteString("import (\n")
	b.WriteString("\t\"log/slog\"\n")
	b.WriteString("\n")
	b.WriteString("\t\"" + modulePath + "/internal/adapters/api\"\n")
	if needsWeb {
		b.WriteString("\t\"" + modulePath + "/internal/adapters/web\"\n")
	}
	if cfg.Auth {
		b.WriteString("\t\"" + modulePath + "/internal/adapters/external/email\"\n")
		b.WriteString("\t\"" + modulePath + "/internal/application\"\n")
		b.WriteString("\t\"" + modulePath + "/internal/domain\"\n")
	} else if hasScaffolds {
		b.WriteString("\t\"" + modulePath + "/internal/application\"\n")
	}
	b.WriteString("\t\"" + modulePath + "/internal/adapters/db\"\n")
	b.WriteString(")\n\n")

	b.WriteString("type Handlers struct {\n")
	b.WriteString("\tStore *db.Store\n")
	if cfg.Auth {
		b.WriteString("\tAuth           *api.AuthHandler\n")
		b.WriteString("\tSessionService domain.SessionService\n")
	}
	for _, name := range names {
		if isSSR {
			b.WriteString("\t" + name + " *web." + name + "Handler\n")
		} else {
			b.WriteString("\t" + name + " *api." + name + "Handler\n")
		}
		if cfg.RenderMode == "both" {
			b.WriteString("\t" + name + "API *api." + name + "APIHandler\n")
		}
	}
	b.WriteString("\n")
	b.WriteString("\tPublicControllers    []api.Controller\n")
	b.WriteString("\tProtectedControllers []api.Controller\n")
	b.WriteString("}\n\n")

	b.WriteString("func WireHandlers(dbStore *db.Store, logger *slog.Logger) *Handlers {\n")
	b.WriteString("\th := &Handlers{Store: dbStore}\n")
	if cfg.Auth {
		b.WriteString("\n")
		b.WriteString("\temailProvider := email.NewNoopProvider()\n")
		b.WriteString("\tsessionSvc := application.NewSessionService(dbStore)\n")
		b.WriteString("\th.SessionService = sessionSvc\n")
		b.WriteString("\th.Auth = api.NewAuthHandler(application.NewAuthService(dbStore, emailProvider), dbStore, sessionSvc, logger)\n")
		b.WriteString("\th.PublicControllers = append(h.PublicControllers, h.Auth)\n")
	}

	for _, name := range names {
		n := strings.ToLower(name[:1]) + name[1:]
		meta := cfg.Scaffolds[name]

		b.WriteString("\n")
		b.WriteString("\t" + n + "Svc := application.New" + name + "Service(dbStore)\n")
		if isSSR {
			b.WriteString("\th." + name + " = web.New" + name + "Handler(" + n + "Svc)\n")
		} else {
			b.WriteString("\th." + name + " = api.New" + name + "Handler(" + n + "Svc)\n")
		}

		if meta.Protected {
			b.WriteString("\th.ProtectedControllers = append(h.ProtectedControllers, h." + name + ")\n")
		} else {
			b.WriteString("\th.PublicControllers = append(h.PublicControllers, h." + name + ")\n")
		}

		if cfg.RenderMode == "both" {
			b.WriteString("\th." + name + "API = api.New" + name + "APIHandler(" + n + "Svc)\n")
			if meta.Protected {
				b.WriteString("\th.ProtectedControllers = append(h.ProtectedControllers, h." + name + "API)\n")
			} else {
				b.WriteString("\th.PublicControllers = append(h.PublicControllers, h." + name + "API)\n")
			}
		}
	}
	b.WriteString("\n\treturn h\n")
	b.WriteString("}\n")

	return []byte(b.String())
}
