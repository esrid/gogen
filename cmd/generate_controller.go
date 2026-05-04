package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/esrid/gogen/internal/config"
	"github.com/esrid/gogen/internal/render"
	"github.com/esrid/gogen/internal/scaffold"
)

var controllerCmd = &cobra.Command{
	Use:     "controller <Name>",
	Aliases: []string{"c", "ctrl"},
	Short:   "Generate a simple page controller",
	Example: "  gogen g controller Contact\n  gogen g controller Dashboard --protected\n  gogen g controller About --route /about-us",
	Args:    cobra.ExactArgs(1),
	RunE:    runGenerateController,
}

func init() {
	generateCmd.AddCommand(controllerCmd)
	controllerCmd.Flags().Bool("protected", false, "Mount route inside the auth-protected group")
	controllerCmd.Flags().String("route", "", "Custom route path (default: /name)")
}

func runGenerateController(cmd *cobra.Command, args []string) error {
	name := scaffold.ToCamel(args[0])
	protected, _ := cmd.Flags().GetBool("protected")
	route, _ := cmd.Flags().GetString("route")

	gogenCfg, err := loadGogenYAML()
	if err != nil {
		return err
	}

	if protected && !gogenCfg.Auth {
		return fmt.Errorf("--protected requires auth (run: gogen g auth)")
	}

	if gogenCfg.Controllers != nil {
		if _, exists := gogenCfg.Controllers[name]; exists {
			return fmt.Errorf("controller %q already exists", name)
		}
	}

	n := strings.ToLower(name[:1]) + name[1:]
	if route == "" {
		route = "/" + strings.ToLower(name)
	}

	data := &scaffold.ControllerData{
		Name:       name,
		NameLC:     n,
		Route:      route,
		ModulePath: gogenCfg.Module,
		RenderMode: gogenCfg.RenderMode,
	}

	fmt.Printf("\nGenerating %s controller...\n\n", name)

	isSSR := gogenCfg.RenderMode == "ssr" || gogenCfg.RenderMode == "both"
	isAPI := gogenCfg.RenderMode == "api" || gogenCfg.RenderMode == "both"

	if isSSR {
		if err := writeControllerFile("controller/handler_ssr.go.tmpl", "internal/adapters/web/"+n+"_handler.go", data); err != nil {
			return err
		}
		if err := writeControllerFile("controller/page.templ.tmpl", "web/components/"+n+"/page.templ", data); err != nil {
			return err
		}
	}
	if isAPI {
		if err := writeControllerFile("controller/handler.go.tmpl", "internal/adapters/api/"+n+"_handler.go", data); err != nil {
			return err
		}
	}

	if !flagDryRun {
		if gogenCfg.Controllers == nil {
			gogenCfg.Controllers = make(map[string]*config.ControllerMeta)
		}
		gogenCfg.Controllers[name] = &config.ControllerMeta{Route: route, Protected: protected}
		yamlData, err := yaml.Marshal(gogenCfg)
		if err != nil {
			return err
		}
		if err := os.WriteFile(".gogen.yaml", yamlData, 0644); err != nil {
			return err
		}
		if err := regenerateWireGen(gogenCfg); err != nil {
			fmt.Printf("  hint    update wire_gen.go manually (%v)\n", err)
		}
	}

	fmt.Println("\nDone.")
	return nil
}

func writeControllerFile(tmplPath, outPath string, data *scaffold.ControllerData) error {
	if _, err := os.Stat(outPath); err == nil {
		if flagForce {
			// fall through to overwrite
		} else if flagSkip || flagDryRun {
			fmt.Printf("  skip    %s\n", outPath)
			return nil
		} else {
			fmt.Printf("  conflict %s\n", outPath)
			return nil
		}
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
