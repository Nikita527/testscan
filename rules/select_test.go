package rules_test

import (
	"testing"

	"github.com/Nikita527/testscan/rules"
	"github.com/Nikita527/testscan/scan"
)

func TestSelect(t *testing.T) {
	all := rules.Default()

	t.Run("empty_only_returns_all_minus_disable", func(t *testing.T) {
		got, err := rules.Select(all, nil, []string{"no-assert", "empty-test"})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(all)-2 {
			t.Fatalf("got %d rules, want %d", len(got), len(all)-2)
		}
		for _, r := range got {
			if r.ID() == "no-assert" || r.ID() == "empty-test" {
				t.Fatalf("disabled rule still present: %s", r.ID())
			}
		}
	})

	t.Run("only_filters", func(t *testing.T) {
		got, err := rules.Select(all, []string{"assert-equals-same"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].ID() != "assert-equals-same" {
			t.Fatalf("got %v", ids(got))
		}
	})

	t.Run("conflict", func(t *testing.T) {
		_, err := rules.Select(all, []string{"todo-test"}, []string{"todo-test"})
		if err == nil {
			t.Fatal("want conflict error")
		}
	})

	t.Run("unknown_rule", func(t *testing.T) {
		_, err := rules.Select(all, []string{"no-such"}, nil)
		if err == nil {
			t.Fatal("want unknown rule error")
		}
	})

	t.Run("unknown_disable", func(t *testing.T) {
		_, err := rules.Select(all, nil, []string{"no-such"})
		if err == nil {
			t.Fatal("want unknown disable error")
		}
	})
}

func ids(rs []scan.Rule) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.ID()
	}
	return out
}
