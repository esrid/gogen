package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/esrid/gogen/internal/config"
	"github.com/esrid/gogen/internal/scaffold"
)

const (
	markerHandlers        = "// gogen:inject:handlers"
	markerRoutes          = "// gogen:inject:routes"
	markerProtectedRoutes = "// gogen:inject:protected-routes"
	markerServices        = "// gogen:inject:services"
)

func autoWireScaffold(data *scaffold.Data, modulePath string) {
	if flagDryRun {
		return
	}
	if err := wireRoutes(data); err != nil {
		fmt.Printf("  hint    wire routes.go manually (%v)\n", err)
	}
	if err := wireMain(data, modulePath); err != nil {
		fmt.Printf("  hint    wire main.go manually (%v)\n", err)
	}
}

// reWireAllScaffolds re-applies all scaffold wiring after infra files are regenerated.
func reWireAllScaffolds(gogenCfg *config.GogenYAML) {
	for modelName, meta := range gogenCfg.Scaffolds {
		fields, _ := scaffold.ParseFields(meta.Fields)
		cfg := &config.ProjectConfig{
			ModulePath: gogenCfg.Module,
			DB:         gogenCfg.DB,
			RenderMode: gogenCfg.RenderMode,
			Auth:       gogenCfg.Auth,
		}
		data := scaffold.NewData(modelName, fields, cfg)
		data.Protected = meta.Protected
		autoWireScaffold(data, gogenCfg.Module)
	}
}

func removeWireScaffold(data *scaffold.Data) {
	if flagDryRun {
		return
	}
	removeFromRoutes(data)
	removeFromMain(data)
}

func wireRoutes(data *scaffold.Data) error {
	path := "internal/server/routes.go"
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	s := string(content)

	if strings.Contains(s, data.ModelName+"Handler") {
		return nil
	}

	// Inject handler field (single-tab, inside struct body)
	field := "\t" + data.ModelName + " *api." + data.ModelName + "Handler"
	updated := injectBefore(s, "\t"+markerHandlers, field)
	if updated == "" {
		return fmt.Errorf("marker %q not found in routes.go", markerHandlers)
	}
	s = updated

	// Inject route mount — protected routes are double-indented (inside r.Group func literal)
	var mount, markerLine string
	if data.Protected {
		mount = fmt.Sprintf("\t\tif h.%s != nil {\n\t\t\tr.Mount(\"%s\", h.%s.Route())\n\t\t}",
			data.ModelName, data.RoutePrefix, data.ModelName)
		markerLine = "\t\t" + markerProtectedRoutes
	} else {
		mount = fmt.Sprintf("\tif h.%s != nil {\n\t\tr.Mount(\"%s\", h.%s.Route())\n\t}",
			data.ModelName, data.RoutePrefix, data.ModelName)
		markerLine = "\t" + markerRoutes
	}

	if !strings.Contains(s, markerLine) {
		return fmt.Errorf("marker %q not found in routes.go", markerLine)
	}
	s = injectBefore(s, markerLine, mount)

	if err := os.WriteFile(path, []byte(s), 0644); err != nil {
		return err
	}
	fmt.Printf("  wire    %s\n", path)
	return nil
}

func wireMain(data *scaffold.Data, modulePath string) error {
	path := "main.go"
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	s := string(content)

	if strings.Contains(s, "New"+data.ModelName+"Service") {
		return nil
	}

	if !strings.Contains(s, markerServices) {
		return fmt.Errorf("marker %q not found in main.go", markerServices)
	}

	// Add missing imports
	for _, imp := range []string{
		`"` + modulePath + `/internal/core/services"`,
		`api "` + modulePath + `/internal/adapters/http"`,
	} {
		if !strings.Contains(s, imp) {
			s = addGoImport(s, imp)
		}
	}

	n := strings.ToLower(data.ModelName[:1]) + data.ModelName[1:]
	wiring := fmt.Sprintf(
		"\t%sService := services.New%sService(dbStore)\n\thandlers.%s = api.New%sHandler(%sService)",
		n, data.ModelName, data.ModelName, data.ModelName, n,
	)
	s = injectBefore(s, "\t"+markerServices, wiring)

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

	field := "\t" + data.ModelName + " *api." + data.ModelName + "Handler\n"
	s = strings.Replace(s, field, "", 1)

	var mount string
	if data.Protected {
		mount = fmt.Sprintf("\t\tif h.%s != nil {\n\t\t\tr.Mount(\"%s\", h.%s.Route())\n\t\t}\n",
			data.ModelName, data.RoutePrefix, data.ModelName)
	} else {
		mount = fmt.Sprintf("\tif h.%s != nil {\n\t\tr.Mount(\"%s\", h.%s.Route())\n\t}\n",
			data.ModelName, data.RoutePrefix, data.ModelName)
	}
	s = strings.Replace(s, mount, "", 1)

	_ = os.WriteFile(path, []byte(s), 0644)
}

func removeFromMain(data *scaffold.Data) {
	path := "main.go"
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}
	s := string(content)

	n := strings.ToLower(data.ModelName[:1]) + data.ModelName[1:]
	wiring := fmt.Sprintf(
		"\t%sService := services.New%sService(dbStore)\n\thandlers.%s = api.New%sHandler(%sService)\n",
		n, data.ModelName, data.ModelName, data.ModelName, n,
	)
	s = strings.Replace(s, wiring, "", 1)

	_ = os.WriteFile(path, []byte(s), 0644)
}

func injectBefore(src, marker, code string) string {
	idx := strings.Index(src, marker)
	if idx < 0 {
		return ""
	}
	return src[:idx] + code + "\n" + src[idx:]
}

func addGoImport(src, imp string) string {
	idx := strings.Index(src, "import (")
	if idx < 0 {
		return src
	}
	end := strings.Index(src[idx:], ")")
	if end < 0 {
		return src
	}
	insertAt := idx + end
	return src[:insertAt] + "\t" + imp + "\n" + src[insertAt:]
}
