package rules

import (
	"github.com/Nikita527/testscan/scan"
)

type assertInEmptyableLoop struct{}

func (assertInEmptyableLoop) ID() string { return "assert-in-emptyable-loop" }

func (assertInEmptyableLoop) NeedsAST() bool { return true }

func (assertInEmptyableLoop) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return nil
	}
	var findings []scan.Finding
	for _, t := range model.Tests {
		if len(t.Asserts) == 0 {
			continue
		}
		onlyFors := make([]struct{ start, end int }, 0)
		for _, fl := range t.ForLoops {
			if !fl.OnlyAsserts {
				continue
			}
			end := fl.EndLineno
			if end < fl.Lineno {
				end = fl.Lineno
			}
			onlyFors = append(onlyFors, struct{ start, end int }{fl.Lineno, end})
		}
		if len(onlyFors) == 0 {
			continue
		}
		allInside := true
		firstLine := t.Asserts[0].Lineno
		for _, a := range t.Asserts {
			inside := false
			for _, fl := range onlyFors {
				if a.Lineno >= fl.start && a.Lineno <= fl.end {
					inside = true
					break
				}
			}
			if !inside {
				allInside = false
				break
			}
			if a.Lineno < firstLine {
				firstLine = a.Lineno
			}
		}
		if !allInside {
			continue
		}
		findings = append(findings, scan.Finding{
			File:     file.Path,
			Line:     firstLine,
			Rule:     "assert-in-emptyable-loop",
			Severity: "warning",
			Message:  "only asserts are inside a for-loop over an emptyable collection",
			QualName: qualName(t),
		})
	}
	return findings
}

func NewAssertInEmptyableLoop() scan.Rule {
	return assertInEmptyableLoop{}
}
