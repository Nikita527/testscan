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

	score := scan.CalculateScoreWithTests(findings, 2, 40)
	score.PathRoot = `C:\proj`
	var buf bytes.Buffer
	if err := scan.WriteHTML(&buf, findings, score); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	for _, want := range []string{
		"<!DOCTYPE html>",
		"testscan",
		"id=\"rule-empty-test\"",
		"id=\"rule-only-happy-path\"",
		"tests/test_a.py",
		"empty test body",
		"data-sev=\"error\"",
		"data-sev=\"warning\"",
		`id="q"`,
		"signal-caption",
		"--show-low-precision",
		"--all",
		"actionable-count",
		"per 100 tests",
		"hero-secondary",
		"0 provisional \u00b7 40 tests in 2 files",
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
	assertNoGrade(t, out)
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

func assertNoGrade(t *testing.T, out string) {
	t.Helper()
	for _, bad := range []string{"score-value", "score-grade", "ring-fg", "data-grade", "grade-muted", "Health Score", "deprecated"} {
		if strings.Contains(out, bad) {
			t.Errorf("HTML must not carry grade artifacts, found %q", bad)
		}
	}
}

func TestWriteHTML_HeroSingleNumber(t *testing.T) {
	withCatalog(t, map[string]scan.PrecisionInfo{
		"act":  scan.Measured(24, 25),
		"prov": scan.Measured(5, 5),
		"low":  {Precision: 0.5, Source: scan.SourceEstimated},
	})
	findings := []scan.Finding{
		{File: "a.py", Line: 1, Rule: "act", Severity: "error", Message: "m1", Fingerprint: "fp1"},
		{File: "a.py", Line: 2, Rule: "prov", Severity: "warning", Message: "m2", Fingerprint: "fp2"},
		{File: "a.py", Line: 3, Rule: "low", Severity: "warning", Message: "m3", Fingerprint: "fp3"},
	}
	score := scan.CalculateScoreWithTests(findings, 1, 50)
	score.ShowLow = true
	var buf bytes.Buffer
	if err := scan.WriteHTML(&buf, findings, score); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	assertNoGrade(t, out)
	if strings.Count(out, `class="actionable-count"`) != 1 {
		t.Error("exactly one headline number expected")
	}
	for _, want := range []string{
		`<span class="actionable-count">1</span>`,
		"2.00 per 100 tests",
		"1 provisional \u00b7 50 tests in 1 files",
		`class="tier tier-provisional"`,
		`class="tier tier-low"`,
		"p=0.96 n=25 measured",
		">estimated<",
		"actionable, provisional and low-precision",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	// No second headline-ish count such as "N finding(s)".
	if strings.Contains(out, "finding(s)") {
		t.Error("contradictory finding count line must be gone")
	}
}

func TestWriteHTML_CollapsedGroupLabelsAllFingerprints(t *testing.T) {
	findings := []scan.Finding{
		{File: "a.py", Line: 4, Rule: "empty-test", Severity: "error", Message: "same", Fingerprint: "fpA"},
		{File: "a.py", Line: 8, Rule: "empty-test", Severity: "error", Message: "same", Fingerprint: "fpB"},
		{File: "a.py", Line: 9, Rule: "empty-test", Severity: "error", Message: "same"}, // no fingerprint
		{File: "a.py", Line: 12, Rule: "empty-test", Severity: "error", Message: "same", Fingerprint: "fpC"},
	}
	var buf bytes.Buffer
	if err := scan.WriteHTML(&buf, findings, scan.CalculateScore(findings, 1)); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "\u00d74") {
		t.Fatal("want x4 collapse badge")
	}
	// One .lbl per group, carrying every fingerprint and its own line.
	if !strings.Contains(out, `data-fps="fpA,fpB,fpC"`) || !strings.Contains(out, `data-lines="4,8,12"`) {
		t.Fatalf("group must carry all fingerprints/lines: %s", out)
	}
	if strings.Count(out, `data-fps="`) != 2 { // file view + rule view
		t.Errorf("want one label group per view, got %d", strings.Count(out, `data-fps="`))
	}
	// JS applies a toggle to every fingerprint of the group.
	for _, want := range []string{"groupFps(span)", "items.every(", "items.forEach("} {
		if !strings.Contains(out, want) {
			t.Errorf("labelling script missing %q", want)
		}
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
	if !strings.Contains(out, "No findings") {
		t.Fatal("want empty toc message")
	}
	if !strings.Contains(out, `<span class="actionable-count">0</span>`) {
		t.Fatalf("want actionable hero, got: %s", out[:min(400, len(out))])
	}
	assertNoGrade(t, out)
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
		" · 1 tool error</p>",
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
	withOverrides(t, map[string]float64{"empty-test": 1, "assert-true": 0.9, "name-body-mismatch": 0.35, "no-assert": 0})
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
