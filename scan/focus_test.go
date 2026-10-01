package scan_test

import (
	"testing"

	"github.com/Nikita527/testscan/scan"
)

func TestFilterFocus(t *testing.T) {
	in := []scan.Finding{
		{Rule: "empty-test", Severity: "error"},
		{Rule: "broad-raises", Severity: "warning"},
		{Rule: "name-body-mismatch", Severity: "note"},
		{Rule: "no-assert", Severity: "note"},
		{Rule: "mock-only-assert", Severity: "warning"}, // precision 0 + neverFocus
		{Rule: "parse-error", Severity: "note"},
		{Rule: "assert-true", Severity: "warning"},
		{Rule: "only-happy-path", Severity: "note"},
	}
	got := scan.FilterFocus(in)
	if len(got) != 4 {
		t.Fatalf("len=%d, want 4: %+v", len(got), got)
	}
	// parse-error is exempt from focus (a file that was not analyzed must show).
	want := []string{"empty-test", "broad-raises", "parse-error", "assert-true"}
	for i, id := range want {
		if got[i].Rule != id {
			t.Fatalf("got[%d]=%q, want %q", i, got[i].Rule, id)
		}
	}
}

func TestFilterFocus_SeverityOverrideStillExcluded(t *testing.T) {
	in := []scan.Finding{
		{Rule: "name-body-mismatch", Severity: "warning"}, // config bump
		{Rule: "overbroad-equality", Severity: "warning"},
		{Rule: "assert-in-emptyable-loop", Severity: "warning"},
		{Rule: "broad-raises", Severity: "warning"},
	}
	got := scan.FilterFocus(in)
	if len(got) != 2 {
		t.Fatalf("len=%d, want 2: %+v", len(got), got)
	}
	if got[0].Rule != "assert-in-emptyable-loop" || got[1].Rule != "broad-raises" {
		t.Fatalf("got %+v", got)
	}
}

func TestFilterFocus_Empty(t *testing.T) {
	if got := scan.FilterFocus(nil); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}
