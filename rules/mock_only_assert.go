package rules

import (
	"path/filepath"
	"strings"
	"unicode"

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

func mockOnlyAssertFromAST(file scan.File, model parse.Model) []scan.Finding {
	var findings []scan.Finding
	for _, t := range model.Tests {
		if !hasMockAssert(t) {
			continue
		}
		if hasNonMockAssert(t) {
			continue
		}
		if skipMockOnlyAssert(file, t) {
			continue
		}
		q := t.QualName
		if q == "" {
			q = t.Name
		}
		line := t.Lineno
		for _, a := range t.Asserts {
			if a.Kind == "mock_method" {
				line = a.Lineno
				break
			}
		}
		findings = append(findings, scan.Finding{
			File:     file.Path,
			Line:     line,
			Rule:     "mock-only-assert",
			Severity: "note",
			Message:  "mock only assert found in test: " + q,
			QualName: q,
		})
	}
	return findings
}

func skipMockOnlyAssert(file scan.File, t parse.TestFunc) bool {
	if mockOnlyBoundaryPath(file.Path) {
		return true
	}
	if hasConstructorPatch(t, string(file.Content)) {
		return true
	}
	// Procedural SUT: calls that aren't mocks/patches are bare (result unused).
	if sutCallsAreProcedural(t) {
		return true
	}
	return false
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
	if strings.HasSuffix(base, "_client.py") {
		return true
	}
	return false
}

func hasConstructorPatch(t parse.TestFunc, src string) bool {
	for _, d := range t.Decorators {
		if patchTargetLooksLikeClass(d) {
			return true
		}
	}
	lines := strings.Split(src, "\n")
	start := t.Lineno - 1
	if start < 0 {
		start = 0
	}
	end := t.EndLineno
	if end <= 0 || end > len(lines) {
		end = len(lines)
	}
	from := start - 3
	if from < 0 {
		from = 0
	}
	snippet := strings.Join(lines[from:end], "\n")
	for _, line := range strings.Split(snippet, "\n") {
		if patchTargetLooksLikeClass(line) {
			return true
		}
	}
	return false
}

func patchTargetLooksLikeClass(decorator string) bool {
	dl := strings.ToLower(decorator)
	if !strings.Contains(dl, "patch") {
		return false
	}
	// Extract last dotted segment inside quotes: patch("a.b.ManagedIdentityCredential")
	start := strings.IndexAny(decorator, `"'`)
	if start < 0 {
		return false
	}
	quote := decorator[start]
	end := strings.IndexByte(decorator[start+1:], quote)
	if end < 0 {
		return false
	}
	target := decorator[start+1 : start+1+end]
	leaf := leafName(target)
	if leaf == "" {
		return false
	}
	r := []rune(leaf)
	return unicode.IsUpper(r[0])
}

func sutCallsAreProcedural(t parse.TestFunc) bool {
	var sut []parse.Call
	for _, c := range t.Calls {
		if isMockOrPatchOrAssertCall(c.Name) {
			continue
		}
		sut = append(sut, c)
	}
	if len(sut) == 0 {
		// only mock asserts — still a hit unless path/constructor skipped
		return false
	}
	for _, c := range sut {
		if !c.Bare {
			// result of SUT call is used (assigned / nested) — real mock-only smell
			return false
		}
	}
	return true
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
