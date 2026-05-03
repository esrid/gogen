package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/esrid/gogen/internal/config"
	"github.com/esrid/gogen/internal/generator"
)

var newCmd = &cobra.Command{
	Use:   "new [project-name]",
	Short: "Create a new Go project",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runNew,
}

func init() {
	rootCmd.AddCommand(newCmd)
	newCmd.Flags().StringP("module", "m", "", "Go module path (e.g. github.com/user/myapp)")
	newCmd.Flags().StringP("db", "d", "", "Database: sqlite or postgres")
	newCmd.Flags().StringP("render", "r", "", "Render mode: ssr, api, or both")
	newCmd.Flags().Bool("auth", false, "Include auth")
	newCmd.Flags().Bool("no-auth", false, "Exclude auth (skip prompt)")
}

func runNew(cmd *cobra.Command, args []string) error {
	cfg := &config.ProjectConfig{
		Year: time.Now().Year(),
	}

	if len(args) > 0 {
		cfg.ProjectName = args[0]
	}
	if m, _ := cmd.Flags().GetString("module"); m != "" {
		cfg.ModulePath = m
	}
	if d, _ := cmd.Flags().GetString("db"); d != "" {
		cfg.DB = d
	}
	if r, _ := cmd.Flags().GetString("render"); r != "" {
		cfg.RenderMode = r
	}

	noAuth, _ := cmd.Flags().GetBool("no-auth")
	if noAuth {
		cfg.Auth = false
		cfg.AuthSet = true
	} else if cmd.Flags().Changed("auth") {
		cfg.Auth, _ = cmd.Flags().GetBool("auth")
		cfg.AuthSet = true
	}

	if err := promptMissing(cfg); err != nil {
		return err
	}

	gen := generator.New(generator.Config{
		Force:  flagForce,
		DryRun: flagDryRun,
		Skip:   flagSkip,
	})

	return gen.GenerateProject(cfg)
}

func promptMissing(cfg *config.ProjectConfig) error {
	var fields []huh.Field

	if cfg.ProjectName == "" {
		fields = append(fields, huh.NewInput().
			Title("Project name").
			Placeholder("myapp").
			Value(&cfg.ProjectName).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("project name required")
				}
				return nil
			}),
		)
	}

	if cfg.ModulePath == "" {
		fields = append(fields, huh.NewInput().
			Title("Module path").
			Description("e.g. github.com/user/myapp").
			Value(&cfg.ModulePath).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("module path required")
				}
				return nil
			}),
		)
	}

	if cfg.DB == "" {
		fields = append(fields, huh.NewSelect[string]().
			Title("Database").
			Options(
				huh.NewOption("SQLite  (embedded, file-based)", "sqlite"),
				huh.NewOption("PostgreSQL", "postgres"),
			).
			Value(&cfg.DB),
		)
	}

	if cfg.RenderMode == "" {
		fields = append(fields, huh.NewSelect[string]().
			Title("Render mode").
			Options(
				huh.NewOption("SSR  – HTML templates", "ssr"),
				huh.NewOption("API  – JSON responses", "api"),
				huh.NewOption("Both – SSR + API", "both"),
			).
			Value(&cfg.RenderMode),
		)
	}

	if !cfg.AuthSet {
		fields = append(fields, huh.NewConfirm().
			Title("Include authentication?").
			Value(&cfg.Auth),
		)
		cfg.AuthSet = true
	}

	if len(fields) == 0 {
		return nil
	}

	return huh.NewForm(huh.NewGroup(fields...)).Run()
}
