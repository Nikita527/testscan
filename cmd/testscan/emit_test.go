package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nikita527/testscan/scan"
)

func TestWriteFindingsToFile(t *testing.T) {
	stubOpenReport(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "report.html")
	findings := []scan.Finding{{
		File: "t.py", Line: 1, Rule: "empty-test", Severity: "error", Message: "empty",
	}}

	if err := writeFindingsToFile(path, findings, 1, "html"); err != nil {
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

func TestEmitFindings_OutputAndOpen(t *testing.T) {
	stubOpenReport(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "out.html")
	if err := emitFindings(nil, 0, "html", path, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("want output file: %v", err)
	}
}

func TestEmitFindings_HTMLToStdoutByDefault(t *testing.T) {
	// html without -o/--open goes to stdout (no Windows auto-write)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	done := make(chan error, 1)
	go func() {
		done <- emitFindings(nil, 0, "html", "", false)
		_ = w.Close()
	}()
	var buf strings.Builder
	tmp := make([]byte, 4096)
	for {
		n, readErr := r.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
		if readErr != nil {
			break
		}
	}
	os.Stdout = old
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "<!DOCTYPE html>") {
		t.Fatalf("want HTML on stdout, got %q", out[:min(120, len(out))])
	}
}

func stubOpenReport(t *testing.T) {
	t.Helper()
	prev := openReport
	openReport = func(string) error { return nil }
	t.Cleanup(func() { openReport = prev })
}
