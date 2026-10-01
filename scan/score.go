package scan

import (
	"fmt"
	"math"
	"strings"
)

// ScoreDensityK scales finding density into a 0–80 penalty.
// Calibrated so a modest number of high-precision error/warning findings
// on a large repo lands around B–C; notes and near-zero-precision rules
// do not drag the grade alone. (Health Score is deprecated and JSON-only.)
const ScoreDensityK = 2.0

// MinPrecisionForGrade is the floor below which a rule contributes 0 to Health Score.
// Mid-precision rules (e.g. 0.35–0.5) still count proportionally — only near-zero
// / known-broken detectors are fully ignored. It is also the floor used by FilterFocus.
const MinPrecisionForGrade = 0.15

// Precision tier thresholds. A rule is judged only by labelled (measured) data;
// estimated or unmeasured rules are always the "low" tier.
const (
	// ActionableMinWilson is the minimum Wilson lower bound (95%) for the actionable tier.
	ActionableMinWilson = 0.7
	// ActionableMinN is the minimum number of labelled findings (TP+FP) for actionable.
	ActionableMinN = 20
	// ProvisionalMinPrecision is the minimum point precision for the provisional tier.
	ProvisionalMinPrecision = 0.8
	// ProvisionalMinN is the minimum number of labelled findings for provisional.
	ProvisionalMinN = 5
)

// Tier names reported in text, HTML and JSON.
const (
	TierActionable  = "actionable"
	TierProvisional = "provisional"
	TierLow         = "low"
)

// PrecisionInfo describes the precision of one rule in the catalog.
// Source is "measured" (from labelled findings; N and WilsonLow are set) or
// "estimated" (triage / post-fix guess; N is 0, WilsonLow may be 0).
// For measured entries TP (true positives out of N) may be left 0, in which case
// it is derived as round(Precision*N); a zero WilsonLow is computed from TP and N.
type PrecisionInfo struct {
	Precision float64
	N         int
	Source    string // "measured" | "estimated"
	WilsonLow float64
	TP        int
}

// Precision sources.
const (
	SourceMeasured  = "measured"
	SourceEstimated = "estimated"
)

func est(p float64) PrecisionInfo {
	return PrecisionInfo{Precision: p, Source: SourceEstimated}
}

// Measured builds a measured catalog entry from labelled counts (tp of n = TP+FP).
func Measured(tp, n int) PrecisionInfo {
	p := 0.0
	if n > 0 {
		p = float64(tp) / float64(n)
	}
	return PrecisionInfo{
		Precision: p, N: n, Source: SourceMeasured, TP: tp,
		WilsonLow: WilsonLower(tp, n, WilsonZ95),
	}
}

// Wilson returns the Wilson lower bound: WilsonLow when set, else computed for
// measured entries, else 0.
func (p PrecisionInfo) Wilson() float64 {
	if p.WilsonLow > 0 {
		return p.WilsonLow
	}
	if p.Source != SourceMeasured || p.N <= 0 {
		return 0
	}
	tp := p.TP
	if tp == 0 && p.Precision > 0 {
		tp = int(math.Round(p.Precision * float64(p.N)))
	}
	return WilsonLower(tp, p.N, WilsonZ95)
}

// Tier classifies the entry: actionable, provisional or low.
func (p PrecisionInfo) Tier() string {
	if p.Source != SourceMeasured {
		return TierLow
	}
	if p.N >= ActionableMinN && p.Wilson() >= ActionableMinWilson {
		return TierActionable
	}
	if p.N >= ProvisionalMinN && p.Precision >= ProvisionalMinPrecision {
		return TierProvisional
	}
	return TierLow
}

// RulePrecision is the measured/estimated precision for score weighting
// (from triage + post-fix estimates). Rules below MinPrecisionForGrade get weight 0.
// Rules ABSENT from the map are unknown: precision 0, estimated — they are never
// actionable/provisional until measured entries are added here (see LookupPrecision,
// Measured and `testscan precision`).
var RulePrecision = map[string]PrecisionInfo{
	"no-assert":                           Measured(0, 4),
	"broad-raises":                        Measured(2, 2),
	"assert-in-emptyable-loop":            Measured(1, 1),
	"mock-only-assert":                    Measured(0, 7),
	"mock-tautology":                      est(0.0),
	"weak-assert":                         Measured(0, 13),
	"near-duplicate-test":                 Measured(6, 7),
	"only-happy-path":                     est(0.50),
	"test-imports-implementation-private": Measured(7, 39),
	"parse-error":                         est(0.0),
	// AI-focused / newer rules — proportional until corpus-measured higher.
	"expected-recomputed": est(0.60),
	"self-patched-sut":    est(0.55),
	"name-body-mismatch":  Measured(0, 1),
	"commented-assert":    est(0.55),
	"overbroad-equality":  Measured(0, 5),
	"assert-true":         est(0.90),
	"sleep-in-test":       Measured(1, 1),
	"wall-clock-in-test":  Measured(6, 6),
	// Opt-in project rules — conservative until corpus-measured.
	"error-contract-assert": est(0.55),
	"raises-without-check":  est(0.50),
	"missing-mirror-test":   est(0.70),
	"rbac-mutation-guard":   est(0.45),
	// Rules that were previously uncatalogued and implicitly trusted at 1.0.
	// Listed explicitly (estimated) so Health Score weights are unchanged; unknown IDs get 0.
	"empty-test":          est(1.0),
	"assert-equals-same":  est(1.0),
	"assert-tuple":        est(1.0),
	"duplicate-test-name": est(1.0),
	"fake-mock-assert":    est(1.0),
	"no-behavior-change":  est(1.0),
	"overmocked-io":       est(1.0),
	"skip-without-reason": est(1.0),
	"snapshot-only":       est(1.0),
	"swallowed-exception": est(1.0),
	"todo-test":           est(1.0),
}

// LookupPrecision returns the catalog entry for a rule. For an unknown rule it
// returns {Precision: 0, Source: "estimated"} and false.
func LookupPrecision(ruleID string) (PrecisionInfo, bool) {
	if p, ok := RulePrecision[ruleID]; ok {
		return p, true
	}
	return PrecisionInfo{Precision: 0, Source: SourceEstimated}, false
}

// RuleTier returns the precision tier of a rule ("actionable"|"provisional"|"low").
// Unknown rules are "low".
func RuleTier(rule string) string {
	info, _ := LookupPrecision(rule)
	return info.Tier()
}

// Trend compares the current actionable metrics to a previous compare point.
// Densities are findings per 100 tests. A legacy compare point (a report that
// predates per-100-tests density) has PrevActionableDensity < 0 and is compared
// on count only.
type Trend struct {
	Direction             string  `json:"direction"` // "improved"|"worsened"|"unchanged"
	PrevActionableCount   int     `json:"prev_actionable_count"`
	PrevActionableDensity float64 `json:"prev_actionable_density"`
	// PrevConfirmedCount / PrevConfirmedDensity are deprecated aliases of the
	// prev_actionable_* values (kept for one release for JSON consumers).
	PrevConfirmedCount   int     `json:"prev_confirmed_count"`
	PrevConfirmedDensity float64 `json:"prev_confirmed_density"`
	DeltaCount           int     `json:"delta_count"`   // current - prev
	DeltaDensity         float64 `json:"delta_density"` // current - prev (0 when prev density unknown)
	// CurrentPreBaseline is true when the trend "current" side was computed before
	// FilterBaseline (emitted actionable_* stay post-baseline).
	CurrentPreBaseline bool `json:"current_pre_baseline,omitempty"`
}

// Score holds counts for a scan (plus the deprecated Health Score for JSON).
type Score struct {
	Value    int    // 0–100 (deprecated, JSON only)
	Grade    string // A, B, C, D, or F (deprecated, JSON only)
	Errors   int
	Warnings int
	Notes    int
	Files    int
	// ParseSkipped is the number of parse-error findings (files that failed AST).
	ParseSkipped int
	// WarningsInGrade is warnings whose rule precision counts toward the grade.
	WarningsInGrade int
	// WarningsIgnored is warnings excluded from the grade (precision below floor or 0).
	WarningsIgnored int
	// Tests is the number of test functions scanned (0 when unknown).
	Tests int
	// ActionableCount is shown findings of actionable-tier rules (excluding parse-error).
	ActionableCount int
	// ProvisionalCount is shown findings of provisional-tier rules.
	ProvisionalCount int
	// ShownCount is the number of findings in the emitted set.
	ShownCount int
	// ActionableDensity is ActionableCount per 100 tests (0 when Tests == 0).
	ActionableDensity float64
	// ConfirmedCount is deprecated: equal to ActionableCount.
	ConfirmedCount int
	// ConfirmedDensity is deprecated: equal to ActionableDensity.
	ConfirmedDensity float64
	// Trend is set when comparing against a previous report (--compare / --baseline).
	Trend *Trend
	// GradeDeprecated is always true (Health Score only survives in JSON).
	GradeDeprecated bool
	// ShowAll / ShowLow record the display mode (--all / --show-low-precision) for captions.
	ShowAll bool
	ShowLow bool
	// PathRoot is the project root for vscode:// file links in HTML (display option).
	PathRoot string
}

// PrecisionWeight returns the score multiplier for a rule ID.
// Precision below MinPrecisionForGrade yields 0; unknown rules yield 0.
func PrecisionWeight(ruleID string) float64 {
	info, _ := LookupPrecision(ruleID)
	p := info.Precision
	if p < MinPrecisionForGrade {
		return 0
	}
	return p
}

// CountTier returns how many findings belong to the given rule tier
// (parse-error tool findings are never counted).
func CountTier(findings []Finding, tier string) int {
	n := 0
	for _, f := range findings {
		if f.Rule == "parse-error" {
			continue
		}
		if RuleTier(f.Rule) == tier {
			n++
		}
	}
	return n
}

// CountActionable returns findings of actionable-tier rules.
func CountActionable(findings []Finding) int { return CountTier(findings, TierActionable) }

// CountProvisional returns findings of provisional-tier rules.
func CountProvisional(findings []Finding) int { return CountTier(findings, TierProvisional) }

// densityDenom is the Health Score density denominator (files based).
func densityDenom(fileCount int) float64 {
	denom := math.Log10(float64(fileCount) + 1)
	if denom < 1 {
		denom = 1
	}
	return denom
}

// DensityPer100 returns count per 100 tests; 0 when tests <= 0.
func DensityPer100(count, tests int) float64 {
	if tests <= 0 {
		return 0
	}
	return float64(count) * 100 / float64(tests)
}

// ComputeTrend compares current actionable metrics to a previous point.
// improved: count↓ OR (count equal AND density↓);
// worsened: count↑ OR (count equal AND density↑);
// else unchanged. A negative prevDensity means "unknown" (legacy compare point):
// density is then ignored.
func ComputeTrend(currCount int, currDensity float64, prevCount int, prevDensity float64) Trend {
	t := Trend{
		PrevActionableCount:   prevCount,
		PrevActionableDensity: prevDensity,
		PrevConfirmedCount:    prevCount,
		PrevConfirmedDensity:  prevDensity,
		DeltaCount:            currCount - prevCount,
	}
	if prevDensity < 0 {
		currDensity, prevDensity = 0, 0
	} else {
		t.DeltaDensity = currDensity - prevDensity
	}
	switch {
	case currCount < prevCount || (currCount == prevCount && currDensity < prevDensity):
		t.Direction = "improved"
	case currCount > prevCount || (currCount == prevCount && currDensity > prevDensity):
		t.Direction = "worsened"
	default:
		t.Direction = "unchanged"
	}
	return t
}

// CalculateScore computes counts and the deprecated Health Score from findings
// and the number of scanned files (test count unknown → density 0).
func CalculateScore(findings []Finding, fileCount int) Score {
	return CalculateScoreWithTests(findings, fileCount, 0)
}

// CalculateScoreWithTests is CalculateScore plus the scanned test count used for
// per-100-tests actionable density.
//
// Health Score (deprecated, JSON only):
//
//	contrib  = severity_weight × precision_weight
//	           severity: error=1.0, warning=0.5, note=0 (notes are UI-only)
//	           precision: RulePrecision (0 if < MinPrecisionForGrade)
//	weighted = Σ contrib
//	density  = weighted / max(1, log10(files+1))
//	penalty  = min(80, round(density × k))
//	score    = max(0, 100 − penalty)
//	grade    = A≥90, B≥75, C≥60, D≥45, F<45
//
// Parse-error (tool) findings are counted in ParseSkipped only; they do not affect the grade.
func CalculateScoreWithTests(findings []Finding, fileCount, tests int) Score {
	var errors, warnings, notes, parseSkipped int
	var warningsInGrade, warningsIgnored int
	var weighted float64
	for _, f := range findings {
		if f.Rule == "parse-error" {
			parseSkipped++
			continue
		}
		sev := strings.ToLower(f.Severity)
		switch sev {
		case "error":
			errors++
		case "warning":
			warnings++
		default:
			notes++
		}

		pw := PrecisionWeight(f.Rule)
		var sevW float64
		switch sev {
		case "error":
			sevW = 1.0
		case "warning":
			sevW = 0.5
			if pw > 0 {
				warningsInGrade++
			} else {
				warningsIgnored++
			}
		default:
			sevW = 0 // notes: UI only
		}
		weighted += sevW * pw
	}

	denom := densityDenom(fileCount)
	wdensity := weighted / denom
	penalty := int(math.Round(wdensity * ScoreDensityK))
	if penalty > 80 {
		penalty = 80
	}

	value := 100 - penalty
	if value < 0 {
		value = 0
	}

	actionable := CountActionable(findings)
	density := DensityPer100(actionable, tests)
	return Score{
		Value:             value,
		Grade:             gradeFor(value),
		Errors:            errors,
		Warnings:          warnings,
		Notes:             notes,
		Files:             fileCount,
		ParseSkipped:      parseSkipped,
		WarningsInGrade:   warningsInGrade,
		WarningsIgnored:   warningsIgnored,
		Tests:             tests,
		ActionableCount:   actionable,
		ProvisionalCount:  CountProvisional(findings),
		ShownCount:        len(findings),
		ActionableDensity: density,
		ConfirmedCount:    actionable,
		ConfirmedDensity:  density,
		GradeDeprecated:   true,
	}
}

func gradeFor(score int) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 75:
		return "B"
	case score >= 60:
		return "C"
	case score >= 45:
		return "D"
	default:
		return "F"
	}
}

// FormatScoreLine returns the one-line text header, e.g.
//
//	testscan: 7 actionable findings (0.19 per 100 tests) · 6 provisional · 3650 tests in 382 files
//
// followed by the trend when present. The density and test count are omitted
// when the number of tests is unknown (0).
func FormatScoreLine(s Score) string {
	noun := "findings"
	if s.ActionableCount == 1 {
		noun = "finding"
	}
	line := fmt.Sprintf("testscan: %d actionable %s", s.ActionableCount, noun)
	if s.Tests > 0 {
		line += fmt.Sprintf(" (%.2f per 100 tests)", s.ActionableDensity)
	}
	line += fmt.Sprintf(" · %d provisional", s.ProvisionalCount)
	if s.Tests > 0 {
		line += fmt.Sprintf(" · %d tests in %d files", s.Tests, s.Files)
	} else {
		line += fmt.Sprintf(" · %d files", s.Files)
	}
	if s.Trend != nil {
		line += fmt.Sprintf(" · trend %s (Δ %d)", s.Trend.Direction, s.Trend.DeltaCount)
		if s.Trend.CurrentPreBaseline {
			line += " [pre-baseline]"
		}
	}
	return line
}

// DisplayOptions selects which findings are shown.
//
//	default            focus filter + actionable/provisional tiers only
//	ShowLowPrecision   focus filter + all tiers
//	All                no filtering at all
type DisplayOptions struct {
	All              bool
	ShowLowPrecision bool
}

// FilterForDisplay applies the display filters (focus, then tier) for opts.
func FilterForDisplay(findings []Finding, opts DisplayOptions) []Finding {
	if opts.All {
		return findings
	}
	findings = FilterFocus(findings)
	if !opts.ShowLowPrecision {
		findings = FilterTiers(findings)
	}
	return findings
}

// TrendCurrentMetrics returns actionable count and per-100-tests density used as
// the trend "current" side. When preBaseline is non-nil, metrics come from that
// snapshot after the same display filters as the emitted set (so baseline
// suppression cannot fake an improved trend). A nil preBaseline means use
// postScore as-is.
func TrendCurrentMetrics(postScore Score, preBaseline []Finding, opts DisplayOptions, tests int) (count int, dens float64) {
	if preBaseline == nil {
		return postScore.ActionableCount, postScore.ActionableDensity
	}
	n := CountActionable(FilterForDisplay(preBaseline, opts))
	return n, DensityPer100(n, tests)
}

// RulePrecisionValue returns the catalog precision for a rule (0 if unknown).
func RulePrecisionValue(ruleID string) float64 {
	info, _ := LookupPrecision(ruleID)
	return info.Precision
}

// FilterTiers keeps findings of actionable and provisional rules. parse-error
// tool findings are always kept.
func FilterTiers(findings []Finding) []Finding {
	out := make([]Finding, 0, len(findings))
	for _, f := range findings {
		if f.Rule == "parse-error" {
			out = append(out, f)
			continue
		}
		if RuleTier(f.Rule) == TierLow {
			continue
		}
		out = append(out, f)
	}
	return out
}
