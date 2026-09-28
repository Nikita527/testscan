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
		line := scan.FormatScoreLine(scan.Score{Value: tc.v, Grade: tc.g})
		want := "Health Score: " + strconv.Itoa(tc.v) + " (" + tc.g + ")"
		if line != want {
			t.Errorf("got %q want %q", line, want)
		}
	}
	clean := scan.CalculateScore(nil, 1)
	if clean.Grade != "A" {
		t.Fatalf("clean grade %s", clean.Grade)
	}
}

func TestCalculateScore_NotesDoNotAffectGrade(t *testing.T) {
	files := 100
	notesOnly := scan.CalculateScore([]scan.Finding{
		{Severity: "note", Rule: "only-happy-path"},
		{Severity: "note", Rule: "only-happy-path"},
		{Severity: "note", Rule: "weak-assert"},
		{Severity: "note", Rule: "near-duplicate-test"},
		{Severity: "note", Rule: "test-imports-implementation-private"},
	}, files)
	if notesOnly.Value != 100 || notesOnly.Grade != "A" {
		t.Fatalf("notes must not lower score: got %d (%s)", notesOnly.Value, notesOnly.Grade)
	}
	if notesOnly.Notes != 5 {
		t.Fatalf("Notes=%d, want 5", notesOnly.Notes)
	}
}

func TestCalculateScore_LowPrecisionErrorsIgnored(t *testing.T) {
	low := scan.CalculateScore([]scan.Finding{
		{Severity: "error", Rule: "no-assert"},
		{Severity: "error", Rule: "no-assert"},
		{Severity: "warning", Rule: "mock-only-assert"},
	}, 100)
	if low.Value != 100 {
		t.Fatalf("low-precision findings must not lower score, got %d", low.Value)
	}
	if low.Errors != 2 || low.Warnings != 1 {
		t.Fatalf("counts still tracked: %+v", low)
	}
}

func TestCalculateScore_HighPrecisionErrorsCount(t *testing.T) {
	findings := []scan.Finding{
		{Severity: "error", Rule: "empty-test"},
		{Severity: "error", Rule: "empty-test"},
	}
	s := scan.CalculateScore(findings, 9)
	weighted := 2.0 // empty-test precision defaults to 1.0
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

func TestCalculateScore_BroadRaisesPrecision(t *testing.T) {
	// broad-raises precision 1.0 → full warning weight
	s := scan.CalculateScore([]scan.Finding{
		{Severity: "warning", Rule: "broad-raises"},
	}, 9)
	weighted := 0.5
	denom := math.Log10(10)
	wantPenalty := int(math.Round(weighted / denom * scan.ScoreDensityK))
	want := 100 - wantPenalty
	if s.Value != want {
		t.Fatalf("got %d, want %d", s.Value, want)
	}
}

func TestPrecisionWeight(t *testing.T) {
	if w := scan.PrecisionWeight("no-assert"); w != 0 {
		t.Fatalf("no-assert weight=%v, want 0", w)
	}
	if w := scan.PrecisionWeight("broad-raises"); w != 1 {
		t.Fatalf("broad-raises weight=%v, want 1", w)
	}
	if w := scan.PrecisionWeight("only-happy-path"); w != 0.5 {
		t.Fatalf("only-happy-path weight=%v, want 0.5", w)
	}
	if w := scan.PrecisionWeight("name-body-mismatch"); w != 0.35 {
		t.Fatalf("name-body-mismatch mid-precision weight=%v, want 0.35", w)
	}
	if w := scan.PrecisionWeight("empty-test"); w != 1 {
		t.Fatalf("unknown rule default weight=%v, want 1", w)
	}
}

func TestCalculateScore_MidPrecisionWarningsCount(t *testing.T) {
	// name-body-mismatch precision 0.35 ≥ floor → volume must lower score (not fake-A).
	findings := make([]scan.Finding, 120)
	for i := range findings {
		findings[i] = scan.Finding{Severity: "warning", Rule: "name-body-mismatch"}
	}
	s := scan.CalculateScore(findings, 360)
	if s.Value >= 90 || s.Grade == "A" {
		t.Fatalf("120 mid-precision warnings must not stay grade A, got %d (%s)", s.Value, s.Grade)
	}
	if s.WarningsInGrade != 120 || s.WarningsIgnored != 0 {
		t.Fatalf("in_grade=%d ignored=%d", s.WarningsInGrade, s.WarningsIgnored)
	}
}

func TestCalculateScore_ZeroPrecisionWarningsIgnored(t *testing.T) {
	s := scan.CalculateScore([]scan.Finding{
		{Severity: "warning", Rule: "mock-only-assert"},
		{Severity: "warning", Rule: "no-assert"},
	}, 100)
	if s.Value != 100 || s.WarningsIgnored != 2 || s.WarningsInGrade != 0 {
		t.Fatalf("got %+v", s)
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
	if withParse.ParseSkipped != 5 {
		t.Fatalf("ParseSkipped=%d, want 5", withParse.ParseSkipped)
	}
	if withParse.Notes != 0 {
		t.Fatalf("parse-error must not count as notes, Notes=%d", withParse.Notes)
	}
	clean := scan.CalculateScore(nil, 10)
	if withParse.Value != clean.Value {
		t.Fatalf("parse-error must not lower score: with=%d clean=%d",
			withParse.Value, clean.Value)
	}
	if withParse.Value != 100 {
		t.Fatalf("want score 100 for 10 files with only parse-errors, got %d", withParse.Value)
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

// High-precision error/warning volume on a large repo should land in B–C,
// while low-precision / note noise is ignored by the grade.
func TestCalculateScore_LargeRepoHighPrecision(t *testing.T) {
	const files = 360
	findings := make([]scan.Finding, 0, 14+10+100)
	for i := 0; i < 14; i++ {
		findings = append(findings, scan.Finding{Severity: "error", Rule: "empty-test"})
	}
	for i := 0; i < 10; i++ {
		findings = append(findings, scan.Finding{Severity: "warning", Rule: "broad-raises"})
	}
	for i := 0; i < 100; i++ {
		findings = append(findings, scan.Finding{Severity: "note", Rule: "only-happy-path"})
	}
	s := scan.CalculateScore(findings, files)
	if s.Grade != "B" && s.Grade != "C" {
		t.Fatalf("large high-precision volume scored %d (%s), want B or C (k=%v)",
			s.Value, s.Grade, scan.ScoreDensityK)
	}
	if s.Value < 60 {
		t.Fatalf("score %d dropped below C threshold", s.Value)
	}
}
