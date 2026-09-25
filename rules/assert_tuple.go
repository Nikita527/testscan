package rules

import (
	"github.com/Nikita527/testscan/scan"
)

type assertTuple struct{}

func (assertTuple) ID() string { return "assert-tuple" }

func (assertTuple) NeedsAST() bool { return true }

func (assertTuple) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return nil
	}
	var findings []scan.Finding
	for _, t := range model.Tests {
		q := qualName(t)
		for _, a := range t.Asserts {
			if a.Kind != "tuple" {
				continue
			}
			findings = append(findings, scan.Finding{
				File:     file.Path,
				Line:     a.Lineno,
				Rule:     "assert-tuple",
				Severity: "warning",
				Message:  "assert on a tuple is always True; did you mean a comparison?",
				QualName: q,
			})
		}
	}
	return findings
}

func NewAssertTuple() scan.Rule {
	return assertTuple{}
}
