package rules

import (
	"strings"

	"github.com/Nikita527/testscan/scan"
)

type skipWithoutReason struct{}

func (skipWithoutReason) ID() string { return "skip-without-reason" }

func (skipWithoutReason) NeedsAST() bool { return true }

func (skipWithoutReason) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return nil
	}
	var findings []scan.Finding
	for _, t := range model.Tests {
		q := qualName(t)
		for _, d := range t.Decorators {
			if !isSkipWithoutReason(d) {
				continue
			}
			findings = append(findings, scan.Finding{
				File:     file.Path,
				Line:     t.Lineno,
				Rule:     "skip-without-reason",
				Severity: "warning",
				Message:  "skip/xfail without reason or strict",
				QualName: q,
			})
			break
		}
	}
	return findings
}

func isSkipWithoutReason(dec string) bool {
	d := strings.TrimSpace(dec)
	if d == "" {
		return false
	}
	// Bare markers.
	switch d {
	case "pytest.mark.skip", "pytest.mark.xfail", "unittest.skip", "unittest.expectedFailure":
		return true
	}
	// Calls: pytest.mark.skip(...), pytest.mark.xfail(...)
	base := d
	args := ""
	if i := strings.Index(d, "("); i >= 0 {
		base = strings.TrimSpace(d[:i])
		args = d[i:]
	}
	switch base {
	case "pytest.mark.skip", "pytest.mark.xfail", "unittest.skip":
		// Has reason= or strict= or a positional string arg → clean.
		if strings.Contains(args, "reason") || strings.Contains(args, "strict") {
			return false
		}
		inner := strings.TrimSuffix(strings.TrimPrefix(args, "("), ")")
		inner = strings.TrimSpace(inner)
		if inner == "" {
			return true
		}
		// Positional string reason: ("msg") or ('msg')
		if strings.HasPrefix(inner, "\"") || strings.HasPrefix(inner, "'") {
			return false
		}
		return true
	}
	return false
}

func NewSkipWithoutReason() scan.Rule {
	return skipWithoutReason{}
}
