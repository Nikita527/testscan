package scan_test

import (
	"strings"
	"testing"

	"github.com/Nikita527/testscan/scan"
)

func TestCollapseNearDuplicatesForText(t *testing.T) {
	in := []scan.Finding{
		{File: "a.py", Line: 1, Rule: "empty-test", Severity: "error", Message: "e"},
		{File: "a.py", Line: 10, Rule: "near-duplicate-test", Severity: "note", Message: "pair1"},
		{File: "a.py", Line: 20, Rule: "near-duplicate-test", Severity: "note", Message: "pair2"},
		{File: "a.py", Line: 30, Rule: "near-duplicate-test", Severity: "note", Message: "pair3"},
		{File: "b.py", Line: 5, Rule: "near-duplicate-test", Severity: "note", Message: "only"},
		{File: "c.py", Line: 1, Rule: "todo-test", Severity: "warning", Message: "t"},
	}
	got := scan.CollapseNearDuplicatesForText(in)
	if len(got) != 4 { // empty + a summary + b single + todo
		t.Fatalf("len=%d, want 4: %+v", len(got), got)
	}
	if got[1].Rule != "near-duplicate-test" || !strings.Contains(got[1].Message, "3 near-duplicate") {
		t.Fatalf("want per-file summary for a.py, got %+v", got[1])
	}
	if got[2].Message != "only" {
		t.Fatalf("single near-dup must keep specific message, got %+v", got[2])
	}
}
