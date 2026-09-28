package rules

import (
	"regexp"
	"strings"

	"github.com/Nikita527/testscan/scan"
)

type commentedAssert struct{}

func (commentedAssert) ID() string { return "commented-assert" }

func (commentedAssert) NeedsAST() bool { return false }

var reCommentedAssert = regexp.MustCompile(`(?i)^\s*#\s*(assert\b|self\.assert)`)

func (commentedAssert) Check(file scan.File) []scan.Finding {
	src := string(file.Content)
	var findings []scan.Finding
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		if !reCommentedAssert.MatchString(line) {
			continue
		}
		findings = append(findings, scan.Finding{
			File:     file.Path,
			Line:     i + 1,
			Rule:     "commented-assert",
			Severity: "note",
			Message:  "commented-out assert; remove or restore the check",
		})
	}
	return findings
}

func NewCommentedAssert() scan.Rule {
	return commentedAssert{}
}
