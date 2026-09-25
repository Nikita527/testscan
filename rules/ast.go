package rules

import (
	"context"
	"strings"
	"time"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

// astModel uses the scan.Run cache (one spawn per file) or parses itself (Check outside Run).
func astModel(file scan.File) (parse.Model, error) {
	if file.ModelOK {
		return file.Model, file.ModelErr
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return parse.File(ctx, file.Path, file.Content)
}

// useHeuristic allows text fallback only when explicitly enabled.
func useHeuristic(file scan.File, err error) bool {
	return err != nil && file.AllowHeuristicFallback
}

func leafName(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i+1:]
	}
	return name
}

// isAssertHelperCall matches assert_*/check_*/unittest assert* / _assert_*,
// plus patterns from file.AssertHelpers (assert-helpers config).
func isAssertHelperCall(name string, patterns []string) bool {
	leaf := leafName(name)
	if len(patterns) > 0 {
		for _, p := range patterns {
			if matchSimpleGlob(p, leaf) || matchSimpleGlob(p, name) {
				return true
			}
		}
	}
	if strings.HasPrefix(leaf, "assert") && leaf != "assert" {
		return true
	}
	if strings.HasPrefix(leaf, "check_") {
		return true
	}
	if strings.Contains(name, "._assert_") || strings.Contains(name, ".assert_") {
		return true
	}
	return false
}

func matchSimpleGlob(pattern, name string) bool {
	if pattern == "" {
		return false
	}
	// * suffix/prefix only — enough for assert_* / check_*
	if strings.HasSuffix(pattern, "*") && strings.HasPrefix(pattern, "*") {
		mid := pattern[1 : len(pattern)-1]
		return mid == "" || strings.Contains(name, mid)
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(name, strings.TrimSuffix(pattern, "*"))
	}
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(name, strings.TrimPrefix(pattern, "*"))
	}
	return pattern == name
}

func testHasAssertOrHelper(t parse.TestFunc, helpers []string) bool {
	if t.HasAssert || t.HasRaises {
		return true
	}
	for _, c := range t.Calls {
		if isAssertHelperCall(c.Name, helpers) {
			return true
		}
	}
	return false
}

func hasMockAssert(t parse.TestFunc) bool {
	for _, a := range t.Asserts {
		if a.Kind == "mock_method" {
			return true
		}
	}
	for _, c := range t.Calls {
		leaf := leafName(c.Name)
		if strings.HasPrefix(leaf, "assert_called") ||
			leaf == "assert_has_calls" ||
			leaf == "assert_any_call" ||
			leaf == "assert_not_called" {
			return true
		}
	}
	return false
}

func hasNonMockAssert(t parse.TestFunc) bool {
	for _, a := range t.Asserts {
		if a.Kind != "mock_method" {
			return true
		}
	}
	if t.HasRaises {
		return true
	}
	for _, c := range t.Calls {
		if !isAssertHelperCall(c.Name, nil) {
			continue
		}
		leaf := leafName(c.Name)
		if strings.HasPrefix(leaf, "assert_called") ||
			leaf == "assert_has_calls" ||
			leaf == "assert_any_call" ||
			leaf == "assert_not_called" {
			continue
		}
		return true
	}
	return false
}

func ioPatchInContent(src string) bool {
	needles := []string{
		`patch("builtins.open"`,
		`patch('builtins.open'`,
		`patch("pathlib`,
		`patch('pathlib`,
		`patch("requests.`,
		`patch('requests.`,
		`patch("httpx.`,
		`patch('httpx.`,
		`patch("urllib`,
		`patch('urllib`,
		`@patch("builtins.open"`,
		`@patch('builtins.open'`,
		`@patch("pathlib`,
		`@patch('pathlib`,
		`@patch("requests.`,
		`@patch('requests.`,
		`@patch("httpx.`,
		`@patch('httpx.`,
		`@patch("urllib`,
		`@patch('urllib`,
		"builtins.open",
	}
	for _, n := range needles {
		if strings.Contains(src, n) {
			return true
		}
	}
	for _, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		if !strings.Contains(t, "patch") {
			continue
		}
		if strings.Contains(t, "builtins.open") ||
			strings.Contains(t, "pathlib") ||
			strings.Contains(t, "requests.") ||
			strings.Contains(t, "httpx.") ||
			strings.Contains(t, "urllib") {
			return true
		}
	}
	return false
}

func testHasIOPatch(t parse.TestFunc, src string) bool {
	lines := strings.Split(src, "\n")
	start := t.Lineno - 1
	if start < 0 {
		start = 0
	}
	end := t.EndLineno
	if end <= 0 || end > len(lines) {
		end = len(lines)
	}
	from := start - 8
	if from < 0 {
		from = 0
	}
	snippet := strings.Join(lines[from:end], "\n")
	if !ioPatchInContent(snippet) {
		return false
	}
	for _, d := range t.Decorators {
		if strings.Contains(d, "patch") {
			return true
		}
	}
	for _, c := range t.Calls {
		if strings.Contains(c.Name, "patch") {
			return true
		}
	}
	// decorator name may be just "patch"
	return ioPatchInContent(snippet)
}
