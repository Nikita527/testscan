package rules

import (
	"strings"

	"github.com/Nikita527/testscan/scan"
)

type noBehaviorChange struct{}

func (noBehaviorChange) ID() string {
	return "no-behavior-change"
}

func (noBehaviorChange) Check(file scan.File) []scan.Finding {
	src := string(file.Content)
	if countTestFuncs(src) < 1 {
		return nil
	}
	if strings.Contains(src, "pytest.raises") ||
		strings.Contains(src, "pytest.warns") ||
		strings.Contains(src, "assertRaises") {
		return nil
	}

	var assertLines []string
	for _, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "assert ") {
			assertLines = append(assertLines, t)
		}
	}
	if len(assertLines) == 0 {
		return nil
	}
	for _, a := range assertLines {
		if !isTypeOnlyAssert(a) {
			return nil
		}
	}
	return []scan.Finding{{
		File:     file.Path,
		Line:     lineOf(src, assertLines[0]),
		Rule:     "no-behavior-change",
		Severity: "warning",
		Message:  "tests only check types (isinstance/type), no behavior assert",
	}}
}

func isTypeOnlyAssert(line string) bool {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "assert "))
	if idx := strings.Index(rest, ","); idx >= 0 {
		rest = strings.TrimSpace(rest[:idx])
	}
	if strings.HasPrefix(rest, "isinstance(") {
		return true
	}
	if strings.HasPrefix(rest, "type(") {
		return strings.Contains(rest, " is ") || strings.Contains(rest, " == ")
	}
	return false
}

func NewNoBehaviorChange() scan.Rule {
	return noBehaviorChange{}
}
