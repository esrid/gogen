package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/esrid/gogen/internal/config"
	"github.com/esrid/gogen/internal/render"
)

var oauthCmd = &cobra.Command{
	Use:     "oauth <provider...>",
	Short:   "Add OAuth2 social login providers",
	Example: "  gogen g oauth google\n  gogen g oauth google apple microsoft",
	Args:    cobra.MinimumNArgs(1),
	RunE:    runGenerateOAuth,
}

func init() {
	generateCmd.AddCommand(oauthCmd)
}

type OAuthProvider struct {
	Name       string
	GothPkg    string
	GothImport string
	EnvPrefix  string
	IsApple    bool
}

type OAuthData struct {
	*config.ProjectConfig
	Providers []OAuthProvider
}

var knownOAuthProviders = map[string]OAuthProvider{
	"google": {
		Name:       "google",
		GothPkg:    "google",
		GothImport: "github.com/markbates/goth/providers/google",
		EnvPrefix:  "GOOGLE",
	},
	"apple": {
		Name:       "apple",
		GothPkg:    "apple",
		GothImport: "github.com/markbates/goth/providers/apple",
		EnvPrefix:  "APPLE",
		IsApple:    true,
	},
	"microsoft": {
		Name:       "microsoft",
		GothPkg:    "microsoft",
		GothImport: "github.com/markbates/goth/providers/microsoft",
		EnvPrefix:  "MICROSOFT",
	},
}

func runGenerateOAuth(_ *cobra.Command, args []string) error {
	gogenCfg, err := loadGogenYAML()
	if err != nil {
		return err
	}
	if !gogenCfg.Auth {
		return fmt.Errorf("oauth requires auth (run: gogen g auth first)")
	}

	// track already-enabled providers
	enabled := map[string]bool{}
	if gogenCfg.OAuth != nil {
		for _, p := range gogenCfg.OAuth.Providers {
			enabled[p] = true
		}
	}

	var newProviders []OAuthProvider
	for _, name := range args {
		name = strings.ToLower(strings.TrimSpace(name))
		p, ok := knownOAuthProviders[name]
		if !ok {
			return fmt.Errorf("unknown provider %q (supported: google, apple, microsoft)", name)
		}
		if enabled[name] {
			fmt.Printf("  skip    %s (already enabled)\n", name)
			continue
		}
		newProviders = append(newProviders, p)
		enabled[name] = true
	}

	if len(newProviders) == 0 {
		fmt.Println("Nothing to do.")
		return nil
	}

	cfg := &config.ProjectConfig{
		ModulePath: gogenCfg.Module,
		DB:         gogenCfg.DB,
		RenderMode: gogenCfg.RenderMode,
		Auth:       true,
	}

	// build full provider list (existing + new) for handler template
	allProviders := make([]OAuthProvider, 0)
	if gogenCfg.OAuth != nil {
		for _, name := range gogenCfg.OAuth.Providers {
			allProviders = append(allProviders, knownOAuthProviders[name])
		}
	}
	allProviders = append(allProviders, newProviders...)

	data := &OAuthData{
		ProjectConfig: cfg,
		Providers:     allProviders,
	}

	fmt.Println("\nAdding OAuth providers...")
	fmt.Println()

	firstTime := gogenCfg.OAuth == nil || len(gogenCfg.OAuth.Providers) == 0

	if !flagDryRun {
		if firstTime {
			// migration: add provider + provider_id to users
			if err := createOAuthMigration(gogenCfg.DB); err != nil {
				return err
			}
			// oauth_port.go
			if err := writeOAuthFile("oauth/internal/domain/oauth_port.go.tmpl", "internal/domain/oauth_port.go", data, false); err != nil {
				return err
			}
			// oauth_store.go
			storeTmpl := "oauth/internal/adapters/db/oauth_store_postgres.go.tmpl"
			if gogenCfg.DB == "sqlite" {
				storeTmpl = "oauth/internal/adapters/db/oauth_store_sqlite.go.tmpl"
			}
			if err := writeOAuthFile(storeTmpl, "internal/adapters/db/oauth_store.go", data, false); err != nil {
				return err
			}
		}

		// oauth_handler.go — always overwrite to register all providers
		if err := writeOAuthFile("oauth/internal/adapters/api/oauth_handler.go.tmpl", "internal/adapters/api/oauth_handler.go", data, true); err != nil {
			return err
		}

		// save to .gogen.yaml
		if gogenCfg.OAuth == nil {
			gogenCfg.OAuth = &config.OAuthConfig{}
		}
		for _, p := range newProviders {
			gogenCfg.OAuth.Providers = append(gogenCfg.OAuth.Providers, p.Name)
		}
		yamlData, err := yaml.Marshal(gogenCfg)
		if err != nil {
			return err
		}
		if err := os.WriteFile(".gogen.yaml", yamlData, 0644); err != nil {
			return err
		}

		reWireAllScaffolds(gogenCfg)
	} else {
		if firstTime {
			fmt.Printf("  dryrun  internal/adapters/db/migrations/NNNNN_add_oauth_to_users.sql\n")
			fmt.Printf("  dryrun  internal/domain/oauth_port.go\n")
			fmt.Printf("  dryrun  internal/adapters/db/oauth_store.go\n")
		}
		fmt.Printf("  dryrun  internal/adapters/api/oauth_handler.go\n")
	}

	fmt.Println("\nRequired env vars:")
	for _, p := range newProviders {
		if p.IsApple {
			fmt.Println("  APPLE_CLIENT_ID, APPLE_TEAM_ID, APPLE_KEY_ID, APPLE_PRIVATE_KEY")
		} else {
			fmt.Printf("  %s_CLIENT_ID, %s_CLIENT_SECRET\n", p.EnvPrefix, p.EnvPrefix)
		}
	}
	fmt.Println("  APP_URL (e.g. https://yourapp.com)")

	fmt.Println("\nInstall deps in your project:")
	fmt.Println("  go get github.com/markbates/goth github.com/gorilla/sessions")

	fmt.Println("\nDone.")
	return nil
}

func writeOAuthFile(tmplPath, outPath string, data *OAuthData, force bool) error {
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
	if exists {
		fmt.Printf("  update  %s\n", outPath)
	} else {
		fmt.Printf("  create  %s\n", outPath)
	}
	return nil
}

func createOAuthMigration(db string) error {
	migrationsDir := "internal/adapters/db/migrations"
	next, err := nextMigrationNumber(migrationsDir)
	if err != nil {
		return err
	}

	var sql string
	if db == "sqlite" {
		sql = "-- +goose Up\n-- +goose StatementBegin\nALTER TABLE users ADD COLUMN provider TEXT NOT NULL DEFAULT 'local';\n-- +goose StatementEnd\n-- +goose StatementBegin\nALTER TABLE users ADD COLUMN provider_id TEXT NOT NULL DEFAULT '';\n-- +goose StatementEnd\n\n-- +goose Down\n-- +goose StatementBegin\nALTER TABLE users DROP COLUMN provider_id;\n-- +goose StatementEnd\n-- +goose StatementBegin\nALTER TABLE users DROP COLUMN provider;\n-- +goose StatementEnd\n"
	} else {
		sql = "-- +goose Up\nALTER TABLE users ADD COLUMN provider TEXT NOT NULL DEFAULT 'local';\nALTER TABLE users ADD COLUMN provider_id TEXT NOT NULL DEFAULT '';\n\n-- +goose Down\nALTER TABLE users DROP COLUMN provider_id;\nALTER TABLE users DROP COLUMN provider;\n"
	}

	filename := fmt.Sprintf("%s/%05d_add_oauth_to_users.sql", migrationsDir, next)
	if err := os.WriteFile(filename, []byte(sql), 0644); err != nil {
		return err
	}
	fmt.Printf("  create  %s\n", filename)
	return nil
}
