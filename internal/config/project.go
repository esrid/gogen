package config

import (
	"fmt"
	"strings"
)

type ProjectConfig struct {
	ProjectName string
	ModulePath  string
	DB          string // "sqlite" | "postgres"
	RenderMode  string // "ssr" | "api" | "both"
	Auth        bool
	AuthSet     bool
	Year        int
}

func (p *ProjectConfig) IsSQLite() bool   { return p.DB == "sqlite" }
func (p *ProjectConfig) IsPostgres() bool  { return p.DB == "postgres" }
func (p *ProjectConfig) IsSSR() bool       { return p.RenderMode == "ssr" || p.RenderMode == "both" }
func (p *ProjectConfig) IsAPI() bool       { return p.RenderMode == "api" || p.RenderMode == "both" }

func (p *ProjectConfig) Validate() error {
	if strings.TrimSpace(p.ProjectName) == "" {
		return fmt.Errorf("project name required")
	}
	if strings.TrimSpace(p.ModulePath) == "" {
		return fmt.Errorf("module path required")
	}
	if p.DB != "sqlite" && p.DB != "postgres" {
		return fmt.Errorf("db must be sqlite or postgres, got %q", p.DB)
	}
	if p.RenderMode != "ssr" && p.RenderMode != "api" && p.RenderMode != "both" {
		return fmt.Errorf("render must be ssr, api, or both, got %q", p.RenderMode)
	}
	return nil
}

type ScaffoldMeta struct {
	Fields    []string `yaml:"fields"`
	Protected bool     `yaml:"protected,omitempty"`
}

type GogenYAML struct {
	Module     string                    `yaml:"module"`
	DB         string                    `yaml:"db"`
	RenderMode string                    `yaml:"render_mode"`
	Auth       bool                      `yaml:"auth"`
	Scaffolds  map[string]*ScaffoldMeta  `yaml:"scaffolds,omitempty"`
}
