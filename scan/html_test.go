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

	score := scan.CalculateScore(findings, 2)
	var buf bytes.Buffer
	if err := scan.WriteHTML(&buf, findings, score); err != nil {
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
		"score-value",
		"score-caption",
		"signal-caption",
		"Showing trusted warnings first",
		"not a claim that tests are excellent",
		"Health Score",
		`data-grade=`,
		").toLowerCase();",
		`data-sev="note"> other</label>`,
		`data-sev="tool-error"> tool error</label>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(out, `data-sev="note" checked>`) {
		t.Error("note filter must be unchecked by default")
	}
	if strings.Contains(out, `data-sev="tool-error" checked>`) {
		t.Error("tool-error filter must be unchecked by default")
	}
	if strings.Contains(out, "<script src=") {
		t.Error("report must be self-contained (no external script)")
	}
}

func TestWriteHTML_Escapes(t *testing.T) {
	findings := []scan.Finding{{
		File:     `tests/<script>.py`,
		Line:     1,
		Rule:     "no-assert",
		Severity: "error",
		Message:  `x < y & "z"`,
	}}
	var buf bytes.Buffer
	err := scan.WriteHTML(&buf, findings, scan.CalculateScore(findings, 1))
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
	if err := scan.WriteHTML(&buf, nil, scan.CalculateScore(nil, 0)); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "0 finding(s)") {
		t.Fatalf("want zero summary, got: %s", out[:min(200, len(out))])
	}
	if !strings.Contains(out, "No findings") {
		t.Fatal("want empty toc message")
	}
	if !strings.Contains(out, ">100<") && !strings.Contains(out, "score-value\">100") {
		t.Fatalf("want perfect score in hero, got: %s", out[:min(400, len(out))])
	}
}

func TestWriteHTML_SnippetRelatedDedup(t *testing.T) {
	findings := []scan.Finding{
		{
			File:            "tests/test_a.py",
			Line:            3,
			Rule:            "near-duplicate-test",
			Severity:        "note",
			Message:         "test body is nearly identical to test_a:1",
			Snippet:         ">  3|     assert x == 2",
			RelatedLine:     1,
			RelatedQualName: "test_a",
		},
		{
			File:     "tests/test_b.py",
			Line:     1,
			Rule:     "only-happy-path",
			Severity: "note",
			Message:  "more than 3 tests without negative-path signals",
		},
		{
			File:     "tests/test_b.py",
			Line:     10,
			Rule:     "only-happy-path",
			Severity: "note",
			Message:  "more than 3 tests without negative-path signals",
		},
		{
			File:     "tests/test_c.py",
			Line:     1,
			Rule:     "only-happy-path",
			Severity: "note",
			Message:  "more than 3 tests without negative-path signals",
		},
	}
	var buf bytes.Buffer
	if err := scan.WriteHTML(&buf, findings, scan.CalculateScore(findings, 3)); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`<pre class="snippet">`,
		"assert x == 2",
		`twin test_a:1`,
		"same message as above",
		"more than 3 tests without negative-path signals",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	// Cross-file identical messages must remain visible (not collapsed).
	if strings.Count(out, "more than 3 tests without negative-path signals") < 2 {
		t.Fatalf("expected full message in at least two files, got:\n%s", out)
	}
}

func TestWriteHTML_ToolErrors(t *testing.T) {
	findings := []scan.Finding{
		{
			File:     "tests/broken.py",
			Line:     1,
			Rule:     "parse-error",
			Severity: "note",
			Message:  "AST parse error",
		},
		{
			File:     "tests/ok.py",
			Line:     2,
			Rule:     "weak-assert",
			Severity: "note",
			Message:  "weak",
		},
	}
	score := scan.CalculateScore(findings, 2)
	if score.ParseSkipped != 1 || score.Notes != 1 {
		t.Fatalf("score breakdown: %+v", score)
	}
	var buf bytes.Buffer
	if err := scan.WriteHTML(&buf, findings, score); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`data-sev="tool-error"`,
		`data-sev="tool-error"> tool error</label>`,
		"<strong>1</strong> tool error",
		" · 1 tool error · ",
		`data-sev="note"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(out, `data-sev="tool-error" checked>`) {
		t.Error("tool-error filter must be unchecked by default")
	}
}

func TestWriteHTML_PrecisionSortAndBadge(t *testing.T) {
	findings := []scan.Finding{
		{File: "a.py", Line: 1, Rule: "name-body-mismatch", Severity: "note", Message: "mismatch"},
		{File: "b.py", Line: 1, Rule: "assert-true", Severity: "warning", Message: "true"},
		{File: "c.py", Line: 1, Rule: "assert-true", Severity: "warning", Message: "true2"},
		{File: "d.py", Line: 1, Rule: "empty-test", Severity: "error", Message: "empty"},
		{File: "e.py", Line: 1, Rule: "no-assert", Severity: "note", Message: "none"},
	}
	var buf bytes.Buffer
	if err := scan.WriteHTML(&buf, findings, scan.CalculateScore(findings, 5)); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	// assert-true (0.90) before empty-test (1.0 default) — wait, empty-test defaults to 1.0
	// which is higher than assert-true 0.90. Order: empty-test (1.0), assert-true (0.90),
	// name-body-mismatch (0.35), no-assert (0.0).
	idxEmpty := strings.Index(out, `id="rule-empty-test"`)
	idxAssertTrue := strings.Index(out, `id="rule-assert-true"`)
	idxName := strings.Index(out, `id="rule-name-body-mismatch"`)
	idxNoAssert := strings.Index(out, `id="rule-no-assert"`)
	if idxEmpty < 0 || idxAssertTrue < 0 || idxName < 0 || idxNoAssert < 0 {
		t.Fatalf("missing rule sections: empty=%d assert=%d name=%d no=%d", idxEmpty, idxAssertTrue, idxName, idxNoAssert)
	}
	if idxEmpty >= idxAssertTrue || idxAssertTrue >= idxName || idxName >= idxNoAssert {
		t.Fatalf("want precision desc order empty < assert-true < name-body < no-assert, got %d %d %d %d",
			idxEmpty, idxAssertTrue, idxName, idxNoAssert)
	}
	for _, want := range []string{
		`<span class="prec">0.90</span>`,
		`<span class="prec">1.00</span>`,
		`<span class="prec">0.35</span>`,
		`<span class="prec">0.00</span>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing precision badge %q", want)
		}
	}
}
