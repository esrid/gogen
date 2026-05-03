package cmd

import (
	"fmt"
	"os"

	"github.com/esrid/gogen/internal/config"
	"github.com/esrid/gogen/internal/generator"
	"github.com/esrid/gogen/internal/scaffold"
)

func autoWireScaffold(data *scaffold.Data, cfg *config.GogenYAML) {
	if flagDryRun {
		return
	}
	if err := regenerateWireGen(cfg); err != nil {
		fmt.Printf("  hint    update wire_gen.go manually (%v)\n", err)
	}
}

func reWireAllScaffolds(cfg *config.GogenYAML) {
	if err := regenerateWireGen(cfg); err != nil {
		fmt.Printf("  hint    update wire_gen.go manually (%v)\n", err)
	}
}

func removeWireScaffold(data *scaffold.Data, cfg *config.GogenYAML) {
	if flagDryRun {
		return
	}
	if err := regenerateWireGen(cfg); err != nil {
		fmt.Printf("  hint    update wire_gen.go manually (%v)\n", err)
	}
}

func regenerateWireGen(cfg *config.GogenYAML) error {
	if flagDryRun {
		return nil
	}
	path := "bootstrap/wire_gen.go"
	if err := os.WriteFile(path, generator.WireGenContent(cfg), 0644); err != nil {
		return err
	}
	fmt.Printf("  wire    %s\n", path)
	return nil
}
