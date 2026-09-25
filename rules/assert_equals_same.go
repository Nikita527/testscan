package rules

import (
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

type assertEqualsSame struct{}

func (assertEqualsSame) ID() string {
	return "assert-equals-same"
}

func (assertEqualsSame) NeedsAST() bool { return true }

func (assertEqualsSame) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		if useHeuristic(file, err) {
			return assertEqualsSameHeuristic(file)
		}
		return nil
	}
	return assertEqualsSameFromAST(file, model)
}

func assertEqualsSameFromAST(file scan.File, model parse.Model) []scan.Finding {
	var findings []scan.Finding
	for _, t := range model.Tests {
		q := t.QualName
		if q == "" {
			q = t.Name
		}
		for _, a := range t.Asserts {
			if a.Kind != "compare" {
				continue
			}
			if strings.TrimSpace(a.Left) == "" || strings.TrimSpace(a.Left) != strings.TrimSpace(a.Right) {
				continue
			}
			// Determinism checks: f(...) == f(...) — skip when both sides are calls.
			if a.LeftIsCall && a.RightIsCall {
				continue
			}
			findings = append(findings, scan.Finding{
				File:     file.Path,
				Line:     a.Lineno,
				Rule:     "assert-equals-same",
				Severity: "warning",
				Message:  "assert compares expression to itself",
				QualName: q,
			})
		}
	}
	return findings
}

func assertEqualsSameHeuristic(file scan.File) []scan.Finding {
	var findings []scan.Finding
	src := string(file.Content)
	for i, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "assert ") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(t, "assert "))
		if idx := strings.Index(rest, ","); idx >= 0 {
			rest = strings.TrimSpace(rest[:idx])
		}
		left, right, ok := splitEq(rest)
		if !ok {
			continue
		}
		left, right = strings.TrimSpace(left), strings.TrimSpace(right)
		if left != right {
			continue
		}
		if strings.Contains(left, "(") && strings.Contains(right, "(") {
			continue
		}
		findings = append(findings, scan.Finding{
			File:     file.Path,
			Line:     i + 1,
			Rule:     "assert-equals-same",
			Severity: "warning",
			Message:  "assert compares expression to itself",
		})
	}
	return findings
}

// splitEq splits "a == b" on the first " == "; otherwise ok=false.
func splitEq(s string) (left, right string, ok bool) {
	idx := strings.Index(s, "==")
	if idx < 0 {
		return "", "", false
	}
	left = s[:idx]
	right = s[idx+2:]
	return left, right, true
}

func NewAssertEqualsSame() scan.Rule {
	return assertEqualsSame{}
}
