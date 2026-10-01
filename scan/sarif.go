package scan

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
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
	Name           string      `json:"name"`
	InformationURI string      `json:"informationUri"`
	Rules          []sarifRule `json:"rules,omitempty"`
}

// sarifRule is a reportingDescriptor; help carries the default fix hint.
type sarifRule struct {
	ID   string    `json:"id"`
	Help sarifHelp `json:"help"`
}

type sarifHelp struct {
	Text     string `json:"text"`
	Markdown string `json:"markdown,omitempty"`
}

type sarifResult struct {
	RuleID              string            `json:"ruleId"`
	Level               string            `json:"level"`
	Message             sarifMessage      `json:"message"`
	Locations           []sarifLocation   `json:"locations"`
	PartialFingerprints map[string]string `json:"partialFingerprints,omitempty"`
	Properties          map[string]any    `json:"properties,omitempty"`
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
		r := sarifResult{
			RuleID:  f.Rule,
			Level:   sarifLevel(f.Severity),
			Message: sarifMessage{Text: f.Message},
			Locations: []sarifLocation{{
				PhysicalLocation: sarifPhysicalLocation{
					ArtifactLocation: sarifArtifactLocation{URI: sarifURI(f.File, cwd)},
					Region:           sarifRegion{StartLine: line},
				},
			}},
		}
		if f.Fingerprint != "" {
			r.PartialFingerprints = map[string]string{
				"primaryLocationLineHash": PrimaryLocationLineHash(f),
				"testscan/v1":             f.Fingerprint,
			}
		}
		if f.QualName != "" || f.Fix != "" {
			r.Properties = map[string]any{}
			if f.QualName != "" {
				r.Properties["qualName"] = f.QualName
			}
			if f.Fix != "" {
				// No artifactChanges to offer, so the hint goes in properties
				// (SARIF "fixes" requires concrete edits).
				r.Properties["fix"] = f.Fix
			}
		}
		results = append(results, r)
	}

	seen := map[string]bool{}
	var ruleDescs []sarifRule
	for _, f := range findings {
		fix := DefaultFix(f.Rule)
		if fix == "" || seen[f.Rule] {
			continue
		}
		seen[f.Rule] = true
		ruleDescs = append(ruleDescs, sarifRule{ID: f.Rule, Help: sarifHelp{
			Text:     "Fix: " + fix,
			Markdown: "**Fix:** " + fix,
		}})
	}
	sort.Slice(ruleDescs, func(i, j int) bool { return ruleDescs[i].ID < ruleDescs[j].ID })

	report := sarifReport{
		Schema:  sarifSchema,
		Version: sarifVersion,
		Runs: []sarifRun{{
			Tool: sarifTool{Driver: sarifDriver{
				Name:           toolName,
				InformationURI: toolURI,
				Rules:          ruleDescs,
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
	return strings.ReplaceAll(filepath.ToSlash(path), "\\", "/")
}
