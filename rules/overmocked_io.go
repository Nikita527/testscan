package rules

import (
	"strings"

	"github.com/Nikita527/testscan/scan"
)

type overmockedIO struct{}

func (overmockedIO) ID() string {
	return "overmocked-io"
}

func (overmockedIO) Check(file scan.File) []scan.Finding {
	src := string(file.Content)
	if !hasIOPatch(src) {
		return nil
	}
	hasMock := strings.Contains(src, "assert_called") ||
		strings.Contains(src, "assert_has_calls")
	if !hasMock {
		return nil
	}
	if strings.Contains(src, "assert ") {
		return nil
	}
	return []scan.Finding{{
		File:     file.Path,
		Line:     lineOfAny(src, ioPatchNeedles...),
		Rule:     "overmocked-io",
		Severity: "warning",
		Message:  "IO patched and only mock asserts found",
	}}
}

var ioPatchNeedles = []string{
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
}

func hasIOPatch(src string) bool {
	for _, n := range ioPatchNeedles {
		if strings.Contains(src, n) {
			return true
		}
	}
	// также patch.object(..., "open") и общие формы с pathlib/requests в строке patch
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

func NewOvermockedIO() scan.Rule {
	return overmockedIO{}
}
