package rules

import (
	"github.com/Nikita527/testscan/scan"
)

type nearDuplicateTest struct{}

func (nearDuplicateTest) ID() string { return "near-duplicate-test" }

func (nearDuplicateTest) NeedsAST() bool { return true }

func (nearDuplicateTest) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return nil
	}
	type seen struct {
		qual string
		line int
	}
	byNorm := map[string]seen{}
	var findings []scan.Finding
	for _, t := range model.Tests {
		norm := t.BodyNorm
		if norm == "" || t.IsEmpty {
			continue
		}
		q := qualName(t)
		if prev, ok := byNorm[norm]; ok && prev.qual != q {
			findings = append(findings, scan.Finding{
				File:     file.Path,
				Line:     t.Lineno,
				Rule:     "near-duplicate-test",
				Severity: "note",
				Message:  "test body is nearly identical to " + prev.qual,
				QualName: q,
			})
			continue
		}
		byNorm[norm] = seen{qual: q, line: t.Lineno}
	}
	return findings
}

func NewNearDuplicateTest() scan.Rule {
	return nearDuplicateTest{}
}
