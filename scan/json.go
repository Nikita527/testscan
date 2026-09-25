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
	Notes       int    `json:"notes"`
	Files       int    `json:"files"`
}

// JSONReport is the --format json document (breaking vs bare findings array).
type JSONReport struct {
	Summary  JSONSummary `json:"summary"`
	Findings []Finding   `json:"findings"`
}

// WriteJSON encodes findings with a Health Score summary wrapper.
func WriteJSON(w io.Writer, findings []Finding, score Score) error {
	if findings == nil {
		findings = []Finding{}
	}
	report := JSONReport{
		Summary: JSONSummary{
			HealthScore: score.Value,
			Grade:       score.Grade,
			Errors:      score.Errors,
			Warnings:    score.Warnings,
			Notes:       score.Notes,
			Files:       score.Files,
		},
		Findings: findings,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
