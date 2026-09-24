package scan

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	sarifVersion = "2.1.0"
	sarifSchema  = "https://json.schemastore.org/sarif-2.1.0.json"
	toolName     = "testscan"
	toolURI      = "https://github.com/Nikita527/testscan"
)

// SARIF report (subset of SARIF 2.1.0 used by GitHub Code Scanning and similar).
type sarifReport struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string `json:"name"`
	InformationURI string `json:"informationUri"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           sarifRegion           `json:"region"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine int `json:"startLine"`
}

// WriteSARIF encodes findings as a SARIF 2.1.0 report.
func WriteSARIF(w io.Writer, findings []Finding) error {
	cwd, _ := os.Getwd() // best-effort; relative URIs help GitHub Code Scanning
	results := make([]sarifResult, 0, len(findings))
	for _, f := range findings {
		line := f.Line
		if line < 1 {
			line = 1 // SARIF requires startLine >= 1
		}
		results = append(results, sarifResult{
			RuleID:  f.Rule,
			Level:   sarifLevel(f.Severity),
			Message: sarifMessage{Text: f.Message},
			Locations: []sarifLocation{{
				PhysicalLocation: sarifPhysicalLocation{
					ArtifactLocation: sarifArtifactLocation{URI: sarifURI(f.File, cwd)},
					Region:           sarifRegion{StartLine: line},
				},
			}},
		})
	}

	report := sarifReport{
		Schema:  sarifSchema,
		Version: sarifVersion,
		Runs: []sarifRun{{
			Tool: sarifTool{Driver: sarifDriver{
				Name:           toolName,
				InformationURI: toolURI,
			}},
			Results: results,
		}},
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func sarifLevel(severity string) string {
	switch strings.ToLower(severity) {
	case "error":
		return "error"
	case "warning":
		return "warning"
	default:
		return "note"
	}
}

// sarifURI uses forward slashes (SARIF / URI convention).
// Absolute paths under cwd become repo-relative so Code Scanning can annotate.
func sarifURI(path, cwd string) string {
	if cwd != "" && filepath.IsAbs(path) {
		if rel, err := filepath.Rel(cwd, path); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			path = rel
		}
	}
	return filepath.ToSlash(path)
}
