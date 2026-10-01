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
	score.PathRoot = `C:\proj`
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
		"score-caption",
		"signal-caption",
		"precision &lt; 0.3",
		"--show-low-precision",
		"Confirmed density",
		"confirmed",
		"density",
		"vscode://file/",
		`data-view="file"`,
		`data-view="rule"`,
		"By file",
		"By rule",
		"file-section",
		`data-sev="note"> other</label>`,
		`data-sev="tool-error"> tool error</label>`,
		").toLowerCase();",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	if !strings.Contains(out, "grade") {
		t.Error("want muted grade text in hero")
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
	// Default view is by-file; rule panel hidden
	if !strings.Contains(out, `id="view-rule"`) || !strings.Contains(out, `view-panel is-hidden`) {
		t.Error("want rule view panel present and hidden by default")
	}
}

func TestWriteHTML_ShowGradeRing(t *testing.T) {
	score := scan.CalculateScore(nil, 1)
	score.ShowGrade = true
	var buf bytes.Buffer
	if err := scan.WriteHTML(&buf, nil, score); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "score-value") || !strings.Contains(out, `data-grade=`) {
		t.Fatalf("want Health Score ring when ShowGrade: %s", out[:min(400, len(out))])
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
	if !strings.Contains(out, "confirmed") {
		t.Fatalf("want confirmed hero, got: %s", out[:min(400, len(out))])
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
		"×2",
		"more than 3 tests without negative-path signals",
		"vscode://file/",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(out, "same message as above") {
		t.Error("must not use same-message-as-above; use ×N collapse")
	}
	// Cross-file identical messages must remain visible (not collapsed across files).
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

	// Rule view still present with precision-desc order.
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
	// By-file sections use hashed ids (stable, collision-free).
	if !strings.Contains(out, `id="file-`) || !strings.Contains(out, "file-section") {
		t.Error("want by-file section ids")
	}
}

func TestFileSectionIDsUnique(t *testing.T) {
	findings := []scan.Finding{
		{File: "a/b.py", Line: 1, Rule: "empty-test", Severity: "error", Message: "m1"},
		{File: "a-b.py", Line: 1, Rule: "empty-test", Severity: "error", Message: "m2"},
	}
	var buf bytes.Buffer
	if err := scan.WriteHTML(&buf, findings, scan.CalculateScore(findings, 2)); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// Both paths must appear; hashed ids must differ (no single shared sanitized id).
	if !strings.Contains(out, "a/b.py") || !strings.Contains(out, "a-b.py") {
		t.Fatal("want both file paths in report")
	}
	// Count distinct file- hex ids in section headers
	count := strings.Count(out, `class="file-section" id="file-`)
	if count != 2 {
		t.Fatalf("want 2 distinct file sections, got %d", count)
	}
}

func TestWriteHTML_TrendInHero(t *testing.T) {
	score := scan.CalculateScore(nil, 1)
	tr := scan.ComputeTrend(0, 0, 2, 1.0)
	tr.CurrentPreBaseline = true
	score.Trend = &tr
	var buf bytes.Buffer
	if err := scan.WriteHTML(&buf, nil, score); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "trend improved") {
		t.Fatalf("want trend in hero: %s", out[:min(600, len(out))])
	}
	if !strings.Contains(out, "pre-baseline") {
		t.Fatalf("want pre-baseline trend label: %s", out[:min(800, len(out))])
	}
}
