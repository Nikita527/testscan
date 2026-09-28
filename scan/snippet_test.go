package scan_test

import (
	"strings"
	"testing"

	"github.com/Nikita527/testscan/scan"
)

func TestContextSnippet(t *testing.T) {
	src := []byte("line1\nline2\nline3\nline4\nline5\nline6\nline7\n")
	got := scan.ContextSnippet(src, 4, 2)
	for _, want := range []string{"line2", "line3", "line4", "line5", "line6"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if !strings.Contains(got, ">") {
		t.Fatalf("want highlight marker in:\n%s", got)
	}
	if strings.Contains(got, "line1") {
		t.Fatalf("line1 should be outside radius:\n%s", got)
	}
}

func TestAssignFingerprints_FillsSnippet(t *testing.T) {
	content := []byte("def test_x():\n    pass\n    assert False\n")
	findings := []scan.Finding{{
		File: "t.py", Line: 2, Rule: "empty-test", Severity: "error", Message: "empty",
	}}
	scan.AssignFingerprints(findings, map[string][]byte{"t.py": content}, "")
	if findings[0].Fingerprint == "" {
		t.Fatal("expected fingerprint")
	}
	if findings[0].Snippet == "" {
		t.Fatal("expected snippet")
	}
	if !strings.Contains(findings[0].Snippet, "pass") {
		t.Fatalf("snippet=%q", findings[0].Snippet)
	}
}
