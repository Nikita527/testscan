package rules

import (
	"strings"

	"github.com/Nikita527/testscan/scan"
)

type onlyHappyPath struct{}

func (onlyHappyPath) ID() string {
	return "only-happy-path"
}

func (onlyHappyPath) Check(file scan.File) []scan.Finding {
	src := string(file.Content)
	n := countTestFuncs(src)
	if n <= 3 {
		return nil
	}
	hasNeg := strings.Contains(src, "pytest.raises") ||
		strings.Contains(src, "pytest.warns") ||
		strings.Contains(src, "assertRaises")
	if hasNeg {
		return nil
	}
	return []scan.Finding{{
		File:     file.Path,
		Line:     1,
		Rule:     "only-happy-path",
		Severity: "warning",
		Message:  "more than 3 tests and no negative-path check (raises/warns/assertRaises)",
	}}
}

func countTestFuncs(src string) int {
	n := 0
	for _, line := range strings.Split(src, "\n") {
		if _, ok := testDefName(line); ok {
			n++
		}
	}
	return n
}

func NewOnlyHappyPath() scan.Rule {
	return onlyHappyPath{}
}
