package rules

import (
	"strings"

	"github.com/Nikita527/testscan/scan"
)

type mockTautology struct{}

func (mockTautology) ID() string { return "mock-tautology" }

func (mockTautology) NeedsAST() bool { return true }

func (mockTautology) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return nil
	}
	var findings []scan.Finding
	for _, t := range model.Tests {
		q := qualName(t)
		if len(t.Assignments) == 0 {
			continue
		}
		seen := map[int]struct{}{}
		for _, asg := range t.Assignments {
			val := strings.TrimSpace(asg.Value)
			if val == "" {
				continue
			}
			for _, a := range t.Asserts {
				if _, dup := seen[a.Lineno]; dup {
					continue
				}
				if a.Kind == "truthy" && strings.Contains(a.Text, "return_value") {
					seen[a.Lineno] = struct{}{}
					findings = append(findings, scan.Finding{
						File:     file.Path,
						Line:     a.Lineno,
						Rule:     "mock-tautology",
						Severity: "warning",
						Message:  "asserting mock return_value is tautological",
						QualName: q,
					})
					continue
				}
				if a.Kind != "compare" {
					continue
				}
				right := strings.TrimSpace(a.Right)
				left := strings.TrimSpace(a.Left)
				if strings.Contains(left, "return_value") || strings.Contains(right, "return_value") {
					seen[a.Lineno] = struct{}{}
					findings = append(findings, scan.Finding{
						File:     file.Path,
						Line:     a.Lineno,
						Rule:     "mock-tautology",
						Severity: "warning",
						Message:  "asserting mock return_value is tautological",
						QualName: q,
					})
					continue
				}
				// Echo only when one side is a call (e.g. sut() == X) matching return_value.
				if (a.LeftIsCall && right == val) || (a.RightIsCall && left == val) {
					seen[a.Lineno] = struct{}{}
					findings = append(findings, scan.Finding{
						File:     file.Path,
						Line:     a.Lineno,
						Rule:     "mock-tautology",
						Severity: "warning",
						Message:  "assert echoes mock return_value assignment",
						QualName: q,
					})
				}
			}
		}
	}
	return findings
}

func NewMockTautology() scan.Rule {
	return mockTautology{}
}
