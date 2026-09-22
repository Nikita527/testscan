package rules

import (
	"strings"

	"github.com/Nikita527/testscan/scan"
)

type mockOnlyAssert struct{}

func (mockOnlyAssert) ID() string {
	return "mock-only-assert"
}

func (mockOnlyAssert) Check(file scan.File) []scan.Finding {
	findings := []scan.Finding{}
	src := string(file.Content)
	hasMock := strings.Contains(src, "assert_called") ||
		strings.Contains(src, "assert_has_calls")
	hasAssert := strings.Contains(src, "assert ")

	if hasMock && !hasAssert {
		findings = append(findings, scan.Finding{
			File:     file.Path,
			Line:     lineOfAny(src, "assert_called", "assert_has_calls"),
			Rule:     "mock-only-assert",
			Severity: "warning",
			Message:  "mock only assert found in test file",
		})
	}
	return findings
}

func NewMockOnlyAssert() scan.Rule {
	return mockOnlyAssert{}
}
