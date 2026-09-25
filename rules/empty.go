package rules

import (
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

type emptyTest struct{}

func (emptyTest) ID() string {
	return "empty-test"
}

func (emptyTest) NeedsAST() bool { return true }

func (emptyTest) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		if useHeuristic(file, err) {
			return emptyTestHeuristic(file)
		}
		return nil
	}
	return emptyTestFromAST(file, model)
}

func emptyTestFromAST(file scan.File, model parse.Model) []scan.Finding {
	var findings []scan.Finding
	for _, t := range model.Tests {
		if !t.IsEmpty {
			continue
		}
		q := t.QualName
		if q == "" {
			q = t.Name
		}
		findings = append(findings, scan.Finding{
			File:     file.Path,
			Line:     t.Lineno,
			Rule:     "empty-test",
			Severity: "error",
			Message:  "empty test function: " + q,
			QualName: q,
		})
	}
	if len(model.Tests) == 0 && isVacuousTestFile(string(file.Content)) {
		findings = append(findings, scan.Finding{
			File:     file.Path,
			Line:     1,
			Rule:     "empty-test",
			Severity: "error",
			Message:  "test file is empty (or only pass/docstring)",
		})
	}
	return findings
}

func emptyTestHeuristic(file scan.File) []scan.Finding {
	lines := strings.Split(string(file.Content), "\n")
	var findings []scan.Finding
	for i, line := range lines {
		name, ok := testDefName(line)
		if !ok {
			continue
		}
		if !bodyVacuous(lines, i) {
			continue
		}
		findings = append(findings, scan.Finding{
			File:     file.Path,
			Line:     i + 1,
			Rule:     "empty-test",
			Severity: "error",
			Message:  "empty test function: " + name,
			QualName: name,
		})
	}
	if len(findings) == 0 && isVacuousTestFile(string(file.Content)) {
		return []scan.Finding{{
			File:     file.Path,
			Line:     1,
			Rule:     "empty-test",
			Severity: "error",
			Message:  "test file is empty (or only pass/docstring)",
		}}
	}
	return findings
}

// bodyVacuous: body until next def/class at same/less indent is only pass/docstring.
func bodyVacuous(lines []string, defIdx int) bool {
	defIndent := leadingSpaces(lines[defIdx])
	for j := defIdx + 1; j < len(lines); j++ {
		raw := lines[j]
		t := strings.TrimSpace(raw)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		ind := leadingSpaces(raw)
		if ind <= defIndent && (strings.HasPrefix(t, "def ") || strings.HasPrefix(t, "async def ") || strings.HasPrefix(t, "class ")) {
			break
		}
		if t == "pass" || isLoneStringLiteral(t) {
			continue
		}
		return false
	}
	return true
}

func leadingSpaces(s string) int {
	n := 0
	for _, r := range s {
		if r == ' ' {
			n++
			continue
		}
		if r == '\t' {
			n += 4
			continue
		}
		break
	}
	return n
}

// isVacuousTestFile: empty file, or only def/class + pass and/or a lone docstring.
func isVacuousTestFile(src string) bool {
	s := strings.TrimSpace(src)
	if s == "" {
		return true
	}
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if strings.HasPrefix(t, "def ") || strings.HasPrefix(t, "async def ") || strings.HasPrefix(t, "class ") {
			continue
		}
		if t == "pass" || isLoneStringLiteral(t) {
			continue
		}
		return false
	}
	return true
}

func isLoneStringLiteral(t string) bool {
	for _, q := range []string{`"""`, `'''`, `"`, `'`} {
		if len(t) >= 2*len(q) && strings.HasPrefix(t, q) && strings.HasSuffix(t, q) {
			return true
		}
	}
	return false
}

func NewEmptyTest() scan.Rule {
	return emptyTest{}
}
