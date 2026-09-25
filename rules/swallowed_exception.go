package rules

import (
	"github.com/Nikita527/testscan/scan"
)

type swallowedException struct{}

func (swallowedException) ID() string { return "swallowed-exception" }

func (swallowedException) NeedsAST() bool { return true }

func (swallowedException) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return nil
	}
	var findings []scan.Finding
	for _, t := range model.Tests {
		q := qualName(t)
		for _, te := range t.TryExcept {
			if te.HasRaise {
				continue
			}
			if !te.Bare && !te.CatchesException {
				continue
			}
			findings = append(findings, scan.Finding{
				File:     file.Path,
				Line:     te.Lineno,
				Rule:     "swallowed-exception",
				Severity: "warning",
				Message:  "bare except / except Exception swallows errors without re-raise",
				QualName: q,
			})
		}
	}
	return findings
}

func NewSwallowedException() scan.Rule {
	return swallowedException{}
}
