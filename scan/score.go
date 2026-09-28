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

// RulePrecision is the measured/estimated precision for score weighting
// (from triage + post-fix estimates). Rules below MinPrecisionForGrade get weight 0.
// Rules absent from the map default to 1.0 (trusted until measured otherwise).
var RulePrecision = map[string]float64{
	"no-assert":                           0.0,
	"broad-raises":                        1.0,
	"assert-in-emptyable-loop":            0.40,
	"mock-only-assert":                    0.0,
	"mock-tautology":                      0.0,
	"weak-assert":                         0.25,
	"near-duplicate-test":                 0.38,
	"only-happy-path":                     0.50,
	"test-imports-implementation-private": 0.0,
	"parse-error":                         0.0,
	// AI-focused / newer rules — proportional until corpus-measured higher.
	"expected-recomputed": 0.60,
	"self-patched-sut":    0.55,
	"name-body-mismatch":  0.35,
	"commented-assert":    0.55,
	"overbroad-equality":  0.55,
	"assert-true":         0.90,
	"sleep-in-test":       0.50,
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
}

// PrecisionWeight returns the score multiplier for a rule ID.
// Precision below MinPrecisionForGrade yields 0; unknown rules yield 1.
func PrecisionWeight(ruleID string) float64 {
	p, ok := RulePrecision[ruleID]
	if !ok {
		return 1.0
	}
	if p < MinPrecisionForGrade {
		return 0
	}
	return p
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

	denom := math.Log10(float64(fileCount) + 1)
	if denom < 1 {
		denom = 1
	}
	density := weighted / denom
	penalty := int(math.Round(density * ScoreDensityK))
	if penalty > 80 {
		penalty = 80
	}

	value := 100 - penalty
	if value < 0 {
		value = 0
	}

	return Score{
		Value:           value,
		Grade:           gradeFor(value),
		Errors:          errors,
		Warnings:        warnings,
		Notes:           notes,
		Files:           fileCount,
		ParseSkipped:    parseSkipped,
		WarningsInGrade: warningsInGrade,
		WarningsIgnored: warningsIgnored,
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

// FormatScoreLine returns the text summary line, e.g. "Health Score: 72 (C)".
func FormatScoreLine(s Score) string {
	return fmt.Sprintf("Health Score: %d (%s)", s.Value, s.Grade)
}
