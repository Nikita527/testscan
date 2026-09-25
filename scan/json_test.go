package scan_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Nikita527/testscan/scan"
)

func TestWriteJSON(t *testing.T) {
	findings := []scan.Finding{{
		File: "t.py", Line: 1, Rule: "empty-test", Severity: "error", Message: "empty",
	}}
	score := scan.CalculateScore(findings, 3)
	var buf bytes.Buffer
	if err := scan.WriteJSON(&buf, findings, score); err != nil {
		t.Fatal(err)
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
