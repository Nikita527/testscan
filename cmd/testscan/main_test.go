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
	cases := []struct {
		name         string
		argv         []string
		wantRoots    []string
		wantFormat   string
		wantFailOn   string
		wantOnly     []string
		wantDisable  []string
		wantBaseline string
		wantErr      bool
	}{
		{
			name:       "m1_flags",
			argv:       []string{"testdata", "--format", "json", "--fail-on", "never"},
			wantRoots:  []string{"testdata"},
			wantFormat: "json",
			wantFailOn: "never",
		},
		{
			name:       "rule_only",
			argv:       []string{"path", "--rule", "assert-equals-same"},
			wantRoots:  []string{"path"},
			wantFormat: "text",
			wantFailOn: "error",
			wantOnly:   []string{"assert-equals-same"},
		},
		{
			name:        "disable_repeat",
			argv:        []string{"--disable", "no-assert", "--disable", "empty-test", "."},
			wantRoots:   []string{"."},
			wantFormat:  "text",
			wantFailOn:  "error",
			wantDisable: []string{"no-assert", "empty-test"},
		},
		{
			name:         "baseline",
			argv:         []string{"path", "--baseline", "base.json"},
			wantRoots:    []string{"path"},
			wantFormat:   "text",
			wantFailOn:   "error",
			wantBaseline: "base.json",
		},
		{
			// конфликт ловит rules.Select, не parseArgs
			name:        "conflict_passed_to_select",
			argv:        []string{"--rule", "todo-test", "--disable", "todo-test"},
			wantFormat:  "text",
			wantFailOn:  "error",
			wantOnly:    []string{"todo-test"},
			wantDisable: []string{"todo-test"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseArgs(tc.argv)
			if tc.wantErr {
				if err == nil {
					t.Fatal("want error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !strSliceEq(got.roots, tc.wantRoots) {
				t.Fatalf("roots=%v, want %v", got.roots, tc.wantRoots)
			}
			if got.format != tc.wantFormat || got.failOn != tc.wantFailOn {
				t.Fatalf("format=%s failOn=%s", got.format, got.failOn)
			}
			if !strSliceEq(got.only, tc.wantOnly) {
				t.Fatalf("only=%v, want %v", got.only, tc.wantOnly)
			}
			if !strSliceEq(got.disable, tc.wantDisable) {
				t.Fatalf("disable=%v, want %v", got.disable, tc.wantDisable)
			}
			if got.baseline != tc.wantBaseline {
				t.Fatalf("baseline=%q, want %q", got.baseline, tc.wantBaseline)
			}
		})
	}
}

func strSliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
