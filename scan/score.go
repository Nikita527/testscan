package scan

import (
	"fmt"
	"math"
	"strings"
)

// ScoreDensityK scales finding density into a 0–80 penalty.
// Calibrated so a modest number of high-precision error/warning findings
// on a large repo lands around B–C; notes and near-zero-precision rules
// do not drag the grade alone.
const ScoreDensityK = 2.0

// MinPrecisionForGrade is the floor below which a rule contributes 0 to Health Score.
// Mid-precision rules (e.g. 0.35–0.5) still count proportionally — only near-zero
// / known-broken detectors are fully ignored.
const MinPrecisionForGrade = 0.15

// MinPrecisionForDisplay is the floor below which findings are omitted from emit
// by default (use --show-low-precision to restore). Confirmed metrics use this floor.
const MinPrecisionForDisplay = 0.3

// PrecisionInfo describes the precision of one rule in the catalog.
// Source is "measured" (from labelled findings; N and WilsonLow are set) or
// "estimated" (triage / post-fix guess; N is 0, WilsonLow may be 0).
type PrecisionInfo struct {
	Precision float64
	N         int
	Source    string // "measured" | "estimated"
	WilsonLow float64
}

// Precision sources.
const (
	SourceMeasured  = "measured"
	SourceEstimated = "estimated"
)

func est(p float64) PrecisionInfo {
	return PrecisionInfo{Precision: p, Source: SourceEstimated}
}

// RulePrecision is the measured/estimated precision for score weighting
// (from triage + post-fix estimates). Rules below MinPrecisionForGrade get weight 0.
// Rules ABSENT from the map are unknown: precision 0, estimated — they are not
// confirmed and carry no weight until added here (see LookupPrecision).
var RulePrecision = map[string]PrecisionInfo{
	"no-assert":                           est(0.0),
	"broad-raises":                        est(1.0),
	"assert-in-emptyable-loop":            est(0.40),
	"mock-only-assert":                    est(0.0),
	"mock-tautology":                      est(0.0),
	"weak-assert":                         est(0.25),
	"near-duplicate-test":                 est(0.38),
	"only-happy-path":                     est(0.50),
	"test-imports-implementation-private": est(0.0),
	"parse-error":                         est(0.0),
	// AI-focused / newer rules — proportional until corpus-measured higher.
	"expected-recomputed": est(0.60),
	"self-patched-sut":    est(0.55),
	"name-body-mismatch":  est(0.35),
	"commented-assert":    est(0.55),
	"overbroad-equality":  est(0.55),
	"assert-true":         est(0.90),
	"sleep-in-test":       est(1.0),
	"wall-clock-in-test":  est(0.50),
	// Opt-in project rules — conservative until corpus-measured.
	"error-contract-assert": est(0.55),
	"raises-without-check":  est(0.50),
	"missing-mirror-test":   est(0.70),
	"rbac-mutation-guard":   est(0.45),
	// Rules that were previously uncatalogued and implicitly trusted at 1.0.
	// Listed explicitly (estimated) so behaviour is unchanged; unknown IDs now get 0.
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

// Trend compares the current confirmed metrics to a previous compare point.
type Trend struct {
	Direction            string  `json:"direction"` // "improved"|"worsened"|"unchanged"
	PrevConfirmedCount   int     `json:"prev_confirmed_count"`
	PrevConfirmedDensity float64 `json:"prev_confirmed_density"`
	DeltaCount           int     `json:"delta_count"`   // current - prev
	DeltaDensity         float64 `json:"delta_density"` // current - prev
	// CurrentPreBaseline is true when the trend "current" side was computed before
	// FilterBaseline (emitted confirmed_* stay post-baseline).
	CurrentPreBaseline bool `json:"current_pre_baseline,omitempty"`
}

// Score is the Health Score (0–100) and severity breakdown for a scan.
type Score struct {
	Value    int    // 0–100
	Grade    string // A, B, C, D, or F
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
	// ConfirmedCount is findings with precision ≥ MinPrecisionForDisplay, excluding parse-error.
	ConfirmedCount int
	// ConfirmedDensity is ConfirmedCount / densityDenom(Files).
	ConfirmedDensity float64
	// Trend is set when comparing against a previous report (--compare / --baseline).
	Trend *Trend
	// GradeDeprecated is always true for emit (Health Score is secondary to confirmed metrics).
	GradeDeprecated bool
	// ShowGrade enables the deprecated Health Score line / full ring in text and HTML.
	ShowGrade bool
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

// CountConfirmed returns how many findings qualify as confirmed
// (precision ≥ MinPrecisionForDisplay and not parse-error).
func CountConfirmed(findings []Finding) int {
	n := 0
	for _, f := range findings {
		if f.Rule == "parse-error" {
			continue
		}
		if RulePrecisionValue(f.Rule) >= MinPrecisionForDisplay {
			n++
		}
	}
	return n
}

// densityDenom is shared by Health Score density and ConfirmedDensity.
func densityDenom(fileCount int) float64 {
	denom := math.Log10(float64(fileCount) + 1)
	if denom < 1 {
		denom = 1
	}
	return denom
}

// ConfirmedDensity returns confirmed_count / max(1, log10(files+1)).
func ConfirmedDensity(count, fileCount int) float64 {
	return float64(count) / densityDenom(fileCount)
}

// ComputeTrend compares current confirmed metrics to a previous point.
// improved: count↓ OR (count equal AND density↓);
// worsened: count↑ OR (count equal AND density↑);
// else unchanged.
func ComputeTrend(currCount int, currDensity float64, prevCount int, prevDensity float64) Trend {
	t := Trend{
		PrevConfirmedCount:   prevCount,
		PrevConfirmedDensity: prevDensity,
		DeltaCount:           currCount - prevCount,
		DeltaDensity:         currDensity - prevDensity,
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

// CalculateScore computes Health Score from findings and the number of scanned files.
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
func CalculateScore(findings []Finding, fileCount int) Score {
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
	density := weighted / denom
	penalty := int(math.Round(density * ScoreDensityK))
	if penalty > 80 {
		penalty = 80
	}

	value := 100 - penalty
	if value < 0 {
		value = 0
	}

	confirmed := CountConfirmed(findings)
	return Score{
		Value:            value,
		Grade:            gradeFor(value),
		Errors:           errors,
		Warnings:         warnings,
		Notes:            notes,
		Files:            fileCount,
		ParseSkipped:     parseSkipped,
		WarningsInGrade:  warningsInGrade,
		WarningsIgnored:  warningsIgnored,
		ConfirmedCount:   confirmed,
		ConfirmedDensity: ConfirmedDensity(confirmed, fileCount),
		GradeDeprecated:  true,
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

// FormatScoreLine returns the text summary. Primary line is confirmed metrics;
// with ShowGrade, a deprecated Health Score line is appended.
func FormatScoreLine(s Score) string {
	line := fmt.Sprintf("Confirmed: %d (density %.2f)", s.ConfirmedCount, s.ConfirmedDensity)
	if s.Trend != nil {
		line += fmt.Sprintf(" · trend %s (Δ %d)", s.Trend.Direction, s.Trend.DeltaCount)
		if s.Trend.CurrentPreBaseline {
			line += " [pre-baseline]"
		}
	}
	if s.ShowGrade {
		line += fmt.Sprintf("\nHealth Score: %d (%s) [deprecated]", s.Value, s.Grade)
	}
	return line
}

// TrendCurrentMetrics returns confirmed count/density used as the trend "current" side.
// When preBaseline is non-nil, metrics come from that snapshot after the same
// focus / low-precision filters as the emitted set (so baseline suppression cannot
// fake an improved trend). A nil preBaseline means use postScore as-is.
func TrendCurrentMetrics(postScore Score, preBaseline []Finding, focus, filterLowPrecision bool, fileCount int) (count int, dens float64) {
	if preBaseline == nil {
		return postScore.ConfirmedCount, postScore.ConfirmedDensity
	}
	tf := preBaseline
	if focus {
		tf = FilterFocus(tf)
	}
	if filterLowPrecision {
		tf = FilterLowPrecision(tf)
	}
	pre := CalculateScore(tf, fileCount)
	return pre.ConfirmedCount, pre.ConfirmedDensity
}

// RulePrecisionValue returns the catalog precision for a rule (0 if unknown).
func RulePrecisionValue(ruleID string) float64 {
	info, _ := LookupPrecision(ruleID)
	return info.Precision
}

// FilterLowPrecision drops findings whose rule precision is below MinPrecisionForDisplay.
// parse-error tool findings are always kept.
func FilterLowPrecision(findings []Finding) []Finding {
	out := make([]Finding, 0, len(findings))
	for _, f := range findings {
		if f.Rule == "parse-error" {
			out = append(out, f)
			continue
		}
		if RulePrecisionValue(f.Rule) < MinPrecisionForDisplay {
			continue
		}
		out = append(out, f)
	}
	return out
}
