package rules

import (
	"github.com/Nikita527/testscan/scan"
)

type mockTautology struct{}

func (mockTautology) ID() string { return "mock-tautology" }

func (mockTautology) NeedsAST() bool { return true }

// Check reports an assert that compares a value derived ONLY from mock
// configuration (`m.return_value = 42; assert m() == 42`, `assert m.return_value`),
// i.e. an assert that involves a configured mock and does not depend on the SUT
// (Assert.DependsOnSUT == false, no foreign call evaluated in the assert).
//
// Decorator patch(... return_value=X) + assert target() == X is owned by
// self-patched-sut to avoid duplicate findings on the same smell.
func (mockTautology) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return nil
	}
	var findings []scan.Finding
	ctx := NewSUTContext(file, model)
	for _, t := range model.Tests {
		if len(t.Asserts) == 0 {
			continue
		}
		df := AnalyzeDataflow(file, model, ctx, t)
		q := qualName(t)
		seen := map[int]struct{}{}
		for i, a := range t.Asserts {
			if !df.Tautology[i] {
				continue
			}
			if _, dup := seen[a.Lineno]; dup {
				continue
			}
			seen[a.Lineno] = struct{}{}
			findings = append(findings, scan.Finding{
				File:     file.Path,
				Line:     a.Lineno,
				Rule:     "mock-tautology",
				Severity: "note",
				Message:  "assert checks a value derived only from mock configuration (return_value), not from the SUT result",
				QualName: q,
			})
		}
	}
	return findings
}

func NewMockTautology() scan.Rule {
	return mockTautology{}
}
