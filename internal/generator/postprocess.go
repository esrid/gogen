package generator

import (
	"fmt"
	"os"
	"os/exec"
)

func PostProcess(outDir string) error {
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
