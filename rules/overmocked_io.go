package rules

import (
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

type overmockedIO struct{}

func (overmockedIO) ID() string {
	return "overmocked-io"
}

func (overmockedIO) NeedsAST() bool { return true }

func (overmockedIO) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		if useHeuristic(file, err) {
			return overmockedIOHeuristic(file)
		}
		return nil
	}
	return overmockedIOFromAST(file, model)
}

func overmockedIOFromAST(file scan.File, model parse.Model) []scan.Finding {
	src := string(file.Content)
	var findings []scan.Finding
	for _, t := range model.Tests {
		if !testHasIOPatch(t, src) {
			continue
		}
		if !hasMockAssert(t) {
			continue
		}
		if hasNonMockAssert(t) {
			continue
		}
		q := t.QualName
		if q == "" {
			q = t.Name
		}
		findings = append(findings, scan.Finding{
			File:     file.Path,
			Line:     t.Lineno,
			Rule:     "overmocked-io",
			Severity: "warning",
			Message:  "IO patched and only mock asserts found in test: " + q,
			QualName: q,
		})
	}
	return findings
}

func overmockedIOHeuristic(file scan.File) []scan.Finding {
	src := string(file.Content)
	if !ioPatchInContent(src) {
		return nil
	}
	hasMock := strings.Contains(src, "assert_called") || strings.Contains(src, "assert_has_calls")
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

func NewOvermockedIO() scan.Rule {
	return overmockedIO{}
}
