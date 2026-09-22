package rules

import (
	"strings"

	"github.com/Nikita527/testscan/scan"
)

type assertTrue struct{}

func (assertTrue) ID() string {
	return "assert-true"
}

func (assertTrue) Check(file scan.File) []scan.Finding {
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
