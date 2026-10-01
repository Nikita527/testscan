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
		line := scan.FormatScoreLine(scan.Score{Value: tc.v, Grade: tc.g, ShowGrade: true})
		want := "Confirmed: 0 (density 0.00)\nHealth Score: " + strconv.Itoa(tc.v) + " (" + tc.g + ") [deprecated]"
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
		t.Fatalf("cataloged empty-test weight=%v, want 1", w)
	}
	if w := scan.PrecisionWeight("no-such-rule"); w != 0 {
		t.Fatalf("unknown rule weight=%v, want 0", w)
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
	got := scan.FormatScoreLine(scan.Score{ConfirmedCount: 5, ConfirmedDensity: 2.5})
	if got != "Confirmed: 5 (density 2.50)" {
		t.Fatalf("got %q", got)
	}
	withTrend := scan.FormatScoreLine(scan.Score{
		ConfirmedCount: 3, ConfirmedDensity: 1.5,
		Trend: &scan.Trend{Direction: "improved", DeltaCount: -2},
	})
	if withTrend != "Confirmed: 3 (density 1.50) · trend improved (Δ -2)" {
		t.Fatalf("got %q", withTrend)
	}
	withPre := scan.FormatScoreLine(scan.Score{
		ConfirmedCount: 0, ConfirmedDensity: 0,
		Trend: &scan.Trend{Direction: "unchanged", DeltaCount: 0, CurrentPreBaseline: true},
	})
	if withPre != "Confirmed: 0 (density 0.00) · trend unchanged (Δ 0) [pre-baseline]" {
		t.Fatalf("got %q", withPre)
	}
	withGrade := scan.FormatScoreLine(scan.Score{
		ConfirmedCount: 1, ConfirmedDensity: 0.5,
		Value: 72, Grade: "C", ShowGrade: true,
	})
	want := "Confirmed: 1 (density 0.50)\nHealth Score: 72 (C) [deprecated]"
	if withGrade != want {
		t.Fatalf("got %q", withGrade)
	}
}

func TestCountConfirmed(t *testing.T) {
	findings := []scan.Finding{
		{Rule: "empty-test"},         // 1.0
		{Rule: "name-body-mismatch"}, // 0.35
		{Rule: "weak-assert"},        // 0.25 < 0.3
		{Rule: "no-assert"},          // 0.0
		{Rule: "parse-error"},        // excluded
	}
	if n := scan.CountConfirmed(findings); n != 2 {
		t.Fatalf("CountConfirmed=%d, want 2", n)
	}
}

func TestConfirmedDensity(t *testing.T) {
	// files=9 → log10(10)=1
	d := scan.ConfirmedDensity(4, 9)
	if d != 4.0 {
		t.Fatalf("density=%v, want 4", d)
	}
	// files=0 → denom max(1, log10(1))=1
	d0 := scan.ConfirmedDensity(3, 0)
	if d0 != 3.0 {
		t.Fatalf("density=%v, want 3", d0)
	}
}

func TestComputeTrend(t *testing.T) {
	cases := []struct {
		name         string
		currN, prevN int
		currD, prevD float64
		wantDir      string
		wantDelta    int
	}{
		{"count_down", 3, 5, 1.0, 2.0, "improved", -2},
		{"count_up", 7, 5, 3.0, 2.0, "worsened", 2},
		{"equal_density_down", 5, 5, 1.0, 2.0, "improved", 0},
		{"equal_density_up", 5, 5, 3.0, 2.0, "worsened", 0},
		{"unchanged", 5, 5, 2.0, 2.0, "unchanged", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tr := scan.ComputeTrend(tc.currN, tc.currD, tc.prevN, tc.prevD)
			if tr.Direction != tc.wantDir || tr.DeltaCount != tc.wantDelta {
				t.Fatalf("got dir=%s Δ=%d, want %s Δ=%d", tr.Direction, tr.DeltaCount, tc.wantDir, tc.wantDelta)
			}
			if tr.PrevConfirmedCount != tc.prevN || tr.PrevConfirmedDensity != tc.prevD {
				t.Fatalf("prev fields: %+v", tr)
			}
		})
	}
}

func TestTrendCurrentMetrics_PreBaseline(t *testing.T) {
	post := scan.CalculateScore(nil, 10) // confirmed 0 after baseline
	pre := []scan.Finding{
		{Rule: "empty-test", Severity: "error", File: "a.py"},
		{Rule: "empty-test", Severity: "error", File: "b.py"},
		{Rule: "no-assert", Severity: "note", File: "c.py"}, // precision 0 → dropped by LP filter
	}
	count, dens := scan.TrendCurrentMetrics(post, pre, false, true, 10)
	if count != 2 {
		t.Fatalf("pre-baseline confirmed=%d, want 2", count)
	}
	wantDens := scan.ConfirmedDensity(2, 10)
	if dens != wantDens {
		t.Fatalf("dens=%v, want %v", dens, wantDens)
	}
	c2, d2 := scan.TrendCurrentMetrics(post, nil, false, true, 10)
	if c2 != 0 || d2 != 0 {
		t.Fatalf("nil pre should use post score, got %d %v", c2, d2)
	}
}

func TestCalculateScore_ConfirmedFields(t *testing.T) {
	findings := []scan.Finding{
		{Severity: "error", Rule: "empty-test"},
		{Severity: "note", Rule: "name-body-mismatch"},
		{Severity: "note", Rule: "weak-assert"},
		{Severity: "note", Rule: "parse-error"},
	}
	s := scan.CalculateScore(findings, 9)
	if s.ConfirmedCount != 2 {
		t.Fatalf("ConfirmedCount=%d, want 2", s.ConfirmedCount)
	}
	if s.ConfirmedDensity != 2.0 {
		t.Fatalf("ConfirmedDensity=%v, want 2", s.ConfirmedDensity)
	}
	if !s.GradeDeprecated {
		t.Fatal("GradeDeprecated should be true")
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

func TestFilterLowPrecision(t *testing.T) {
	in := []scan.Finding{
		{Rule: "no-assert", Severity: "note"},
		{Rule: "empty-test", Severity: "error"},
		{Rule: "weak-assert", Severity: "note"},        // 0.25 < 0.3
		{Rule: "name-body-mismatch", Severity: "note"}, // 0.35
		{Rule: "parse-error", Severity: "note"},
	}
	got := scan.FilterLowPrecision(in)
	if len(got) != 3 {
		t.Fatalf("got %d, want 3: %v", len(got), got)
	}
	for _, f := range got {
		switch f.Rule {
		case "empty-test", "name-body-mismatch", "parse-error":
		default:
			t.Fatalf("unexpected kept rule %q", f.Rule)
		}
	}
}
