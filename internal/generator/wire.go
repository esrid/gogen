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

	ctrlNames := make([]string, 0, len(cfg.Controllers))
	for name := range cfg.Controllers {
		ctrlNames = append(ctrlNames, name)
	}
	sort.Strings(ctrlNames)

	if cfg.Auth {
		filtered := names[:0]
		for _, n := range names {
			if n != "User" {
				filtered = append(filtered, n)
			}
		}
		names = filtered
	}
	hasScaffolds := len(names) > 0
	hasControllers := len(ctrlNames) > 0
	isSSR := cfg.RenderMode == "ssr" || cfg.RenderMode == "both"
	needsWeb := isSSR && (hasScaffolds || hasControllers)
	needsAPIForSSR := func() bool {
		if cfg.RenderMode != "ssr" {
			return false
		}
		for _, m := range cfg.Scaffolds {
			if m.API {
				return true
			}
		}
		return false
	}()

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
	if cfg.Auth {
		b.WriteString("\tAuth           *api.AuthHandler\n")
		b.WriteString("\tSessionService domain.SessionService\n")
	}
	if cfg.HasOAuth() {
		b.WriteString("\tOAuth *api.OAuthHandler\n")
	}
	for _, name := range names {
		meta := cfg.Scaffolds[name]
		if isSSR {
			b.WriteString("\t" + name + " *web." + name + "Handler\n")
		} else {
			b.WriteString("\t" + name + " *api." + name + "Handler\n")
		}
		if cfg.RenderMode == "both" || (needsAPIForSSR && meta.API) {
			b.WriteString("\t" + name + "API *api." + name + "APIHandler\n")
		}
	}
	for _, name := range ctrlNames {
		if isSSR {
			b.WriteString("\t" + name + " *web." + name + "Handler\n")
		} else {
			b.WriteString("\t" + name + " *api." + name + "Handler\n")
		}
	}
	b.WriteString("\n")
	b.WriteString("\tPublicControllers    []api.Controller\n")
	b.WriteString("\tProtectedControllers []api.Controller\n")
	b.WriteString("}\n\n")

	b.WriteString("func WireHandlers(dbStore *db.Store, logger *slog.Logger) *Handlers {\n")
	b.WriteString("\th := &Handlers{}\n")
	if cfg.Auth {
		b.WriteString("\n")
		b.WriteString("\temailProvider := email.NewNoopProvider()\n")
		b.WriteString("\tsessionSvc := application.NewSessionService(dbStore)\n")
		b.WriteString("\th.SessionService = sessionSvc\n")
		b.WriteString("\th.Auth = api.NewAuthHandler(application.NewAuthService(dbStore, emailProvider), dbStore, sessionSvc, logger)\n")
		b.WriteString("\th.PublicControllers = append(h.PublicControllers, h.Auth)\n")
		if cfg.HasOAuth() {
			b.WriteString("\th.OAuth = api.NewOAuthHandler(dbStore, sessionSvc)\n")
			b.WriteString("\th.PublicControllers = append(h.PublicControllers, h.OAuth)\n")
		}
	}

	for _, name := range names {
		n := strings.ToLower(name[:1]) + name[1:]
		meta := cfg.Scaffolds[name]

		b.WriteString("\n")
		b.WriteString("\t" + n + "Svc := application.New" + name + "Service(dbStore)\n")
		if isSSR {
			b.WriteString("\th." + name + " = web.New" + name + "Handler(" + n + "Svc, logger)\n")
		} else {
			b.WriteString("\th." + name + " = api.New" + name + "Handler(" + n + "Svc, logger)\n")
		}

		if meta.Protected {
			b.WriteString("\th.ProtectedControllers = append(h.ProtectedControllers, h." + name + ")\n")
		} else {
			b.WriteString("\th.PublicControllers = append(h.PublicControllers, h." + name + ")\n")
		}

		if cfg.RenderMode == "both" || (needsAPIForSSR && meta.API) {
			b.WriteString("\th." + name + "API = api.New" + name + "APIHandler(" + n + "Svc, logger)\n")
			if meta.Protected {
				b.WriteString("\th.ProtectedControllers = append(h.ProtectedControllers, h." + name + "API)\n")
			} else {
				b.WriteString("\th.PublicControllers = append(h.PublicControllers, h." + name + "API)\n")
			}
		}
	}
	for _, name := range ctrlNames {
		meta := cfg.Controllers[name]
		b.WriteString("\n")
		if isSSR {
			b.WriteString("\th." + name + " = web.New" + name + "Handler()\n")
		} else {
			b.WriteString("\th." + name + " = api.New" + name + "Handler()\n")
		}
		if meta.Protected {
			b.WriteString("\th.ProtectedControllers = append(h.ProtectedControllers, h." + name + ")\n")
		} else {
			b.WriteString("\th.PublicControllers = append(h.PublicControllers, h." + name + ")\n")
		}
	}

	b.WriteString("\n\treturn h\n")
	b.WriteString("}\n")

	return []byte(b.String())
}
