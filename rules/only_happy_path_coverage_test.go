package rules_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/rules"
	"github.com/Nikita527/testscan/scan"
)

func TestOnlyHappyPath_CoverageMode(t *testing.T) {
	root, err := filepath.Abs("testdata/only_happy_path_coverage")
	if err != nil {
		t.Fatal(err)
	}
	cov := filepath.Join(root, "coverage.json")

	res, err := scan.Run(context.Background(), []string{filepath.Join(root, "tests")}, scan.Options{
		Rules: []scan.Rule{rules.NewOnlyHappyPathOpts(rules.OnlyHappyPathOpts{
			Mode:         "coverage",
			CoveragePath: cov,
		})},
		PathRoot:     root,
		CoveragePath: cov,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) < 2 {
		t.Fatalf("got %d findings, want at least raise+except: %v", len(res.Findings), res.Findings)
	}
	var sawRaise, sawExcept bool
	for _, f := range res.Findings {
		if f.Rule != "only-happy-path" {
			t.Errorf("unexpected rule %q", f.Rule)
		}
		if f.Severity != "note" {
			t.Errorf("severity=%q, want note", f.Severity)
		}
		if filepath.Base(f.File) != "app.py" {
			t.Errorf("file=%s, want app.py", f.File)
		}
		switch f.Line {
		case 3:
			sawRaise = true
		case 10:
			sawExcept = true
		}
	}
	if !sawRaise || !sawExcept {
		t.Fatalf("want raise(line 3) and except(line 10); findings=%v", res.Findings)
	}
}

func TestOnlyHappyPath_CoverageModeClean(t *testing.T) {
	root, err := filepath.Abs("testdata/only_happy_path_coverage")
	if err != nil {
		t.Fatal(err)
	}
	cov := filepath.Join(root, "coverage_clean.json")

	res, err := scan.Run(context.Background(), []string{filepath.Join(root, "tests")}, scan.Options{
		Rules: []scan.Rule{rules.NewOnlyHappyPathOpts(rules.OnlyHappyPathOpts{
			Mode:         "coverage",
			CoveragePath: cov,
		})},
		PathRoot:     root,
		CoveragePath: cov,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 0 {
		t.Fatalf("got %d findings, want 0: %v", len(res.Findings), res.Findings)
	}
}

func TestOnlyHappyPath_CoverageMissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-coverage.json")
	res, err := scan.Run(context.Background(), []string{"testdata/only_happy_path/hit"}, scan.Options{
		Rules: []scan.Rule{rules.NewOnlyHappyPathOpts(rules.OnlyHappyPathOpts{
			Mode:         "coverage",
			CoveragePath: missing,
		})},
		CoveragePath: missing,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("got %d findings, want 1 note about missing file: %v", len(res.Findings), res.Findings)
	}
	if res.Findings[0].Rule != "only-happy-path" {
		t.Fatalf("rule=%q", res.Findings[0].Rule)
	}
}

func TestEnableOnlyHappyPathCoverage(t *testing.T) {
	root, err := filepath.Abs("testdata/only_happy_path_coverage")
	if err != nil {
		t.Fatal(err)
	}
	cov := filepath.Join(root, "coverage_clean.json")
	got := rules.EnableOnlyHappyPathCoverage([]scan.Rule{rules.NewOnlyHappyPathMin(5)}, cov)
	res, err := scan.Run(context.Background(), []string{filepath.Join(root, "tests")}, scan.Options{
		Rules:        got,
		PathRoot:     root,
		CoveragePath: cov,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 0 {
		t.Fatalf("got %v", res.Findings)
	}
}

func TestOnlyHappyPath_CustomNegativeNames(t *testing.T) {
	model := parse.Model{Tests: []parse.TestFunc{
		{Name: "test_a", QualName: "test_a", Lineno: 1, HasAssert: true},
		{Name: "test_b", QualName: "test_b", Lineno: 3, HasAssert: true},
		{Name: "test_c", QualName: "test_c", Lineno: 5, HasAssert: true},
		{Name: "test_forbidden_thing", QualName: "test_forbidden_thing", Lineno: 7, HasAssert: true},
	}}
	file := scan.File{
		Path:    "t.py",
		Content: []byte("def test_forbidden_thing():\n    assert True\n"),
		ModelOK: true,
		Model:   model,
	}

	hit := rules.NewOnlyHappyPathOpts(rules.OnlyHappyPathOpts{
		NegativeNames: []string{"invalid"},
	})
	if fs := hit.Check(file); len(fs) != 1 {
		t.Fatalf("custom names without forbidden: got %d, want 1", len(fs))
	}

	clean := rules.NewOnlyHappyPathOpts(rules.OnlyHappyPathOpts{
		NegativeNames: []string{"forbidden"},
	})
	if fs := clean.Check(file); len(fs) != 0 {
		t.Fatalf("custom names with forbidden: got %d, want 0", len(fs))
	}
}
