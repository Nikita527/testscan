package rules

import (
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

type assertTrue struct{}

func (assertTrue) ID() string { return "assert-true" }

func (assertTrue) NeedsAST() bool { return true }

func (assertTrue) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		if useHeuristic(file, err) {
			return assertTrueHeuristic(file)
		}
		return nil
	}
	return assertTrueFromAST(file, model)
}

func assertTrueFromAST(file scan.File, model parse.Model) []scan.Finding {
	var findings []scan.Finding
	for _, t := range model.Tests {
		q := qualName(t)
		for _, a := range t.Asserts {
			switch a.Kind {
			case "truthy":
				if strings.TrimSpace(a.Text) != "True" {
					continue
				}
				findings = append(findings, scan.Finding{
					File:     file.Path,
					Line:     a.Lineno,
					Rule:     "assert-true",
					Severity: "warning",
					Message:  "assert True found in test",
					QualName: q,
				})
			case "unittest_bool":
				findings = append(findings, scan.Finding{
					File:     file.Path,
					Line:     a.Lineno,
					Rule:     "assert-true",
					Severity: "warning",
					Message:  "assertTrue/assertFalse with a comparison; prefer assertEqual",
					QualName: q,
				})
			}
		}
	}
	return findings
}

func assertTrueHeuristic(file scan.File) []scan.Finding {
	findings := []scan.Finding{}
	src := string(file.Content)
	if strings.Contains(src, "assert True") {
		findings = append(findings, scan.Finding{
			File:     file.Path,
			Line:     lineOf(src, "assert True"),
			Rule:     "assert-true",
			Severity: "warning",
			Message:  "assert True found in test file",
		})
	}
	return findings
}

func NewAssertTrue() scan.Rule {
	return assertTrue{}
}
