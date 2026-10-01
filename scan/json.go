package scan

import (
	"encoding/json"
	"io"
)

// JSONSummary is the health / counts block for --format json.
type JSONSummary struct {
	HealthScore int    `json:"health_score"`
	Grade       string `json:"grade"`
	// GradeDeprecated is always true; health_score/grade are removed from text and
	// HTML and will be dropped from JSON in a future release.
	GradeDeprecated bool `json:"grade_deprecated"`
	// ActionableCount / ActionableDensity (per 100 tests) are the primary metrics.
	ActionableCount  int     `json:"actionable_count"`
	ActionablePer100 float64 `json:"actionable_per_100_tests"`
	ProvisionalCount int     `json:"provisional_count"`
	ShownCount       int     `json:"shown_count"`
	// HiddenCount is the number of findings hidden by the default display filters.
	HiddenCount int `json:"hidden_count"`
	Tests       int `json:"tests"`
	// Deprecated: ConfirmedCount equals actionable_count, ConfirmedDensity equals
	// actionable_per_100_tests. Kept for one release; will be removed.
	ConfirmedCount   int     `json:"confirmed_count"`
	ConfirmedDensity float64 `json:"confirmed_density"`
	Trend            *Trend  `json:"trend,omitempty"`
	Errors           int     `json:"errors"`
	Warnings         int     `json:"warnings"`
	Notes            int     `json:"notes"`
	Files            int     `json:"files"`
	ParseSkipped     int     `json:"parse_skipped"`
	// WarningsInGrade / WarningsIgnored explain Health Score vs raw warning count.
	WarningsInGrade int `json:"warnings_in_grade"`
	WarningsIgnored int `json:"warnings_ignored"`
}

// JSONReport is the --format json document (breaking vs bare findings array).
type JSONReport struct {
	Summary  JSONSummary `json:"summary"`
	Findings []Finding   `json:"findings"`
}

// WriteJSON encodes findings with a Health Score summary wrapper.
// Snippets are omitted from JSON to keep CI artifacts small (HTML still includes them).
func WriteJSON(w io.Writer, findings []Finding, score Score) error {
	if findings == nil {
		findings = []Finding{}
	}
	out := AnnotateTiers(findings)
	for i := range out {
		out[i].Snippet = ""
	}
	report := JSONReport{
		Summary: JSONSummary{
			HealthScore:      score.Value,
			Grade:            score.Grade,
			GradeDeprecated:  true,
			ActionableCount:  score.ActionableCount,
			ActionablePer100: score.ActionableDensity,
			ProvisionalCount: score.ProvisionalCount,
			ShownCount:       score.ShownCount,
			HiddenCount:      score.HiddenCount,
			Tests:            score.Tests,
			ConfirmedCount:   score.ActionableCount,
			ConfirmedDensity: score.ActionableDensity,
			Trend:            score.Trend,
			Errors:           score.Errors,
			Warnings:         score.Warnings,
			Notes:            score.Notes,
			Files:            score.Files,
			ParseSkipped:     score.ParseSkipped,
			WarningsInGrade:  score.WarningsInGrade,
			WarningsIgnored:  score.WarningsIgnored,
		},
		Findings: out,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
