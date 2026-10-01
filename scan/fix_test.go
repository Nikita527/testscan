package scan_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Nikita527/testscan/scan"
)

func fixFindings() []scan.Finding {
	return []scan.Finding{
		{File: "tests/test_a.py", Line: 7, Rule: "sleep-in-test", Severity: "warning",
			Message: "sleep in test", Fix: "remove `time.sleep(...)`; wait on the condition"},
		{File: "tests/test_a.py", Line: 20, Rule: "broad-raises", Severity: "warning",
			Message: "too broad"},
	}
}

func TestApplyDefaultFixes(t *testing.T) {
	fs := fixFindings()
	scan.ApplyDefaultFixes(fs)
	if !strings.Contains(fs[0].Fix, "time.sleep") {
		t.Fatalf("concrete fix overwritten: %q", fs[0].Fix)
	}
	if fs[1].Fix != scan.DefaultFix("broad-raises") || fs[1].Fix == "" {
		t.Fatalf("default not applied: %q", fs[1].Fix)
	}
}

func TestWriteJSON_Fix(t *testing.T) {
	var buf bytes.Buffer
	if err := scan.WriteJSON(&buf, fixFindings()[:1], scan.CalculateScore(nil, 1)); err != nil {
		t.Fatal(err)
	}
	var rep struct {
		Findings []map[string]any `json:"findings"`
	}
	if err := json.Unmarshal(buf.Bytes(), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Findings[0]["fix"] != "remove `time.sleep(...)`; wait on the condition" {
		t.Fatalf("fix=%v", rep.Findings[0]["fix"])
	}
}

func TestWriteSARIF_Fix(t *testing.T) {
	fs := fixFindings()
	scan.ApplyDefaultFixes(fs)
	var buf bytes.Buffer
	if err := scan.WriteSARIF(&buf, fs); err != nil {
		t.Fatal(err)
	}
	var rep struct {
		Runs []struct {
			Tool struct {
				Driver struct {
					Rules []struct {
						ID   string `json:"id"`
						Help struct {
							Text     string `json:"text"`
							Markdown string `json:"markdown"`
						} `json:"help"`
					} `json:"rules"`
				} `json:"driver"`
			} `json:"tool"`
			Results []struct {
				Properties map[string]any `json:"properties"`
				Fixes      []any          `json:"fixes"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(buf.Bytes(), &rep); err != nil {
		t.Fatal(err)
	}
	run := rep.Runs[0]
	if len(run.Tool.Driver.Rules) != 2 {
		t.Fatalf("rules=%d", len(run.Tool.Driver.Rules))
	}
	if r := run.Tool.Driver.Rules[0]; r.ID != "broad-raises" ||
		!strings.HasPrefix(r.Help.Text, "Fix: ") || !strings.HasPrefix(r.Help.Markdown, "**Fix:**") {
		t.Fatalf("rule help: %+v", r)
	}
	if run.Results[0].Properties["fix"] == nil {
		t.Fatalf("result fix missing: %+v", run.Results[0])
	}
	if run.Results[0].Fixes != nil {
		t.Fatalf("must not emit SARIF fixes without artifactChanges")
	}
}

func TestWriteHTML_Fix(t *testing.T) {
	fs := fixFindings()[:1]
	var buf bytes.Buffer
	if err := scan.WriteHTML(&buf, fs, scan.CalculateScore(fs, 1)); err != nil {
		t.Fatal(err)
	}
	want := "<div class=\"fix\">Fix: remove `time.sleep(...)`; wait on the condition</div>"
	if !strings.Contains(buf.String(), want) {
		t.Fatalf("html missing fix line")
	}
}

func TestWriteAgent(t *testing.T) {
	fs := fixFindings()
	scan.ApplyDefaultFixes(fs)
	score := scan.CalculateScoreWithTests(fs, 1, 12)
	var buf bytes.Buffer
	if err := scan.WriteAgent(&buf, fs, score); err != nil {
		t.Fatal(err)
	}
	want := "tests/test_a.py:7 sleep-in-test — sleep in test → remove `time.sleep(...)`; wait on the condition\n" +
		"tests/test_a.py:20 broad-raises — too broad → " + scan.DefaultFix("broad-raises") + "\n" +
		"testscan: 2 findings (2 warning) · 12 tests\n"
	if buf.String() != want {
		t.Fatalf("got:\n%s\nwant:\n%s", buf.String(), want)
	}
}

func TestWriteAgent_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := scan.WriteAgent(&buf, nil, scan.CalculateScoreWithTests(nil, 1, 3)); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "testscan: 0 findings · 3 tests\n" {
		t.Fatalf("got %q", buf.String())
	}
}
