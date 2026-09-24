package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Nikita527/testscan/scan"
)

func TestWriteHTMLFileAndOpen_WritesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.html")
	findings := []scan.Finding{{
		File: "t.py", Line: 1, Rule: "empty-test", Severity: "error", Message: "empty",
	}}

	// On Windows this also tries to open the browser; Start() is async and should not fail
	// for a valid file path. We still verify the file contents.
	if err := writeHTMLFileAndOpen(path, findings); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	if !strings.Contains(out, "<!DOCTYPE html>") || !strings.Contains(out, "empty-test") {
		t.Fatalf("bad report: %s", out[:min(200, len(out))])
	}
}

func TestEmitFindings_HTMLWindowsWritesDefaultName(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only emit path")
	}
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	if err := emitFindings(nil, "html"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(defaultHTMLReport); err != nil {
		t.Fatalf("want %s in cwd: %v", defaultHTMLReport, err)
	}
}
