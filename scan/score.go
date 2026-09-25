package scan

import (
	"fmt"
	"math"
	"strings"
)

// ScoreDensityK scales finding density into a 0–80 penalty.
// Calibrated so note-heavy suites (weak-assert / private-import / near-duplicate /
// only-happy-path) on a large repo land around B–C after rule accuracy fixes;
// a note-only only-happy-path suite alone stays well above D.
const ScoreDensityK = 2.0

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
}

// CalculateScore computes Health Score from findings and the number of scanned files.
//
//	weighted = errors×1.0 + warnings×0.5 + notes×0.2
//	density  = weighted / max(1, log10(files+1))
//	penalty  = min(80, round(density × k))
//	parse_penalty = max(0, round(skipped/total × 20))  when total > 0
//	score    = max(0, 100 − penalty − parse_penalty)
//	grade    = A≥90, B≥75, C≥60, D≥45, F<45
func CalculateScore(findings []Finding, fileCount int) Score {
	var errors, warnings, notes, parseSkipped int
	for _, f := range findings {
		if f.Rule == "parse-error" {
			parseSkipped++
			// Counted via parse_penalty only — avoid double-counting as notes.
			continue
		}
		switch strings.ToLower(f.Severity) {
		case "error":
			errors++
		case "warning":
			warnings++
		default:
			notes++
		}
	}

	weighted := float64(errors)*1.0 + float64(warnings)*0.5 + float64(notes)*0.2
	denom := math.Log10(float64(fileCount) + 1)
	if denom < 1 {
		denom = 1
	}
	density := weighted / denom
	penalty := int(math.Round(density * ScoreDensityK))
	if penalty > 80 {
		penalty = 80
	}

	parsePenalty := 0
	if fileCount > 0 && parseSkipped > 0 {
		parsePenalty = int(math.Round(float64(parseSkipped) / float64(fileCount) * 20))
	}

	value := 100 - penalty - parsePenalty
	if value < 0 {
		value = 0
	}

	return Score{
		Value:        value,
		Grade:        gradeFor(value),
		Errors:       errors,
		Warnings:     warnings,
		Notes:        notes,
		Files:        fileCount,
		ParseSkipped: parseSkipped,
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
