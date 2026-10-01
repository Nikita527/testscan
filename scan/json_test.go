package scan_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nikita527/testscan/scan"
)

func TestWriteJSON(t *testing.T) {
	findings := []scan.Finding{{
		File: "t.py", Line: 1, Rule: "empty-test", Severity: "error", Message: "empty",
	}}
	withCatalog(t, map[string]scan.PrecisionInfo{"empty-test": scan.Measured(30, 30)})
	score := scan.CalculateScoreWithTests(findings, 3, 200)
	var buf bytes.Buffer
	if err := scan.WriteJSON(&buf, findings, score); err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	{
		var sum map[string]any
		if err := json.Unmarshal(raw["summary"], &sum); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{
			"actionable_count", "actionable_per_100_tests", "provisional_count", "shown_count",
			"tests", "confirmed_count", "confirmed_density", "health_score", "grade", "grade_deprecated",
		} {
			if _, ok := sum[key]; !ok {
				t.Errorf("summary missing %q", key)
			}
		}
	}
	var report scan.JSONReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Summary.HealthScore != score.Value || report.Summary.Grade != score.Grade {
		t.Fatalf("summary mismatch: %+v vs score %+v", report.Summary, score)
	}
	if report.Summary.Files != 3 || report.Summary.Errors != 1 {
		t.Fatalf("unexpected summary: %+v", report.Summary)
	}
	if !report.Summary.GradeDeprecated {
		t.Fatal("want grade_deprecated true")
	}
	sum := report.Summary
	if sum.ActionableCount != 1 || sum.ActionablePer100 != 0.5 || sum.ProvisionalCount != 0 ||
		sum.ShownCount != 1 || sum.Tests != 200 {
		t.Fatalf("actionable summary: %+v", sum)
	}
	if sum.ConfirmedCount != sum.ActionableCount || sum.ConfirmedDensity != sum.ActionablePer100 {
		t.Fatalf("deprecated confirmed_* must mirror actionable: %+v", sum)
	}
	f0 := report.Findings[0]
	if f0.Tier != scan.TierActionable || f0.Precision == nil || f0.Precision.Source != "measured" ||
		f0.Precision.N != 30 || f0.Precision.Value != 1 {
		t.Fatalf("finding tier/precision: %+v %+v", f0, f0.Precision)
	}
	if len(report.Findings) != 1 || report.Findings[0].Rule != "empty-test" {
		t.Fatalf("findings: %+v", report.Findings)
	}
}

func TestWriteJSON_NilFindings(t *testing.T) {
	var buf bytes.Buffer
	if err := scan.WriteJSON(&buf, nil, scan.CalculateScore(nil, 0)); err != nil {
		t.Fatal(err)
	}
	var report scan.JSONReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Findings == nil {
		t.Fatal("want empty findings array, not null")
	}
	if report.Summary.HealthScore != 100 || report.Summary.Grade != "A" {
		t.Fatalf("want perfect score, got %+v", report.Summary)
	}
}

func TestLoadBaseline_Wrapper(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.json")
	body := `{
  "summary": {"health_score": 90, "grade": "A", "errors": 0, "warnings": 0, "notes": 0, "files": 1},
  "findings": [{"file": "a.py", "line": 1, "rule": "empty-test", "severity": "error", "message": "m", "fingerprint": "fp1"}]
}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := scan.LoadBaseline(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Fingerprint != "fp1" {
		t.Fatalf("got %+v", got)
	}
}

func TestLoadBaseline_LegacyArray(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.json")
	body := `[{"file": "a.py", "line": 1, "rule": "empty-test", "message": "m"}]`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := scan.LoadBaseline(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].File != "a.py" {
		t.Fatalf("got %+v", got)
	}
}

func TestWriteJSON_HiddenCount(t *testing.T) {
	score := scan.CalculateScore(nil, 1)
	score.HiddenCount = 9
	var buf bytes.Buffer
	if err := scan.WriteJSON(&buf, nil, score); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"hidden_count": 9`) {
		t.Fatalf("want hidden_count in summary: %s", buf.String())
	}
}

func TestWriteAgent_HiddenAndParse(t *testing.T) {
	score := scan.CalculateScoreWithTests(nil, 1, 5)
	score.HiddenCount = 3
	score.ParseSkipped = 2
	var buf bytes.Buffer
	if err := scan.WriteAgent(&buf, nil, score); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"3 hidden (use --all)", "2 files not analyzed (parse error)"} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("want %q in %q", want, buf.String())
		}
	}
}
