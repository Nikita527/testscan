package scan_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Nikita527/testscan/scan"
)

func TestLoadComparePoint_BareFindings(t *testing.T) {
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
	// empty-test only (weak-assert 0.25, parse-error excluded); 2 unique files
	if cp.ConfirmedCount != 1 {
		t.Fatalf("count=%d, want 1", cp.ConfirmedCount)
	}
	want := scan.ConfirmedDensity(1, 2)
	if cp.ConfirmedDensity != want {
		t.Fatalf("density=%v, want %v", cp.ConfirmedDensity, want)
	}
}

func TestLoadComparePoint_WrapperRecompute(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prev.json")
	body := `{
  "summary": {
    "health_score": 80, "grade": "B",
    "confirmed_count": 99, "confirmed_density": 9.9,
    "errors": 1, "warnings": 0, "notes": 0, "files": 9
  },
  "findings": [
    {"file":"a.py","line":1,"rule":"empty-test","severity":"error","message":"m"},
    {"file":"a.py","line":2,"rule":"empty-test","severity":"error","message":"m2"}
  ]
}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cp, err := scan.LoadComparePoint(path)
	if err != nil {
		t.Fatal(err)
	}
	// Non-empty findings → recompute (ignore stale summary confirmed_*)
	if cp.ConfirmedCount != 2 {
		t.Fatalf("count=%d, want 2", cp.ConfirmedCount)
	}
	want := scan.ConfirmedDensity(2, 9)
	if cp.ConfirmedDensity != want {
		t.Fatalf("density=%v, want %v", cp.ConfirmedDensity, want)
	}
}

func TestLoadComparePoint_WrapperSummaryOnly(t *testing.T) {
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
	if cp.ConfirmedCount != 4 || cp.ConfirmedDensity != 1.25 {
		t.Fatalf("got %+v", cp)
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
