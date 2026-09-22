package rules

import (
	"strings"

	"github.com/Nikita527/testscan/scan"
)

type noAssert struct{}

func (noAssert) ID() string {
	return "no-assert"
}

func (noAssert) Check(file scan.File) []scan.Finding {
	findings := []scan.Finding{}
	src := string(file.Content)
	hasAssert := strings.Contains(src, "assert") ||
		strings.Contains(src, "pytest.raises") ||
		strings.Contains(src, "pytest.warns")
	if !hasAssert {
		findings = append(findings, scan.Finding{
			File:     file.Path,
			Line:     1,
			Rule:     "no-assert",
			Severity: "error",
			Message:  "no assert found in test file",
		})
	}
	return findings
}

func NewNoAssert() scan.Rule {
	return noAssert{}
}
