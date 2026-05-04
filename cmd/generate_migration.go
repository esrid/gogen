package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/esrid/gogen/internal/config"
	"github.com/esrid/gogen/internal/scaffold"
)

var migrationCmd = &cobra.Command{
	Use:     "migration <name> [field:type ...]",
	Short:   "Create a new migration file",
	Example: "  gogen g migration AddPartNumberToProducts part_number:string\n  gogen g migration RemovePartNumberFromProducts part_number:string\n  gogen g migration RenamePartNumberToSkuInProducts",
	Args:    cobra.MinimumNArgs(1),
	RunE:    runMigration,
}

func init() {
	generateCmd.AddCommand(migrationCmd)
}

var migNumRe = regexp.MustCompile(`^(\d+)_`)
var migAddToRe = regexp.MustCompile(`^Add.+To([A-Z][a-zA-Z0-9]*)$`)
var migRemoveFromRe = regexp.MustCompile(`^Remove.+From([A-Z][a-zA-Z0-9]*)$`)
var migRenameRe = regexp.MustCompile(`^Rename([A-Za-z0-9]+)To([A-Za-z0-9]+)In([A-Z][a-zA-Z0-9]*)$`)

func runMigration(_ *cobra.Command, args []string) error {
	rawName := strings.TrimSpace(args[0])
	name := strings.ToLower(strings.ReplaceAll(rawName, " ", "_"))
	fieldArgs := args[1:]

	cfg, err := loadGogenYAML()
	if err != nil {
		return err
	}

	migrationsDir := filepath.Join("internal", "adapters", "store", "migrations")
	if _, err := os.Stat(migrationsDir); err != nil {
		return fmt.Errorf("migrations dir not found: %s\nare you inside a gogen project?", migrationsDir)
	}

	next, err := nextMigrationNumber(migrationsDir)
	if err != nil {
		return err
	}

	filename := filepath.Join(migrationsDir, fmt.Sprintf("%05d_%s.sql", next, name))

	var content string
	if oldCol, newCol, table := parseRenameIntent(rawName); table != "" {
		content = renameMigrationContent(table, oldCol, newCol, cfg.DB)
	} else if table, action := parseMigrationIntent(rawName); table != "" && len(fieldArgs) > 0 {
		fields, err := scaffold.ParseFields(fieldArgs)
		if err != nil {
			return err
		}
		content = alterMigrationContent(action, table, fields, cfg.DB)
	} else {
		content = migrationContent(cfg.DB)
	}

	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		return err
	}

	fmt.Printf("  create  %s\n", filename)
	return nil
}

func parseRenameIntent(name string) (oldCol, newCol, table string) {
	m := migRenameRe.FindStringSubmatch(name)
	if m == nil {
		return "", "", ""
	}
	return scaffold.ToSnake(m[1]), scaffold.ToSnake(m[2]), scaffold.Pluralize(scaffold.ToSnake(m[3]))
}

func renameMigrationContent(table, oldCol, newCol, db string) string {
	up := fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s;", table, oldCol, newCol)
	down := fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s;", table, newCol, oldCol)
	if db == "sqlite" {
		return fmt.Sprintf("-- +goose Up\n-- +goose StatementBegin\n%s\n-- +goose StatementEnd\n\n-- +goose Down\n-- +goose StatementBegin\n%s\n-- +goose StatementEnd\n", up, down)
	}
	return fmt.Sprintf("-- +goose Up\n%s\n\n-- +goose Down\n%s\n", up, down)
}

func parseMigrationIntent(name string) (table, action string) {
	if m := migAddToRe.FindStringSubmatch(name); m != nil {
		return scaffold.Pluralize(scaffold.ToSnake(m[1])), "add"
	}
	if m := migRemoveFromRe.FindStringSubmatch(name); m != nil {
		return scaffold.Pluralize(scaffold.ToSnake(m[1])), "remove"
	}
	return "", ""
}

func alterMigrationContent(action, table string, fields []scaffold.Field, db string) string {
	var upLines, downLines []string
	for _, f := range fields {
		colDef := f.SQLiteCol
		if db == "postgres" {
			colDef = f.PGCol
		}
		if action == "add" {
			upLines = append(upLines, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s;", table, f.Name, colDef))
			downLines = append(downLines, fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", table, f.Name))
			if f.IsRef {
				upLines = append(upLines, fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_%s ON %s (%s);", table, f.Name, table, f.Name))
				downLines = append(downLines, fmt.Sprintf("DROP INDEX IF EXISTS idx_%s_%s;", table, f.Name))
			}
		} else {
			upLines = append(upLines, fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", table, f.Name))
			downLines = append(downLines, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s;", table, f.Name, colDef))
		}
	}

	up := strings.Join(upLines, "\n")
	down := strings.Join(downLines, "\n")
	if db == "sqlite" {
		return fmt.Sprintf("-- +goose Up\n-- +goose StatementBegin\n%s\n-- +goose StatementEnd\n\n-- +goose Down\n-- +goose StatementBegin\n%s\n-- +goose StatementEnd\n", up, down)
	}
	return fmt.Sprintf("-- +goose Up\n%s\n\n-- +goose Down\n%s\n", up, down)
}

func loadGogenYAML() (*config.GogenYAML, error) {
	data, err := os.ReadFile(".gogen.yaml")
	if err != nil {
		return nil, fmt.Errorf(".gogen.yaml not found: are you inside a gogen project?")
	}
	var cfg config.GogenYAML
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid .gogen.yaml: %w", err)
	}
	return &cfg, nil
}

func nextMigrationNumber(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}

	var nums []int
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".sql" {
			continue
		}
		m := migNumRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[1])
		nums = append(nums, n)
	}

	if len(nums) == 0 {
		return 1, nil
	}
	sort.Ints(nums)
	return nums[len(nums)-1] + 1, nil
}

func migrationContent(db string) string {
	if db == "postgres" {
		return "-- +goose Up\n\n\n-- +goose Down\n"
	}
	return "-- +goose Up\n-- +goose StatementBegin\n\n-- +goose StatementEnd\n\n-- +goose Down\n-- +goose StatementBegin\n\n-- +goose StatementEnd\n"
}
