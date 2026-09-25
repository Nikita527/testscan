package rules_test

import (
	"testing"

	"github.com/Nikita527/testscan/internal/config"
	"github.com/Nikita527/testscan/rules"
	"github.com/Nikita527/testscan/scan"
)

func TestApplyConfig_OnlyHappyPathMode(t *testing.T) {
	selected := []scan.Rule{rules.NewOnlyHappyPath(), rules.NewEmptyTest()}
	got := rules.ApplyConfig(selected, config.Config{
		Rules: map[string]config.RuleConfig{
			"only-happy-path": {
				MinTests:      7,
				Mode:          "coverage",
				Coverage:      "/tmp/cov.json",
				NegativeNames: []string{"bad"},
			},
		},
	})
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].ID() != "only-happy-path" {
		t.Fatalf("id=%q", got[0].ID())
	}
	// Coverage mode: per-file Check is a no-op.
	fs := got[0].Check(scan.File{
		Path:    "t.py",
		Content: []byte("def test_a():\n  assert 1\ndef test_b():\n  assert 2\ndef test_c():\n  assert 3\ndef test_d():\n  assert 4\n"),
		ModelOK: true,
	})
	if len(fs) != 0 {
		t.Fatalf("coverage mode Check should be empty, got %v", fs)
	}
	if got[1].ID() != "empty-test" {
		t.Fatalf("second rule=%q", got[1].ID())
	}
}

func TestCoveragePathFromConfig(t *testing.T) {
	if p := rules.CoveragePathFromConfig(config.Config{}); p != "" {
		t.Fatalf("got %q", p)
	}
	p := rules.CoveragePathFromConfig(config.Config{
		Rules: map[string]config.RuleConfig{
			"only-happy-path": {Coverage: "c.json"},
		},
	})
	if p != "c.json" {
		t.Fatalf("got %q", p)
	}
}
