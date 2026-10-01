package scan_test

import (
	"math"
	"strings"
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
		// Grade is JSON-only: the text header must never carry it.
		line := scan.FormatScoreLine(scan.Score{Value: tc.v, Grade: tc.g})
		if strings.Contains(line, "Health") || strings.Contains(line, "grade") {
			t.Errorf("text header leaks grade: %q", line)
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
	withOverrides(t, map[string]float64{"no-assert": 0, "broad-raises": 1, "only-happy-path": 0.5, "name-body-mismatch": 0.35})
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
	withOverrides(t, map[string]float64{"no-assert": 0, "broad-raises": 1, "only-happy-path": 0.5, "name-body-mismatch": 0.35})
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
	got := scan.FormatScoreLine(scan.Score{
		ActionableCount: 7, ActionableDensity: 0.19, ProvisionalCount: 6, Tests: 3650, Files: 382,
	})
	want := "testscan: 7 actionable findings (0.19 per 100 tests) \u00b7 6 provisional \u00b7 3650 tests in 382 files"
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
	one := scan.FormatScoreLine(scan.Score{ActionableCount: 1, Tests: 10, Files: 2})
	if !strings.HasPrefix(one, "testscan: 1 actionable finding (") {
		t.Fatalf("singular: %q", one)
	}
	noTests := scan.FormatScoreLine(scan.Score{Files: 4})
	if noTests != "testscan: 0 actionable findings \u00b7 0 provisional \u00b7 4 files" {
		t.Fatalf("unknown tests: %q", noTests)
	}
	withTrend := scan.FormatScoreLine(scan.Score{
		ActionableCount: 3, ActionableDensity: 1.5, Tests: 200, Files: 5,
		Trend: &scan.Trend{Direction: "improved", DeltaCount: -2},
	})
	if !strings.HasSuffix(withTrend, " \u00b7 trend improved (\u0394 -2)") {
		t.Fatalf("got %q", withTrend)
	}
	withPre := scan.FormatScoreLine(scan.Score{
		Trend: &scan.Trend{Direction: "unchanged", DeltaCount: 0, CurrentPreBaseline: true},
	})
	if !strings.HasSuffix(withPre, "trend unchanged (\u0394 0) [pre-baseline]") {
		t.Fatalf("got %q", withPre)
	}
}

func TestRuleTier(t *testing.T) {
	withCatalog(t, map[string]scan.PrecisionInfo{
		"act-exact":       scan.Measured(20, 20),
		"act-n19":         scan.Measured(19, 19),
		"act-wilson-low":  scan.Measured(16, 20),
		"prov":            scan.Measured(4, 5),
		"prov-p79":        scan.Measured(15, 19),
		"prov-n4":         scan.Measured(4, 4),
		"est":             {Precision: 1, Source: scan.SourceEstimated},
		"derived":         {Precision: 0.95, N: 40, Source: scan.SourceMeasured},
		"explicit-wilson": {Precision: 0.9, N: 25, Source: scan.SourceMeasured, WilsonLow: 0.69},
	})
	for rule, want := range map[string]string{
		"act-exact": scan.TierActionable, "act-n19": scan.TierProvisional,
		"act-wilson-low": scan.TierProvisional, "prov": scan.TierProvisional,
		"prov-p79": scan.TierLow, "prov-n4": scan.TierLow, "est": scan.TierLow,
		"derived": scan.TierActionable, "explicit-wilson": scan.TierProvisional,
		"not-in-catalog": scan.TierLow, "parse-error": scan.TierLow,
	} {
		if got := scan.RuleTier(rule); got != want {
			t.Errorf("RuleTier(%s)=%s, want %s", rule, got, want)
		}
	}
}

func TestCountTiersAndDensity(t *testing.T) {
	withCatalog(t, map[string]scan.PrecisionInfo{
		"a": scan.Measured(30, 30), "p": scan.Measured(5, 5),
	})
	findings := []scan.Finding{
		{Rule: "a", Severity: "error"}, {Rule: "a", Severity: "error"},
		{Rule: "p", Severity: "warning"}, {Rule: "zzz", Severity: "warning"},
		{Rule: "parse-error", Severity: "note"},
	}
	s := scan.CalculateScoreWithTests(findings, 3, 400)
	if s.ActionableCount != 2 || s.ProvisionalCount != 1 || s.ShownCount != 5 || s.Tests != 400 {
		t.Fatalf("score=%+v", s)
	}
	if s.ActionableDensity != 0.5 { // 2 per 400 tests = 0.5 per 100
		t.Fatalf("density=%v, want 0.5", s.ActionableDensity)
	}
	if s.ConfirmedCount != 2 || s.ConfirmedDensity != 0.5 {
		t.Fatalf("deprecated confirmed_* must mirror actionable: %+v", s)
	}
	zero := scan.CalculateScoreWithTests(findings, 3, 0)
	if zero.ActionableDensity != 0 {
		t.Fatalf("tests==0 must give density 0, got %v", zero.ActionableDensity)
	}
	if d := scan.DensityPer100(5, 0); d != 0 {
		t.Fatalf("DensityPer100 with 0 tests = %v", d)
	}
}

func TestFilterForDisplay(t *testing.T) {
	withCatalog(t, map[string]scan.PrecisionInfo{
		"act": scan.Measured(30, 30), "prov": scan.Measured(5, 5),
		"low": {Precision: 1, Source: scan.SourceEstimated},
	})
	findings := []scan.Finding{
		{Rule: "act", Severity: "error"},
		{Rule: "prov", Severity: "warning"},
		{Rule: "low", Severity: "error"},
		{Rule: "act", Severity: "note"}, // focus drops notes
		{Rule: "parse-error", Severity: "note"},
	}
	rules := func(fs []scan.Finding) string {
		var out []string
		for _, f := range fs {
			out = append(out, f.Rule+"/"+f.Severity)
		}
		return strings.Join(out, ",")
	}
	if got := rules(scan.FilterForDisplay(findings, scan.DisplayOptions{})); got != "act/error,prov/warning" {
		t.Errorf("default: %s", got)
	}
	if got := rules(scan.FilterForDisplay(findings, scan.DisplayOptions{ShowLowPrecision: true})); got != "act/error,prov/warning,low/error" {
		t.Errorf("show-low: %s", got)
	}
	if got := scan.FilterForDisplay(findings, scan.DisplayOptions{All: true}); len(got) != len(findings) {
		t.Errorf("all: %s", rules(got))
	}
}

func TestPrecisionInfo_WilsonDerived(t *testing.T) {
	p := scan.PrecisionInfo{Precision: 0.9, N: 40, Source: scan.SourceMeasured}
	want := scan.WilsonLower(36, 40, scan.WilsonZ95)
	if math.Abs(p.Wilson()-want) > 1e-9 {
		t.Fatalf("wilson=%v want %v", p.Wilson(), want)
	}
	if (scan.PrecisionInfo{Precision: 1, Source: scan.SourceEstimated}).Wilson() != 0 {
		t.Fatal("estimated entries have no wilson bound")
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
			if tr.PrevActionableCount != tc.prevN || tr.PrevActionableDensity != tc.prevD {
				t.Fatalf("prev fields: %+v", tr)
			}
		})
	}
}

func TestTrendCurrentMetrics_PreBaseline(t *testing.T) {
	withCatalog(t, map[string]scan.PrecisionInfo{"act": scan.Measured(30, 30)})
	post := scan.CalculateScoreWithTests(nil, 10, 200) // 0 actionable after baseline
	pre := []scan.Finding{
		{Rule: "act", Severity: "error", File: "a.py"},
		{Rule: "act", Severity: "error", File: "b.py"},
		{Rule: "no-assert", Severity: "note", File: "c.py"}, // dropped by display filters
	}
	count, dens := scan.TrendCurrentMetrics(post, pre, scan.DisplayOptions{}, 200)
	if count != 2 {
		t.Fatalf("pre-baseline actionable=%d, want 2", count)
	}
	if dens != 1.0 {
		t.Fatalf("dens=%v, want 1.0 per 100 tests", dens)
	}
	c2, d2 := scan.TrendCurrentMetrics(post, nil, scan.DisplayOptions{}, 200)
	if c2 != 0 || d2 != 0 {
		t.Fatalf("nil pre should use post score, got %d %v", c2, d2)
	}
}

func TestComputeTrend_LegacyIgnoresDensity(t *testing.T) {
	tr := scan.ComputeTrend(3, 9.9, 3, -1)
	if tr.Direction != "unchanged" || tr.DeltaDensity != 0 {
		t.Fatalf("trend=%+v", tr)
	}
	if tr := scan.ComputeTrend(2, 9.9, 3, -1); tr.Direction != "improved" {
		t.Fatalf("trend=%+v", tr)
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

func TestFilterTiers(t *testing.T) {
	withCatalog(t, map[string]scan.PrecisionInfo{
		"act": scan.Measured(30, 30), "prov": scan.Measured(5, 5),
		"est": {Precision: 1, Source: scan.SourceEstimated},
	})
	in := []scan.Finding{
		{Rule: "act"}, {Rule: "prov"}, {Rule: "est"}, {Rule: "unknown"}, {Rule: "parse-error"},
	}
	got := scan.FilterTiers(in)
	if len(got) != 3 || got[0].Rule != "act" || got[1].Rule != "prov" || got[2].Rule != "parse-error" {
		t.Fatalf("got %v", got)
	}
}
