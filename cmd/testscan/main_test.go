package main

import (
	"testing"

	"github.com/Nikita527/testscan/scan"
)

func TestExitCode(t *testing.T) {
	errFinding := scan.Finding{Severity: "error"}
	warnFinding := scan.Finding{Severity: "warning"}

	cases := []struct {
		name     string
		findings []scan.Finding
		failOn   string
		want     int
	}{
		{"never_with_error", []scan.Finding{errFinding}, "never", 0},
		{"error_with_error", []scan.Finding{errFinding}, "error", 1},
		{"error_with_warning_only", []scan.Finding{warnFinding}, "error", 0},
		{"warning_with_warning", []scan.Finding{warnFinding}, "warning", 1},
		{"warning_with_error", []scan.Finding{errFinding}, "warning", 1},
		{"empty_error", nil, "error", 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := exitCode(tc.findings, tc.failOn)
			if got != tc.want {
				t.Fatalf("exitCode=%d, want %d", got, tc.want)
			}
		})
	}
}

func TestParseArgs(t *testing.T) {
	roots, format, failOn, err := parseArgs([]string{"testdata", "--format", "json", "--fail-on", "never"})
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 || roots[0] != "testdata" {
		t.Fatalf("roots=%v", roots)
	}
	if format != "json" || failOn != "never" {
		t.Fatalf("format=%s failOn=%s", format, failOn)
	}
}
