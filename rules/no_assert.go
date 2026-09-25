package rules

import (
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

type noAssert struct{}

func (noAssert) ID() string {
	return "no-assert"
}

func (noAssert) NeedsAST() bool { return true }

func (noAssert) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		if useHeuristic(file, err) {
			return noAssertHeuristic(file)
		}
		return nil
	}
	return noAssertFromAST(file, model)
}

func noAssertFromAST(file scan.File, model parse.Model) []scan.Finding {
	var findings []scan.Finding
	for _, t := range model.Tests {
		if t.IsEmpty {
			continue
		}
		if testHasAssertOrHelper(t, file.AssertHelpers) {
			continue
		}
		q := t.QualName
		if q == "" {
			q = t.Name
		}
		findings = append(findings, scan.Finding{
			File:     file.Path,
			Line:     t.Lineno,
			Rule:     "no-assert",
			Severity: "error",
			Message:  "no assert found in test: " + q,
			QualName: q,
		})
	}
	return findings
}

func noAssertHeuristic(file scan.File) []scan.Finding {
	src := string(file.Content)
	if strings.Contains(src, "assert") ||
		strings.Contains(src, "pytest.raises") ||
		strings.Contains(src, "pytest.warns") {
		return nil
	}
	return []scan.Finding{{
		File:     file.Path,
		Line:     1,
		Rule:     "no-assert",
		Severity: "error",
		Message:  "no assert found in test file",
	}}
}

func NewNoAssert() scan.Rule {
	return noAssert{}
}
