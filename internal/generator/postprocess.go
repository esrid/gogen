package generator

import (
	"fmt"
	"os"
	"os/exec"
)

// RunTemplGenerate runs `templ generate ./...` in dir. Safe to call in any
// SSR project directory — prints a warning but never returns an error.
func RunTemplGenerate(dir string) {
	fmt.Println("\nRunning templ generate...")
	cmd := exec.Command("templ", "generate", "./...")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("\nWarning: templ generate failed: %v\n", err)
		fmt.Println("Run manually: templ generate ./...")
	}
}

func PostProcess(outDir string, isSSR bool) error {
	// templ generate must run before go mod tidy:
	//   go.mod already contains github.com/a-h/templ so the CLI accepts it,
	//   and the resulting _templ.go files let go mod tidy resolve local packages.
	if isSSR {
		RunTemplGenerate(outDir)
	}

	fmt.Println("\nRunning go mod tidy...")
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = outDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("\nWarning: go mod tidy failed: %v\n", err)
		fmt.Printf("Run manually: cd %s && go mod tidy\n", outDir)
		return nil
	}
	fmt.Printf("\nDone! Project ready at ./%s\n", outDir)
	fmt.Printf("Run: cd %s && go run .\n", outDir)
	return nil
}
