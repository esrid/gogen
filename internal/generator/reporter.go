package generator

import (
	"fmt"

	"github.com/fatih/color"
)

type Reporter struct {
	dryRun bool
}

func NewReporter(dryRun bool) *Reporter {
	return &Reporter{dryRun: dryRun}
}

var (
	green  = color.New(color.FgGreen).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
	red    = color.New(color.FgRed).SprintFunc()
	cyan   = color.New(color.FgCyan).SprintFunc()
)

func (r *Reporter) Created(path string) {
	fmt.Printf("  %s  %s\n", green("create"), path)
}

func (r *Reporter) Skipped(path string) {
	fmt.Printf("    %s  %s\n", yellow("skip"), path)
}

func (r *Reporter) Conflict(path string) {
	fmt.Printf("%s  %s\n", red("conflict"), path)
}

func (r *Reporter) DryRun(path string) {
	fmt.Printf("  %s  %s\n", cyan("create"), path)
}
