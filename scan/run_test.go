package scan_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Nikita527/testscan/scan"
)

type stubRule struct{}

func (stubRule) ID() string { return "stub" }

func (stubRule) Check(file scan.File) []scan.Finding {
	return []scan.Finding{{
		File:    file.Path,
		Rule:    "stub",
		Message: "ok",
	}}
}

func TestRun(t *testing.T) {
	findings, err := scan.Run(context.Background(), []string{"testdata"}, scan.Options{
		Rules: []scan.Rule{stubRule{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	// 2 файла в testdata × 1 правило
	if len(findings) != 2 {
		t.Fatalf("got %d findings, want 2", len(findings))
	}
}

func TestRun_NoRules(t *testing.T) {
	_, err := scan.Run(context.Background(), []string{"testdata"}, scan.Options{})
	if !errors.Is(err, scan.ErrRulesRequired) {
		t.Fatalf("got %v, want ErrRulesRequired", err)
	}
}
