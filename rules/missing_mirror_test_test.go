package rules_test

import (
	"path/filepath"
	"testing"

	"github.com/Nikita527/testscan/rules"
	"github.com/Nikita527/testscan/scan"
)

func TestMissingMirrorTest_HitAndClean(t *testing.T) {
	hitRoot, err := filepath.Abs("testdata/missing_mirror_test/hit")
	if err != nil {
		t.Fatal(err)
	}
	rule := rules.NewMissingMirrorTest(rules.MissingMirrorTestOpts{
		SourceGlob:     "app/**/domain/*.py",
		MirrorTemplate: "tests/{x}/domain/test_{m}.py",
	})
	res, err := testRunOpts(t, []string{hitRoot}, scan.Options{
		Rules:    []scan.Rule{rule},
		PathRoot: hitRoot,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("hit: got %d findings, want 1: %v", len(res.Findings), res.Findings)
	}
	if res.Findings[0].Rule != "missing-mirror-test" {
		t.Fatalf("rule=%q", res.Findings[0].Rule)
	}
	if filepath.Base(res.Findings[0].File) != "pricing.py" {
		t.Fatalf("file=%s, want pricing.py", res.Findings[0].File)
	}

	cleanRoot, err := filepath.Abs("testdata/missing_mirror_test/clean")
	if err != nil {
		t.Fatal(err)
	}
	res, err = testRunOpts(t, []string{cleanRoot}, scan.Options{
		Rules:    []scan.Rule{rule},
		PathRoot: cleanRoot,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 0 {
		t.Fatalf("clean: got %v", res.Findings)
	}
}
