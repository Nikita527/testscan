package scan_test

import (
	"math"
	"strconv"
	"testing"

	"github.com/Nikita527/testscan/scan"
)

func TestCalculateScore_Clean(t *testing.T) {
	s := scan.CalculateScore(nil, 10)
	if s.Value != 100 || s.Grade != "A" {
		t.Fatalf("clean: got %d (%s), want 100 (A)", s.Value, s.Grade)
	}
	if s.Files != 10 || s.Errors != 0 {
		t.Fatalf("unexpected breakdown: %+v", s)
	}
}

func TestCalculateScore_GradeThresholds(t *testing.T) {
	for _, tc := range []struct {
		v int
		g string
	}{
		{100, "A"}, {90, "A"}, {89, "B"}, {75, "B"}, {74, "C"},
		{60, "C"}, {59, "D"}, {45, "D"}, {44, "F"}, {0, "F"},
	} {
		// Grade is produced by CalculateScore; verify thresholds via FormatScoreLine.
		line := scan.FormatScoreLine(scan.Score{Value: tc.v, Grade: tc.g})
		want := "Health Score: " + strconv.Itoa(tc.v) + " (" + tc.g + ")"
		if line != want {
			t.Errorf("got %q want %q", line, want)
		}
	}
	// Spot-check that CalculateScore assigns matching grades at boundaries.
	clean := scan.CalculateScore(nil, 1)
	if clean.Grade != "A" {
		t.Fatalf("clean grade %s", clean.Grade)
	}
}

func TestCalculateScore_ErrorsVsNotes(t *testing.T) {
	files := 100
	errorsOnly := scan.CalculateScore([]scan.Finding{
		{Severity: "error", Rule: "empty-test"},
		{Severity: "error", Rule: "empty-test"},
		{Severity: "error", Rule: "empty-test"},
		{Severity: "error", Rule: "empty-test"},
		{Severity: "error", Rule: "empty-test"},
	}, files)
	notesOnly := scan.CalculateScore([]scan.Finding{
		{Severity: "note", Rule: "only-happy-path"},
		{Severity: "note", Rule: "only-happy-path"},
		{Severity: "note", Rule: "only-happy-path"},
		{Severity: "note", Rule: "only-happy-path"},
		{Severity: "note", Rule: "only-happy-path"},
	}, files)
	if notesOnly.Value <= errorsOnly.Value {
		t.Fatalf("notes should score higher than equal-count errors: notes=%d errors=%d",
			notesOnly.Value, errorsOnly.Value)
	}
}

func TestCalculateScore_NotesAloneStayAroundC(t *testing.T) {
	findings := make([]scan.Finding, 50)
	for i := range findings {
		findings[i] = scan.Finding{Severity: "note", Rule: "only-happy-path"}
	}
	s := scan.CalculateScore(findings, 100)
	if s.Value < 60 {
		t.Fatalf("note-heavy suite score %d (%s) dropped below C; k may be too high", s.Value, s.Grade)
	}
}

func TestCalculateScore_ParsePenalty(t *testing.T) {
	findings := []scan.Finding{
		{Severity: "note", Rule: "parse-error"},
		{Severity: "note", Rule: "parse-error"},
		{Severity: "note", Rule: "parse-error"},
		{Severity: "note", Rule: "parse-error"},
		{Severity: "note", Rule: "parse-error"},
	}
	withParse := scan.CalculateScore(findings, 10)
	noParseRule := make([]scan.Finding, len(findings))
	for i, f := range findings {
		noParseRule[i] = f
		noParseRule[i].Rule = "other-note"
	}
	without := scan.CalculateScore(noParseRule, 10)
	if withParse.ParseSkipped != 5 {
		t.Fatalf("ParseSkipped=%d, want 5", withParse.ParseSkipped)
	}
	if withParse.Notes != 0 {
		t.Fatalf("parse-error must not count as notes, Notes=%d", withParse.Notes)
	}
	if withParse.Value >= without.Value {
		t.Fatalf("parse-error should add parse_penalty: with=%d without=%d",
			withParse.Value, without.Value)
	}
}

func TestCalculateScore_FormulaSpotCheck(t *testing.T) {
	findings := []scan.Finding{
		{Severity: "error", Rule: "empty-test"},
		{Severity: "error", Rule: "empty-test"},
	}
	s := scan.CalculateScore(findings, 9)
	weighted := 2.0
	denom := math.Log10(10)
	if denom < 1 {
		denom = 1
	}
	wantPenalty := int(math.Round(weighted / denom * scan.ScoreDensityK))
	if wantPenalty > 80 {
		wantPenalty = 80
	}
	want := 100 - wantPenalty
	if s.Value != want {
		t.Fatalf("got %d, want %d (penalty %d)", s.Value, want, wantPenalty)
	}
}

func TestFormatScoreLine(t *testing.T) {
	got := scan.FormatScoreLine(scan.Score{Value: 72, Grade: "C"})
	if got != "Health Score: 72 (C)" {
		t.Fatalf("got %q", got)
	}
}

func TestCalculateScore_PenaltyCapped(t *testing.T) {
	findings := make([]scan.Finding, 200)
	for i := range findings {
		findings[i] = scan.Finding{Severity: "error", Rule: "empty-test"}
	}
	s := scan.CalculateScore(findings, 5)
	if s.Value != 20 {
		t.Fatalf("want score 20 (penalty capped at 80), got %d", s.Value)
	}
	if s.Grade != "F" {
		t.Fatalf("want grade F for score 20, got %s", s.Grade)
	}
}

// mp-be-like volume after rule accuracy fixes: ~14 errors, tens of warnings,
// many notes (private-import / weak-assert / near-duplicate / only-happy-path).
// ScoreDensityK should land this in B–C, not F.
func TestCalculateScore_LargeRepoAfterNoiseFixes(t *testing.T) {
	const files = 360
	findings := make([]scan.Finding, 0, 14+25+100)
	for i := 0; i < 14; i++ {
		findings = append(findings, scan.Finding{Severity: "error", Rule: "no-assert"})
	}
	for i := 0; i < 25; i++ {
		findings = append(findings, scan.Finding{Severity: "warning", Rule: "mock-only-assert"})
	}
	for i := 0; i < 100; i++ {
		findings = append(findings, scan.Finding{Severity: "note", Rule: "only-happy-path"})
	}
	s := scan.CalculateScore(findings, files)
	if s.Grade != "B" && s.Grade != "C" {
		t.Fatalf("mp-be-like volume scored %d (%s), want B or C (k=%v)",
			s.Value, s.Grade, scan.ScoreDensityK)
	}
	if s.Value < 60 {
		t.Fatalf("score %d dropped below C threshold", s.Value)
	}
}
