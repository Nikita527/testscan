package scan_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Nikita527/testscan/scan"
)

func TestWriteHTML(t *testing.T) {
	findings := []scan.Finding{
		{
			File:     "tests/test_a.py",
			Line:     3,
			Rule:     "empty-test",
			Severity: "error",
			Message:  "empty test body",
		},
		{
			File:     "tests/test_b.py",
			Line:     7,
			Rule:     "only-happy-path",
			Severity: "warning",
			Message:  "more than 3 tests",
		},
		{
			File:     "tests/test_a.py",
			Line:     10,
			Rule:     "empty-test",
			Severity: "error",
			Message:  "another empty",
		},
	}

	var buf bytes.Buffer
	if err := scan.WriteHTML(&buf, findings); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	for _, want := range []string{
		"<!DOCTYPE html>",
		"testscan",
		"3 finding(s)",
		"id=\"rule-empty-test\"",
		"id=\"rule-only-happy-path\"",
		"tests/test_a.py",
		"empty test body",
		"data-sev=\"error\"",
		"data-sev=\"warning\"",
		`id="q"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(out, "<script src=") {
		t.Error("report must be self-contained (no external script)")
	}
}

func TestWriteHTML_Escapes(t *testing.T) {
	var buf bytes.Buffer
	err := scan.WriteHTML(&buf, []scan.Finding{{
		File:     `tests/<script>.py`,
		Line:     1,
		Rule:     "no-assert",
		Severity: "error",
		Message:  `x < y & "z"`,
	}})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "<script>.py") {
		t.Fatal("path must be HTML-escaped")
	}
	if !strings.Contains(out, "tests/&lt;script&gt;.py") {
		t.Fatalf("expected escaped path, got snippet missing")
	}
	if !strings.Contains(out, "x &lt; y &amp; &#34;z&#34;") && !strings.Contains(out, `x &lt; y &amp; "z"`) {
		// html.EscapeString uses &#34; for quotes
		if !strings.Contains(out, "x &lt; y &amp;") {
			t.Fatalf("message not escaped: %s", out)
		}
	}
}

func TestWriteHTML_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := scan.WriteHTML(&buf, nil); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "0 finding(s)") {
		t.Fatalf("want zero summary, got: %s", out[:min(200, len(out))])
	}
	if !strings.Contains(out, "No findings") {
		t.Fatal("want empty toc message")
	}
}
