package rules

import (
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

type mockOnlyAssert struct{}

func (mockOnlyAssert) ID() string {
	return "mock-only-assert"
}

func (mockOnlyAssert) NeedsAST() bool { return true }

func (mockOnlyAssert) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		if useHeuristic(file, err) {
			return mockOnlyAssertHeuristic(file)
		}
		return nil
	}
	return mockOnlyAssertFromAST(file, model)
}

func mockOnlyAssertFromAST(file scan.File, model parse.Model) []scan.Finding {
	var findings []scan.Finding
	for _, t := range model.Tests {
		if !hasMockAssert(t) {
			continue
		}
		if hasNonMockAssert(t) {
			continue
		}
		q := t.QualName
		if q == "" {
			q = t.Name
		}
		line := t.Lineno
		for _, a := range t.Asserts {
			if a.Kind == "mock_method" {
				line = a.Lineno
				break
			}
		}
		findings = append(findings, scan.Finding{
			File:     file.Path,
			Line:     line,
			Rule:     "mock-only-assert",
			Severity: "warning",
			Message:  "mock only assert found in test: " + q,
			QualName: q,
		})
	}
	return findings
}

func mockOnlyAssertHeuristic(file scan.File) []scan.Finding {
	src := string(file.Content)
	hasMock := strings.Contains(src, "assert_called") || strings.Contains(src, "assert_has_calls")
	hasAssert := strings.Contains(src, "assert ")
	if hasMock && !hasAssert {
		return []scan.Finding{{
			File:     file.Path,
			Line:     lineOfAny(src, "assert_called", "assert_has_calls"),
			Rule:     "mock-only-assert",
			Severity: "warning",
			Message:  "mock only assert found in test file",
		}}
	}
	return nil
}

func NewMockOnlyAssert() scan.Rule {
	return mockOnlyAssert{}
}
