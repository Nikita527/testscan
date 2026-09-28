package rules

import (
	"regexp"
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

type mockTautology struct{}

func (mockTautology) ID() string { return "mock-tautology" }

func (mockTautology) NeedsAST() bool { return true }

var rePatchReturnValue = regexp.MustCompile(
	`(?i)patch(?:\.object)?\s*\(\s*["']([^"']+)["'][^)]*return_value\s*=\s*([^,\)]+)`,
)

func (mockTautology) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return nil
	}
	var findings []scan.Finding
	for _, t := range model.Tests {
		q := qualName(t)
		seen := map[int]struct{}{}

		for _, a := range t.Asserts {
			if _, dup := seen[a.Lineno]; dup {
				continue
			}
			if directMockReturnValueAssert(a) {
				seen[a.Lineno] = struct{}{}
				findings = append(findings, scan.Finding{
					File:     file.Path,
					Line:     a.Lineno,
					Rule:     "mock-tautology",
					Severity: "note",
					Message:  "asserting mock return_value is tautological",
					QualName: q,
				})
			}
		}

		// Decorator patch(... return_value=X) + assert target() == X is owned by
		// self-patched-sut to avoid duplicate findings on the same smell.

		for _, asg := range t.Assignments {
			if !strings.HasSuffix(asg.Target, ".return_value") {
				continue
			}
			mockRoot := strings.TrimSuffix(asg.Target, ".return_value")
			val := strings.TrimSpace(asg.Value)
			if mockRoot == "" || val == "" {
				continue
			}
			for _, a := range t.Asserts {
				if _, dup := seen[a.Lineno]; dup {
					continue
				}
				if a.Kind != "compare" {
					continue
				}
				if assertEchoesMockRoot(a, mockRoot, val) {
					seen[a.Lineno] = struct{}{}
					findings = append(findings, scan.Finding{
						File:     file.Path,
						Line:     a.Lineno,
						Rule:     "mock-tautology",
						Severity: "note",
						Message:  "assert echoes mock return_value assignment",
						QualName: q,
					})
				}
			}
		}
	}
	return findings
}

func directMockReturnValueAssert(a parse.Assert) bool {
	left := strings.TrimSpace(a.Left)
	right := strings.TrimSpace(a.Right)
	text := strings.TrimSpace(a.Text)
	if a.Kind == "truthy" && strings.Contains(text, ".return_value") {
		return true
	}
	if a.Kind != "compare" {
		return false
	}
	return strings.Contains(left, ".return_value") || strings.Contains(right, ".return_value")
}

func parsePatchReturnValue(decorator string) (target, value string, ok bool) {
	m := rePatchReturnValue.FindStringSubmatch(decorator)
	if len(m) < 3 {
		return "", "", false
	}
	return strings.TrimSpace(m[1]), strings.TrimSpace(m[2]), true
}

// assertEchoesMockRoot is true when LHS/RHS is the mock itself (m / m() / m.attr),
// not an unrelated SUT call that happens to equal the return_value.
func assertEchoesMockRoot(a parse.Assert, mockRoot, val string) bool {
	left := strings.TrimSpace(a.Left)
	right := strings.TrimSpace(a.Right)
	val = strings.TrimSpace(val)

	if sideIsMockValue(left, mockRoot) && right == val {
		return true
	}
	if sideIsMockValue(right, mockRoot) && left == val {
		return true
	}
	return false
}

func sideIsMockValue(side, mockRoot string) bool {
	side = strings.TrimSpace(side)
	if side == "" || mockRoot == "" {
		return false
	}
	if side == mockRoot || side == mockRoot+"()" {
		return true
	}
	if strings.HasPrefix(side, mockRoot+".") {
		rest := strings.TrimPrefix(side, mockRoot+".")
		// m.return_value / m.attr / m.attr() — direct mock access
		if rest == "return_value" || !strings.Contains(rest, "(") {
			return true
		}
		if strings.HasSuffix(rest, "()") && !strings.Contains(strings.TrimSuffix(rest, "()"), "(") {
			return true
		}
	}
	return false
}

func NewMockTautology() scan.Rule {
	return mockTautology{}
}
