package rules

import (
	"strings"

	"github.com/Nikita527/testscan/scan"
)

type emptyTest struct{}

func (emptyTest) ID() string {
	return "empty-test"
}

func (emptyTest) Check(file scan.File) []scan.Finding {
	var findings []scan.Finding
	if isVacuousTestFile(string(file.Content)) {
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

// isVacuousTestFile: пустой файл, либо только def/class + pass и/или одиночный docstring.
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
