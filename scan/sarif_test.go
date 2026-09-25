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

func TestWriteSARIF(t *testing.T) {
	findings := []scan.Finding{
		{
			File:     `tests\test_a.py`,
			Line:     3,
			Rule:     "empty-test",
			Severity: "error",
			Message:  "empty test body",
		},
		{
			File:     "tests/test_b.py",
			Line:     0,
			Rule:     "todo-test",
			Severity: "warning",
			Message:  "TODO skip",
		},
	}

	var buf bytes.Buffer
	if err := scan.WriteSARIF(&buf, findings); err != nil {
		t.Fatal(err)
	}

	var report map[string]any
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if report["version"] != "2.1.0" {
		t.Fatalf("version=%v", report["version"])
	}
	schema, _ := report["$schema"].(string)
	if !strings.Contains(schema, "sarif-2.1.0") {
		t.Fatalf("$schema=%q", schema)
	}

	runs, _ := report["runs"].([]any)
	if len(runs) != 1 {
		t.Fatalf("runs len=%d", len(runs))
	}
	run, _ := runs[0].(map[string]any)
	results, _ := run["results"].([]any)
	if len(results) != 2 {
		t.Fatalf("results len=%d", len(results))
	}

	r0, _ := results[0].(map[string]any)
	if r0["ruleId"] != "empty-test" || r0["level"] != "error" {
		t.Fatalf("result0=%v", r0)
	}
	locs, _ := r0["locations"].([]any)
	loc0, _ := locs[0].(map[string]any)
	phys, _ := loc0["physicalLocation"].(map[string]any)
	art, _ := phys["artifactLocation"].(map[string]any)
	uri, _ := art["uri"].(string)
	if uri != "tests/test_a.py" {
		t.Fatalf("uri=%q (want forward slashes)", uri)
	}
	region, _ := phys["region"].(map[string]any)
	if int(region["startLine"].(float64)) != 3 {
		t.Fatalf("startLine=%v", region["startLine"])
	}

	r1, _ := results[1].(map[string]any)
	if r1["level"] != "warning" {
		t.Fatalf("result1 level=%v", r1["level"])
	}
	locs1, _ := r1["locations"].([]any)
	loc1, _ := locs1[0].(map[string]any)
	phys1, _ := loc1["physicalLocation"].(map[string]any)
	region1, _ := phys1["region"].(map[string]any)
	if int(region1["startLine"].(float64)) != 1 {
		t.Fatalf("line 0 should clamp to 1, got %v", region1["startLine"])
	}
}

func TestWriteSARIF_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := scan.WriteSARIF(&buf, nil); err != nil {
		t.Fatal(err)
	}
	var report map[string]any
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	runs, _ := report["runs"].([]any)
	run, _ := runs[0].(map[string]any)
	results, _ := run["results"].([]any)
	if len(results) != 0 {
		t.Fatalf("want empty results, got %d", len(results))
	}
}

func TestWriteSARIF_QualNameAndFingerprint(t *testing.T) {
	var buf bytes.Buffer
	if err := scan.WriteSARIF(&buf, []scan.Finding{{
		File:        "tests/test_a.py",
		Line:        4,
		Rule:        "empty-test",
		Severity:    "error",
		Message:     "empty",
		QualName:    "TestFoo.test_bar",
		Fingerprint: "abc123",
	}}); err != nil {
		t.Fatal(err)
	}
	var report map[string]any
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	runs := report["runs"].([]any)
	run := runs[0].(map[string]any)
	results := run["results"].([]any)
	r0 := results[0].(map[string]any)

	pf, _ := r0["partialFingerprints"].(map[string]any)
	if pf["testscan/v1"] != "abc123" {
		t.Fatalf("partialFingerprints=%v", pf)
	}
	if pf["primaryLocationLineHash"] == nil || pf["primaryLocationLineHash"] == "" {
		t.Fatalf("missing primaryLocationLineHash: %v", pf)
	}
	props, _ := r0["properties"].(map[string]any)
	if props["qualName"] != "TestFoo.test_bar" {
		t.Fatalf("properties=%v", props)
	}
}

func TestWriteSARIF_RelativizesAbsPath(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	abs := filepath.Join(cwd, "tests", "test_x.py")
	var buf bytes.Buffer
	if err := scan.WriteSARIF(&buf, []scan.Finding{{
		File:     abs,
		Line:     2,
		Rule:     "empty-test",
		Severity: "error",
		Message:  "empty",
	}}); err != nil {
		t.Fatal(err)
	}
	var report map[string]any
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	runs := report["runs"].([]any)
	run := runs[0].(map[string]any)
	results := run["results"].([]any)
	r0 := results[0].(map[string]any)
	locs := r0["locations"].([]any)
	loc0 := locs[0].(map[string]any)
	phys := loc0["physicalLocation"].(map[string]any)
	art := phys["artifactLocation"].(map[string]any)
	uri, _ := art["uri"].(string)
	if uri != "tests/test_x.py" {
		t.Fatalf("uri=%q, want tests/test_x.py (cwd-relative)", uri)
	}
}
