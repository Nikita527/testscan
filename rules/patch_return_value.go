package rules

import (
	"regexp"
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
)

// Helpers shared with self-patched-sut (formerly in mock_tautology.go).

var rePatchReturnValue = regexp.MustCompile(
	`(?i)patch(?:\.object)?\s*\(\s*["']([^"']+)["'][^)]*return_value\s*=\s*([^,\)]+)`,
)

func directMockReturnValueAssert(a parse.Assert) bool {
	left := strings.TrimSpace(a.Left)
	right := strings.TrimSpace(a.Right)
	text := strings.TrimSpace(a.Text)
	if a.Kind == "truthy" && strings.Contains(text, ".return_value") {
		return true
	}
	if a.Kind != "compare" {
		return false
	}
	return strings.Contains(left, ".return_value") || strings.Contains(right, ".return_value")
}

func parsePatchReturnValue(decorator string) (target, value string, ok bool) {
	m := rePatchReturnValue.FindStringSubmatch(decorator)
	if len(m) < 3 {
		return "", "", false
	}
	return strings.TrimSpace(m[1]), strings.TrimSpace(m[2]), true
}
