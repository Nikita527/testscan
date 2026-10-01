package rules_test

import (
	"testing"

	"github.com/Nikita527/testscan/rules"
	"github.com/Nikita527/testscan/scan"
)

func TestSelect(t *testing.T) {
	defaults := rules.Default()
	all := rules.All()

	t.Run("empty_only_returns_defaults_minus_disable", func(t *testing.T) {
		got, err := rules.Select(defaults, all, nil, nil, []string{"no-assert", "empty-test"})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(defaults)-2 {
			t.Fatalf("got %d rules, want %d", len(got), len(defaults)-2)
		}
		for _, r := range got {
			if r.ID() == "no-assert" || r.ID() == "empty-test" {
				t.Fatalf("disabled rule still present: %s", r.ID())
			}
		}
	})

	t.Run("only_filters", func(t *testing.T) {
		got, err := rules.Select(defaults, all, []string{"assert-equals-same"}, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].ID() != "assert-equals-same" {
			t.Fatalf("got %v", ids(got))
		}
	})

	t.Run("enable_optional", func(t *testing.T) {
		got, err := rules.Select(defaults, all, nil, []string{"error-contract-assert"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(defaults)+1 {
			t.Fatalf("got %d rules, want %d", len(got), len(defaults)+1)
		}
		found := false
		for _, r := range got {
			if r.ID() == "error-contract-assert" {
				found = true
			}
		}
		if !found {
			t.Fatalf("optional rule missing: %v", ids(got))
		}
	})

	t.Run("enable_disable_conflict", func(t *testing.T) {
		_, err := rules.Select(defaults, all, nil, []string{"error-contract-assert"}, []string{"error-contract-assert"})
		if err == nil {
			t.Fatal("want conflict error")
		}
	})

	t.Run("conflict", func(t *testing.T) {
		_, err := rules.Select(defaults, all, []string{"todo-test"}, nil, []string{"todo-test"})
		if err == nil {
			t.Fatal("want conflict error")
		}
	})

	t.Run("unknown_rule", func(t *testing.T) {
		_, err := rules.Select(defaults, all, []string{"no-such"}, nil, nil)
		if err == nil {
			t.Fatal("want unknown rule error")
		}
	})

	t.Run("unknown_disable", func(t *testing.T) {
		_, err := rules.Select(defaults, all, nil, nil, []string{"no-such"})
		if err == nil {
			t.Fatal("want unknown disable error")
		}
	})

	t.Run("unknown_enable", func(t *testing.T) {
		_, err := rules.Select(defaults, all, nil, []string{"no-such"}, nil)
		if err == nil {
			t.Fatal("want unknown enable error")
		}
	})

	t.Run("only_can_pick_optional", func(t *testing.T) {
		got, err := rules.Select(defaults, all, []string{"missing-mirror-test"}, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].ID() != "missing-mirror-test" {
			t.Fatalf("got %v", ids(got))
		}
	})
}

func TestOptionalAndAll(t *testing.T) {
	opt := rules.Optional()
	if len(opt) != 4 {
		t.Fatalf("Optional=%d, want 4", len(opt))
	}
	all := rules.All()
	if len(all) != len(rules.Default())+len(opt) {
		t.Fatalf("All=%d, want %d", len(all), len(rules.Default())+len(opt))
	}
}

func ids(rs []scan.Rule) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.ID()
	}
	return out
}
