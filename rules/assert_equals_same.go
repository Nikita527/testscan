package rules

import (
	"strings"

	"github.com/Nikita527/testscan/scan"
)

type assertEqualsSame struct{}

func (assertEqualsSame) ID() string {
	return "assert-equals-same"
}

func (assertEqualsSame) Check(file scan.File) []scan.Finding {
	var findings []scan.Finding
	src := string(file.Content)
	for i, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "assert ") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(t, "assert "))
		// отрежем сообщение после запятой: assert x == x, "msg"
		if idx := strings.Index(rest, ","); idx >= 0 {
			rest = strings.TrimSpace(rest[:idx])
		}
		left, right, ok := splitEq(rest)
		if !ok {
			continue
		}
		if strings.TrimSpace(left) == strings.TrimSpace(right) {
			findings = append(findings, scan.Finding{
				File:     file.Path,
				Line:     i + 1,
				Rule:     "assert-equals-same",
				Severity: "warning",
				Message:  "assert compares expression to itself",
			})
		}
	}
	return findings
}

// splitEq делит "a == b" по первому " == "; иначе ok=false.
func splitEq(s string) (left, right string, ok bool) {
	idx := strings.Index(s, "==")
	if idx < 0 {
		return "", "", false
	}
	left = s[:idx]
	right = s[idx+2:]
	return left, right, true
}

func NewAssertEqualsSame() scan.Rule {
	return assertEqualsSame{}
}
