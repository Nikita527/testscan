package main

import (
	"testing"

	"github.com/Nikita527/testscan/internal/config"
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
		wantWorkers  int
		wantWorkersS bool
		wantFailOnS  bool
		wantErr      bool
	}{
		{
			name:        "m1_flags",
			argv:        []string{"testdata", "--format", "json", "--fail-on", "never"},
			wantRoots:   []string{"testdata"},
			wantFormat:  "json",
			wantFailOn:  "never",
			wantFailOnS: true,
		},
		{
			name:       "format_sarif",
			argv:       []string{"path", "--format", "sarif"},
			wantRoots:  []string{"path"},
			wantFormat: "sarif",
			wantFailOn: "error",
		},
		{
			name:       "format_html",
			argv:       []string{"path", "--format", "html"},
			wantRoots:  []string{"path"},
			wantFormat: "html",
			wantFailOn: "error",
		},
		{
			name:    "format_invalid",
			argv:    []string{"--format", "xml"},
			wantErr: true,
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
			name:         "workers",
			argv:         []string{"path", "--workers", "4"},
			wantRoots:    []string{"path"},
			wantFormat:   "text",
			wantFailOn:   "error",
			wantWorkers:  4,
			wantWorkersS: true,
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
			if got.workers != tc.wantWorkers || got.workersSet != tc.wantWorkersS {
				t.Fatalf("workers=%d set=%v, want %d set=%v", got.workers, got.workersSet, tc.wantWorkers, tc.wantWorkersS)
			}
			if got.failOnSet != tc.wantFailOnS {
				t.Fatalf("failOnSet=%v, want %v", got.failOnSet, tc.wantFailOnS)
			}
		})
	}
}

func TestApplyConfig(t *testing.T) {
	t.Run("cli_overrides_fail_on_and_workers", func(t *testing.T) {
		args := cliArgs{failOn: "never", failOnSet: true, workers: 8, workersSet: true, roots: []string{"cli"}}
		cfg := config.Config{FailOn: "warning", Workers: 2, Paths: []string{"cfg"}, Disable: []string{"todo-test"}}
		got := applyConfig(args, cfg)
		if got.failOn != "never" || got.workers != 8 {
			t.Fatalf("got failOn=%s workers=%d", got.failOn, got.workers)
		}
		if !strSliceEq(got.roots, []string{"cli"}) {
			t.Fatalf("roots=%v", got.roots)
		}
		if !strSliceEq(got.disable, []string{"todo-test"}) {
			t.Fatalf("disable=%v", got.disable)
		}
	})

	t.Run("config_fills_defaults", func(t *testing.T) {
		args := cliArgs{failOn: "error", format: "text"}
		cfg := config.Config{
			FailOn:  "warning",
			Workers: 3,
			Paths:   []string{"tests"},
			Disable: []string{"no-assert"},
		}
		got := applyConfig(args, cfg)
		if got.failOn != "warning" || got.workers != 3 {
			t.Fatalf("got failOn=%s workers=%d", got.failOn, got.workers)
		}
		if !strSliceEq(got.roots, []string{"tests"}) {
			t.Fatalf("roots=%v", got.roots)
		}
		if !strSliceEq(got.disable, []string{"no-assert"}) {
			t.Fatalf("disable=%v", got.disable)
		}
	})

	t.Run("disable_union", func(t *testing.T) {
		args := cliArgs{failOn: "error", disable: []string{"empty-test"}, roots: []string{"."}}
		cfg := config.Config{Disable: []string{"no-assert"}}
		got := applyConfig(args, cfg)
		if !strSliceEq(got.disable, []string{"no-assert", "empty-test"}) {
			t.Fatalf("disable=%v", got.disable)
		}
	})

	t.Run("no_config_roots_default_dot", func(t *testing.T) {
		got := applyConfig(cliArgs{failOn: "error"}, config.Config{})
		if !strSliceEq(got.roots, []string{"."}) {
			t.Fatalf("roots=%v", got.roots)
		}
	})
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
