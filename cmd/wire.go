package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/esrid/gogen/internal/config"
	"github.com/esrid/gogen/internal/scaffold"
)

const (
	markerRoutes          = "// gogen:inject:routes"
	markerProtectedRoutes = "// gogen:inject:protected-routes"
)

func autoWireScaffold(data *scaffold.Data, cfg *config.GogenYAML) {
	if flagDryRun {
		return
	}
	if err := wireRoutes(data); err != nil {
		fmt.Printf("  hint    wire routes.go manually (%v)\n", err)
	}
	if err := regenerateWireGen(cfg); err != nil {
		fmt.Printf("  hint    update wire_gen.go manually (%v)\n", err)
	}
}

func reWireAllScaffolds(cfg *config.GogenYAML) {
	for modelName, meta := range cfg.Scaffolds {
		fields, _ := scaffold.ParseFields(meta.Fields)
		projectCfg := &config.ProjectConfig{
			ModulePath: cfg.Module,
			DB:         cfg.DB,
			RenderMode: cfg.RenderMode,
			Auth:       cfg.Auth,
		}
		data := scaffold.NewData(modelName, fields, projectCfg)
		data.Protected = meta.Protected
		if err := wireRoutes(data); err != nil {
			fmt.Printf("  hint    wire routes.go for %s manually (%v)\n", modelName, err)
		}
	}
	if err := regenerateWireGen(cfg); err != nil {
		fmt.Printf("  hint    update wire_gen.go manually (%v)\n", err)
	}
}

func removeWireScaffold(data *scaffold.Data, cfg *config.GogenYAML) {
	if flagDryRun {
		return
	}
	removeFromRoutes(data)
	if err := regenerateWireGen(cfg); err != nil {
		fmt.Printf("  hint    update wire_gen.go manually (%v)\n", err)
	}
}

func wireRoutes(data *scaffold.Data) error {
	path := "internal/server/routes.go"
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	s := string(content)

	if strings.Contains(s, `"`+data.RoutePrefix+`"`) {
		return nil
	}

	tab := "\t"
	if data.Protected {
		tab = "\t\t"
	}
	markerLine := tab + func() string {
		if data.Protected {
			return markerProtectedRoutes
		}
		return markerRoutes
	}()

	if !strings.Contains(s, markerLine) {
		return fmt.Errorf("marker %q not found in routes.go", markerLine)
	}

	var toInject string
	if data.IsBoth() {
		toInject = fmt.Sprintf(
			"%[1]sif h.%[2]s != nil {\n%[1]s\tr.Mount(\"%[3]s\", h.%[2]s.Route())\n%[1]s}\n%[1]sif h.%[2]sAPI != nil {\n%[1]s\tr.Mount(\"/api%[3]s\", h.%[2]sAPI.Route())\n%[1]s}",
			tab, data.ModelName, data.RoutePrefix,
		)
	} else {
		toInject = fmt.Sprintf(
			"%[1]sif h.%[2]s != nil {\n%[1]s\tr.Mount(\"%[3]s\", h.%[2]s.Route())\n%[1]s}",
			tab, data.ModelName, data.RoutePrefix,
		)
	}

	s = injectBefore(s, markerLine, toInject)
	if err := os.WriteFile(path, []byte(s), 0644); err != nil {
		return err
	}
	fmt.Printf("  wire    %s\n", path)
	return nil
}

func removeFromRoutes(data *scaffold.Data) {
	path := "internal/server/routes.go"
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}
	s := string(content)

	tab := "\t"
	if data.Protected {
		tab = "\t\t"
	}

	ssrMount := fmt.Sprintf("%[1]sif h.%[2]s != nil {\n%[1]s\tr.Mount(\"%[3]s\", h.%[2]s.Route())\n%[1]s}\n",
		tab, data.ModelName, data.RoutePrefix)
	s = strings.Replace(s, ssrMount, "", 1)

	if data.IsBoth() {
		apiMount := fmt.Sprintf("%[1]sif h.%[2]sAPI != nil {\n%[1]s\tr.Mount(\"/api%[3]s\", h.%[2]sAPI.Route())\n%[1]s}\n",
			tab, data.ModelName, data.RoutePrefix)
		s = strings.Replace(s, apiMount, "", 1)
	}

	_ = os.WriteFile(path, []byte(s), 0644)
}

func regenerateWireGen(cfg *config.GogenYAML) error {
	if flagDryRun {
		return nil
	}

	path := "internal/server/wire_gen.go"
	modulePath := cfg.Module

	names := make([]string, 0, len(cfg.Scaffolds))
	for name := range cfg.Scaffolds {
		names = append(names, name)
	}
	sort.Strings(names)

	hasScaffolds := len(names) > 0
	needsAPI := hasScaffolds || cfg.Auth
	needsServices := hasScaffolds || cfg.Auth

	var b strings.Builder
	b.WriteString("package server\n\n")
	b.WriteString("import (\n")
	b.WriteString("\t\"log/slog\"\n")
	if needsAPI || cfg.Auth {
		b.WriteString("\n")
	}
	if needsAPI {
		b.WriteString("\tapi \"" + modulePath + "/internal/adapters/http\"\n")
	}
	if cfg.Auth {
		b.WriteString("\t\"" + modulePath + "/internal/adapters/external/email\"\n")
	}
	b.WriteString("\t\"" + modulePath + "/internal/adapters/store\"\n")
	if cfg.Auth {
		b.WriteString("\t\"" + modulePath + "/internal/core/ports\"\n")
	}
	if needsServices {
		b.WriteString("\t\"" + modulePath + "/internal/core/services\"\n")
	}
	b.WriteString(")\n\n")

	b.WriteString("type Handler struct {\n")
	b.WriteString("\tStore *store.Store\n")
	if cfg.Auth {
		b.WriteString("\tAuth           *api.AuthHandler\n")
		b.WriteString("\tSessionService ports.SessionService\n")
	}
	for _, name := range names {
		b.WriteString("\t" + name + " *api." + name + "Handler\n")
		if cfg.RenderMode == "both" {
			b.WriteString("\t" + name + "API *api." + name + "APIHandler\n")
		}
	}
	b.WriteString("}\n\n")

	b.WriteString("func WireHandlers(dbStore *store.Store, logger *slog.Logger) *Handler {\n")
	b.WriteString("\th := &Handler{Store: dbStore}\n")
	if cfg.Auth {
		b.WriteString("\n")
		b.WriteString("\temailProvider := email.NewNoopProvider()\n")
		b.WriteString("\tsessionSvc := services.NewSessionService(dbStore)\n")
		b.WriteString("\th.SessionService = sessionSvc\n")
		b.WriteString("\th.Auth = api.NewAuthHandler(services.NewAuthService(dbStore, emailProvider), dbStore, sessionSvc, logger)\n")
	}
	for _, name := range names {
		n := strings.ToLower(name[:1]) + name[1:]
		b.WriteString("\t" + n + "Svc := services.New" + name + "Service(dbStore)\n")
		b.WriteString("\th." + name + " = api.New" + name + "Handler(" + n + "Svc)\n")
		if cfg.RenderMode == "both" {
			b.WriteString("\th." + name + "API = api.New" + name + "APIHandler(" + n + "Svc)\n")
		}
	}
	b.WriteString("\treturn h\n")
	b.WriteString("}\n")

	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		return err
	}
	fmt.Printf("  wire    %s\n", path)
	return nil
}

func injectBefore(src, marker, code string) string {
	idx := strings.Index(src, marker)
	if idx < 0 {
		return ""
	}
	return src[:idx] + code + "\n" + src[idx:]
}
