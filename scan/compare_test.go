package scan_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Nikita527/testscan/scan"
)

func TestLoadComparePoint_BareFindings(t *testing.T) {
	withCatalog(t, map[string]scan.PrecisionInfo{"empty-test": scan.Measured(24, 25)})
	dir := t.TempDir()
	path := filepath.Join(dir, "prev.json")
	body := `[
  {"file":"a.py","line":1,"rule":"empty-test","severity":"error","message":"m"},
  {"file":"b.py","line":2,"rule":"weak-assert","severity":"note","message":"w"},
  {"file":"a.py","line":3,"rule":"parse-error","severity":"note","message":"p"}
]`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cp, err := scan.LoadComparePoint(path)
	if err != nil {
		t.Fatal(err)
	}
	// empty-test only (actionable); weak-assert is low, parse-error excluded.
	if cp.ActionableCount != 1 {
		t.Fatalf("count=%d, want 1", cp.ActionableCount)
	}
	if cp.ActionableDensity >= 0 || !cp.Legacy {
		t.Fatalf("bare array must be a legacy point with unknown density: %+v", cp)
	}
}

func TestLoadComparePoint_NewSummary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prev.json")
	body := `{
  "summary": {
    "actionable_count": 7, "actionable_per_100_tests": 0.19,
    "confirmed_count": 99, "confirmed_density": 9.9, "tests": 3650, "files": 9
  },
  "findings": [
    {"file":"a.py","line":1,"rule":"empty-test","severity":"error","message":"m"}
  ]
}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cp, err := scan.LoadComparePoint(path)
	if err != nil {
		t.Fatal(err)
	}
	// The summary wins over findings and the deprecated confirmed_* keys.
	if cp.ActionableCount != 7 || cp.ActionableDensity != 0.19 || cp.Legacy {
		t.Fatalf("got %+v", cp)
	}
}

func TestLoadComparePoint_LegacyConfirmedFallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prev.json")
	body := `{
  "summary": {
    "health_score": 90, "grade": "A",
    "confirmed_count": 4, "confirmed_density": 1.25,
    "errors": 0, "warnings": 0, "notes": 0, "files": 10
  },
  "findings": []
}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cp, err := scan.LoadComparePoint(path)
	if err != nil {
		t.Fatal(err)
	}
	if cp.ActionableCount != 4 || cp.ActionableDensity >= 0 || !cp.Legacy {
		t.Fatalf("got %+v", cp)
	}
	// A legacy point is compared on count only (density units differ).
	tr := scan.ComputeTrend(4, 3.0, cp.ActionableCount, cp.ActionableDensity)
	if tr.Direction != "unchanged" || tr.DeltaDensity != 0 {
		t.Fatalf("trend=%+v", tr)
	}
}

func TestLoadComparePoint_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte(`{"summary":`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := scan.LoadComparePoint(path)
	if err == nil {
		t.Fatal("want error")
	}
	if !containsPrefix(err.Error(), "compare:") {
		t.Fatalf("want compare: prefix, got %v", err)
	}
	// Should wrap the JSON syntax error, not a bare-array mismatch.
	if containsPrefix(err.Error(), "compare: json: cannot unmarshal") {
		t.Fatalf("unexpected bare-array error wrapped: %v", err)
	}
}

func TestLoadComparePoint_MissingFile(t *testing.T) {
	_, err := scan.LoadComparePoint(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil {
		t.Fatal("want error")
	}
	if !containsPrefix(err.Error(), "compare:") {
		t.Fatalf("want compare: prefix, got %v", err)
	}
}

func containsPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
