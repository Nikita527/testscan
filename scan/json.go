package scan

import (
	"encoding/json"
	"io"
)

// JSONSummary is the health / counts block for --format json.
type JSONSummary struct {
	HealthScore int    `json:"health_score"`
	Grade       string `json:"grade"`
	Errors      int    `json:"errors"`
	Warnings    int    `json:"warnings"`
	Notes        int `json:"notes"`
	Files        int `json:"files"`
	ParseSkipped int `json:"parse_skipped"`
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
	out := make([]Finding, len(findings))
	copy(out, findings)
	for i := range out {
		out[i].Snippet = ""
	}
	report := JSONReport{
		Summary: JSONSummary{
			HealthScore: score.Value,
			Grade:       score.Grade,
			Errors:      score.Errors,
			Warnings:    score.Warnings,
			Notes:           score.Notes,
			Files:           score.Files,
			ParseSkipped:    score.ParseSkipped,
			WarningsInGrade: score.WarningsInGrade,
			WarningsIgnored: score.WarningsIgnored,
		},
		Findings: out,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
