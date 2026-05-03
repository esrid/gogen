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
)

var migrationCmd = &cobra.Command{
	Use:   "migration <name>",
	Short: "Create a new migration file",
	Args:  cobra.ExactArgs(1),
	RunE:  runMigration,
}

func init() {
	generateCmd.AddCommand(migrationCmd)
}

var migNumRe = regexp.MustCompile(`^(\d+)_`)

func runMigration(_ *cobra.Command, args []string) error {
	name := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(args[0]), " ", "_"))

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

	content := migrationContent(cfg.DB)

	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		return err
	}

	fmt.Printf("  create  %s\n", filename)
	return nil
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
