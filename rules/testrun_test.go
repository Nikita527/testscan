package rules_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

// sharedBatch is one long-lived Python AST helper for the whole package.
var sharedBatch *parse.Batch

func TestMain(m *testing.M) {
	b, err := parse.StartBatch(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse.StartBatch: %v\n", err)
		os.Exit(1)
	}
	sharedBatch = b
	code := m.Run()
	_ = sharedBatch.Close()
	os.Exit(code)
}

// testRun runs a scan with the shared Batch parser (Workers: 1).
func testRun(t *testing.T, roots []string, rules ...scan.Rule) (scan.Result, error) {
	t.Helper()
	return testRunOpts(t, roots, scan.Options{Rules: rules})
}

// testRunOpts is like testRun but accepts full Options (PathRoot, CoveragePath, …).
func testRunOpts(t *testing.T, roots []string, opts scan.Options) (scan.Result, error) {
	t.Helper()
	opts.Parser = parse.BatchParser{Batch: sharedBatch}
	if opts.Workers <= 0 {
		opts.Workers = 1
	}
	return scan.Run(context.Background(), roots, opts)
}
