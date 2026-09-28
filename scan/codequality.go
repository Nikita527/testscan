package scan

import (
	"encoding/json"
	"io"
	"strings"
)

// codeQualityIssue is one GitLab Code Quality / Code Climate engine issue.
// See https://docs.gitlab.com/ee/ci/testing/code_quality.html#code-quality-report-format
type codeQualityIssue struct {
	Description string              `json:"description"`
	CheckName   string              `json:"check_name"`
	Fingerprint string              `json:"fingerprint"`
	Severity    string              `json:"severity"`
	Location    codeQualityLocation `json:"location"`
}

type codeQualityLocation struct {
	Path  string           `json:"path"`
	Lines codeQualityLines `json:"lines"`
}

type codeQualityLines struct {
	Begin int `json:"begin"`
}

// WriteCodeQuality encodes findings as a GitLab Code Quality report (Code Climate JSON array).
func WriteCodeQuality(w io.Writer, findings []Finding) error {
	issues := make([]codeQualityIssue, 0, len(findings))
	for _, f := range findings {
		line := f.Line
		if line < 1 {
			line = 1
		}
		fp := f.Fingerprint
		if fp == "" {
			fp = PrimaryLocationLineHash(f)
		}
		issues = append(issues, codeQualityIssue{
			Description: f.Message,
			CheckName:   f.Rule,
			Fingerprint: fp,
			Severity:    codeQualitySeverity(f.Severity),
			Location: codeQualityLocation{
				Path:  filepathToSlash(f.File),
				Lines: codeQualityLines{Begin: line},
			},
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(issues)
}

func codeQualitySeverity(sev string) string {
	switch strings.ToLower(sev) {
	case "error":
		return "major"
	case "warning":
		return "minor"
	default:
		return "info"
	}
}

func filepathToSlash(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}
