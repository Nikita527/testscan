package rules

import (
	"path/filepath"
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

type mockOnlyAssert struct{}

func (mockOnlyAssert) ID() string {
	return "mock-only-assert"
}

func (mockOnlyAssert) NeedsAST() bool { return true }

func (mockOnlyAssert) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		if useHeuristic(file, err) {
			return mockOnlyAssertHeuristic(file)
		}
		return nil
	}
	return mockOnlyAssertFromAST(file, model)
}

// mockOnlyAssertFromAST: the test's only checks are mock assertions AND the asserted
// mock is not wired into anything the test executes (data-flow, see wired in
// dataflow.go). A mock injected into the SUT, patched at a boundary the SUT calls,
// or reachable through other test activity is the SUT's observable boundary — the
// call on it IS the behaviour (orchestration code) — and is never reported.
func mockOnlyAssertFromAST(file scan.File, model parse.Model) []scan.Finding {
	var findings []scan.Finding
	ctx := NewSUTContext(file, model)
	for _, t := range model.Tests {
		if !hasMockAssert(t) || hasNonMockAssert(t) {
			continue
		}
		df := AnalyzeDataflow(file, model, ctx, t)
		if len(df.MockAsserts) == 0 {
			continue
		}
		line := df.MockAsserts[0].Lineno
		unwired := true
		for _, m := range df.MockAsserts {
			if m.Lineno < line {
				line = m.Lineno
			}
			if m.Wired {
				unwired = false
				break
			}
		}
		if !unwired {
			continue
		}
		q := t.QualName
		if q == "" {
			q = t.Name
		}
		if line == 0 {
			line = t.Lineno
		}
		findings = append(findings, scan.Finding{
			File:     file.Path,
			Line:     line,
			Rule:     "mock-only-assert",
			Severity: "note",
			Message:  "mock only assert found in test: " + q + "; the asserted mock is not passed to or patched into any code the test runs, so the assert does not depend on the code under test",
			QualName: q,
		})
	}
	return findings
}

func mockOnlyBoundaryPath(path string) bool {
	p := filepath.ToSlash(strings.ToLower(path))
	base := filepath.Base(p)
	segs := strings.Split(p, "/")
	for _, seg := range segs {
		switch seg {
		case "adapters", "clients", "orchestrator", "admin":
			return true
		}
	}
	return strings.HasSuffix(base, "_client.py")
}

func isMockOrPatchOrAssertCall(name string) bool {
	leaf := leafName(name)
	n := strings.ToLower(name)
	if strings.Contains(n, "patch") ||
		leaf == "MagicMock" || leaf == "Mock" || leaf == "AsyncMock" || leaf == "PropertyMock" {
		return true
	}
	if strings.HasPrefix(leaf, "assert_called") ||
		leaf == "assert_has_calls" || leaf == "assert_any_call" || leaf == "assert_not_called" {
		return true
	}
	if isAssertHelperCall(name, nil) {
		return true
	}
	return false
}

func mockOnlyAssertHeuristic(file scan.File) []scan.Finding {
	if mockOnlyBoundaryPath(file.Path) {
		return nil
	}
	src := string(file.Content)
	hasMock := strings.Contains(src, "assert_called") || strings.Contains(src, "assert_has_calls")
	hasAssert := strings.Contains(src, "assert ")
	if hasMock && !hasAssert {
		return []scan.Finding{{
			File:     file.Path,
			Line:     lineOfAny(src, "assert_called", "assert_has_calls"),
			Rule:     "mock-only-assert",
			Severity: "note",
			Message:  "mock only assert found in test file",
		}}
	}
	return nil
}

func NewMockOnlyAssert() scan.Rule {
	return mockOnlyAssert{}
}
