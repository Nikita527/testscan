package rules

import (
	"strings"

	"github.com/Nikita527/testscan/scan"
)

type todoTest struct{}

func (todoTest) ID() string {
	return "todo-test"
}

func (todoTest) Check(file scan.File) []scan.Finding {
	var findings []scan.Finding
	src := string(file.Content)

	markers := []string{
		"pytest.skip",
		"unittest.skip",
		`pytest.fail("TODO")`,
		`pytest.fail('TODO')`,
		`assert False, "TODO"`,
		`assert False, 'TODO'`,
	}
	hasTodo := false
	for _, m := range markers {
		if strings.Contains(src, m) {
			hasTodo = true
			break
		}
	}

	if hasTodo {
		findings = append(findings, scan.Finding{
			File:     file.Path,
			Line:     lineOfAny(src, markers...),
			Rule:     "todo-test",
			Severity: "warning",
			Message:  "todo/skip marker found in test file",
		})
	}
	return findings
}

func NewTodoTest() scan.Rule {
	return todoTest{}
}
