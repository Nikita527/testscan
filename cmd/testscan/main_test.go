package main

import (
	"strings"
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
		name          string
		argv          []string
		wantRoots     []string
		wantFormat    string
		wantFailOn    string
		wantOnly      []string
		wantEnable    []string
		wantDisable   []string
		wantBaseline  string
		wantCompare   string
		wantShowGrade bool
		wantCoverage  string
		wantDiff      string
		wantFocus     bool
		wantShowLP    bool
		wantWorkers   int
		wantWorkersS  bool
		wantFailOnS   bool
		wantOutput    string
		wantOpen      bool
		wantErr       bool
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
			name:         "coverage",
			argv:         []string{"path", "--coverage", "coverage.json"},
			wantRoots:    []string{"path"},
			wantFormat:   "text",
			wantFailOn:   "error",
			wantCoverage: "coverage.json",
		},
		{
			name:       "diff",
			argv:       []string{"path", "--diff", "origin/main"},
			wantRoots:  []string{"path"},
			wantFormat: "text",
			wantFailOn: "error",
			wantDiff:   "origin/main",
		},
		{
			name:       "focus",
			argv:       []string{"path", "--focus", "--diff", "origin/main"},
			wantRoots:  []string{"path"},
			wantFormat: "text",
			wantFailOn: "error",
			wantDiff:   "origin/main",
			wantFocus:  true,
		},
		{
			name:       "enable_and_show_low_precision",
			argv:       []string{"path", "--enable", "error-contract-assert", "--show-low-precision"},
			wantRoots:  []string{"path"},
			wantFormat: "text",
			wantFailOn: "error",
			wantEnable: []string{"error-contract-assert"},
			wantShowLP: true,
		},
		{
			name:          "compare_and_show_grade",
			argv:          []string{"path", "--compare", "prev.json", "--show-grade"},
			wantRoots:     []string{"path"},
			wantFormat:    "text",
			wantFailOn:    "error",
			wantCompare:   "prev.json",
			wantShowGrade: true,
		},
		{
			name:       "format_codequality",
			argv:       []string{"path", "--format", "codequality"},
			wantRoots:  []string{"path"},
			wantFormat: "codequality",
			wantFailOn: "error",
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
			name:       "output_and_open",
			argv:       []string{"path", "--format", "html", "-o", "out.html", "--open"},
			wantRoots:  []string{"path"},
			wantFormat: "html",
			wantFailOn: "error",
			wantOutput: "out.html",
			wantOpen:   true,
		},
		{
			// conflict is caught by rules.Select, not parseArgs
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
			if !strSliceEq(got.enable, tc.wantEnable) {
				t.Fatalf("enable=%v, want %v", got.enable, tc.wantEnable)
			}
			if !strSliceEq(got.disable, tc.wantDisable) {
				t.Fatalf("disable=%v, want %v", got.disable, tc.wantDisable)
			}
			if got.baseline != tc.wantBaseline {
				t.Fatalf("baseline=%q, want %q", got.baseline, tc.wantBaseline)
			}
			if got.compare != tc.wantCompare {
				t.Fatalf("compare=%q, want %q", got.compare, tc.wantCompare)
			}
			if got.showGrade != tc.wantShowGrade {
				t.Fatalf("showGrade=%v, want %v", got.showGrade, tc.wantShowGrade)
			}
			if got.coverage != tc.wantCoverage {
				t.Fatalf("coverage=%q, want %q", got.coverage, tc.wantCoverage)
			}
			if got.diff != tc.wantDiff {
				t.Fatalf("diff=%q, want %q", got.diff, tc.wantDiff)
			}
			if got.focus != tc.wantFocus {
				t.Fatalf("focus=%v, want %v", got.focus, tc.wantFocus)
			}
			if got.showLowPrecision != tc.wantShowLP {
				t.Fatalf("showLowPrecision=%v, want %v", got.showLowPrecision, tc.wantShowLP)
			}
			if got.workers != tc.wantWorkers || got.workersSet != tc.wantWorkersS {
				t.Fatalf("workers=%d set=%v, want %d set=%v", got.workers, got.workersSet, tc.wantWorkers, tc.wantWorkersS)
			}
			if got.failOnSet != tc.wantFailOnS {
				t.Fatalf("failOnSet=%v, want %v", got.failOnSet, tc.wantFailOnS)
			}
			if got.output != tc.wantOutput || got.open != tc.wantOpen {
				t.Fatalf("output=%q open=%v, want %q %v", got.output, got.open, tc.wantOutput, tc.wantOpen)
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

	t.Run("enable_union_and_show_low_precision", func(t *testing.T) {
		args := cliArgs{failOn: "error", enable: []string{"rbac-mutation-guard"}, roots: []string{"."}}
		cfg := config.Config{
			Enable:           []string{"error-contract-assert"},
			ShowLowPrecision: true,
		}
		got := applyConfig(args, cfg)
		if !strSliceEq(got.enable, []string{"error-contract-assert", "rbac-mutation-guard"}) {
			t.Fatalf("enable=%v", got.enable)
		}
		if !got.showLowPrecision {
			t.Fatal("want showLowPrecision from config")
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

func TestWriteFindings_TextAndJSONScore(t *testing.T) {
	findings := []scan.Finding{{
		File: "t.py", Line: 1, Rule: "empty-test", Severity: "error", Message: "empty",
	}}
	score := scan.CalculateScore(findings, 5)
	var textBuf strings.Builder
	if err := writeFindings(&textBuf, findings, score, "text"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(textBuf.String(), "Confirmed:") {
		t.Fatalf("text missing confirmed: %q", textBuf.String())
	}
	if strings.Contains(textBuf.String(), "Health Score:") {
		t.Fatalf("text must not show Health Score without ShowGrade: %q", textBuf.String())
	}

	score.ShowGrade = true
	var textGrade strings.Builder
	if err := writeFindings(&textGrade, findings, score, "text"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(textGrade.String(), "Health Score:") || !strings.Contains(textGrade.String(), "[deprecated]") {
		t.Fatalf("want deprecated Health Score: %q", textGrade.String())
	}

	var jsonBuf strings.Builder
	if err := writeFindings(&jsonBuf, findings, score, "json"); err != nil {
		t.Fatal(err)
	}
	out := jsonBuf.String()
	for _, want := range []string{`"summary"`, `"health_score"`, `"confirmed_count"`, `"confirmed_density"`, `"grade_deprecated"`, `"findings"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("json missing %s: %s", want, out)
		}
	}
}
