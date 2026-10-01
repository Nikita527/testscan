package rules

import (
	"regexp"
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

type noAssert struct{}

func (noAssert) ID() string {
	return "no-assert"
}

func (noAssert) NeedsAST() bool { return true }

func (noAssert) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		if useHeuristic(file, err) {
			return noAssertHeuristic(file)
		}
		return nil
	}
	return noAssertFromAST(file, model)
}

var noRaiseNamePhrases = []string{
	"does_not_raise", "no_raise", "doesnt_raise", "no_op",
	"does_not_propagate", "must_not_raise", "should_not_raise", "no_error", "no_errors",
	"no_exception", "smoke",
}

// noRaiseNameVerbsEdge — first/last token or name prefix/suffix only
// (avoid mid-token FP/FN like test_password_passes_checks).
var noRaiseNameVerbsEdge = []string{
	"ok", "noop", "accepts", "passes",
}

// noRaiseNameVerbsMid — intentional no-raise verbs matched on any underscore token.
var noRaiseNameVerbsMid = []string{
	"swallows", "ignores", "skips", "allows", "allow", "allowed", "tolerates",
}

var implicitCheckPrefixes = []string{
	"validate_", "raise_if_", "check_", "ensure_", "assert_",
}

func isNoRaiseTestName(name string) bool {
	n := strings.ToLower(leafName(name))
	n = strings.TrimPrefix(n, "test_")
	for _, phrase := range noRaiseNamePhrases {
		if strings.Contains("_"+n+"_", "_"+phrase+"_") {
			return true
		}
	}
	// Mid-token: test_release_swallows_redis_errors
	for _, tok := range strings.Split(n, "_") {
		if tok == "" {
			continue
		}
		for _, verb := range noRaiseNameVerbsMid {
			if tok == verb {
				return true
			}
		}
	}
	// Edge-only: accepts_*, *_passes, ok — not password_passes_checks.
	for _, verb := range noRaiseNameVerbsEdge {
		if n == verb || strings.HasPrefix(n, verb+"_") || strings.HasSuffix(n, "_"+verb) {
			return true
		}
	}
	return false
}

func isImplicitCheckCall(name string) bool {
	leaf := leafName(name)
	for _, p := range implicitCheckPrefixes {
		if strings.HasPrefix(leaf, p) || strings.HasPrefix(leaf, "_"+p) {
			return true
		}
	}
	return false
}

func isBoringCall(name string) bool {
	leaf := leafName(name)
	switch leaf {
	case "patch", "MagicMock", "Mock", "AsyncMock", "PropertyMock",
		"print", "len", "str", "int", "bool", "list", "dict", "set", "tuple",
		"getattr", "setattr", "isinstance", "type", "super":
		return true
	}
	if strings.HasPrefix(leaf, "assert_called") ||
		leaf == "assert_has_calls" || leaf == "assert_any_call" || leaf == "assert_not_called" {
		return true
	}
	if strings.Contains(strings.ToLower(name), "patch") {
		return true
	}
	return false
}

// isImplicitNoRaiseBody is true when the test body is essentially a call to
// validate_/raise_if_/check_/ensure_/assert_* (no-raise style).
func isImplicitNoRaiseBody(t parse.TestFunc) bool {
	var meaningful []parse.Call
	for _, c := range t.Calls {
		if isBoringCall(c.Name) {
			continue
		}
		meaningful = append(meaningful, c)
	}
	if len(meaningful) == 0 {
		return false
	}
	for _, c := range meaningful {
		if !isImplicitCheckCall(c.Name) {
			return false
		}
	}
	return true
}

// noAssertFromAST implements the data-flow semantics:
//
//   - the test has no assert / raises / assert helper at all: reported (old
//     behaviour) unless it is an intentional "does not raise" test;
//   - the test has checks and calls the SUT, but none of them depends on the SUT
//     result (AnalyzeDataflow): reported;
//   - the test has checks but never calls the SUT (fixture-driven, helper-driven,
//     pure-literal): NOT reported — SUT calls cannot be told apart, so no claim;
//   - tests whose only checks are mock assertions belong to mock-only-assert.
func noAssertFromAST(file scan.File, model parse.Model) []scan.Finding {
	var findings []scan.Finding
	ctx := NewSUTContext(file, model)
	for _, t := range model.Tests {
		if t.IsEmpty {
			continue
		}
		q := t.QualName
		if q == "" {
			q = t.Name
		}
		if !testHasAssertOrHelper(t, file.AssertHelpers, model) && !hasQueryCountCheck(t) && !t.NestedAssert {
			if isNoRaiseTestName(q) || isImplicitNoRaiseBody(t) || hasNoRaiseComment(file, t) {
				// Legitimate no-raise / validate_* style — not a finding.
				continue
			}
			findings = append(findings, scan.Finding{
				File:     file.Path,
				Line:     t.Lineno,
				Rule:     "no-assert",
				Severity: "note",
				Message:  "no assert found in test: " + q + "; if intentional, use does_not_raise() or rename (e.g. test_…_does_not_raise_…)",
				QualName: q,
			})
			continue
		}
		if hasMockAssert(t) && !hasNonMockAssert(t) {
			continue // mock-only-assert territory
		}
		df := AnalyzeDataflow(file, model, ctx, t)
		if !df.NoDependentCheck() {
			continue
		}
		if isNoRaiseTestName(q) || hasNoRaiseComment(file, t) {
			continue
		}
		findings = append(findings, scan.Finding{
			File:     file.Path,
			Line:     t.Lineno,
			Rule:     "no-assert",
			Severity: "note",
			Message:  "no assert depends on the code under test in: " + q + "; asserts check data unrelated to the SUT call result or its side effects",
			QualName: q,
		})
	}
	return findings
}

var reNoRaiseComment = regexp.MustCompile(
	`(?i)#.*\b(?:must\s*not|should\s*not|shouldn'?t|mustn'?t|does\s*not|doesn'?t|do\s*not|don'?t|never)\s+(?:raise|throw|fail|crash|blow\s*up)`)

// hasNoRaiseComment: a comment inside the test saying it must/should not raise.
func hasNoRaiseComment(file scan.File, t parse.TestFunc) bool {
	if len(file.Content) == 0 {
		return false
	}
	lines := strings.Split(string(file.Content), "\n")
	start := t.Lineno - 1
	if start < 0 {
		start = 0
	}
	end := t.EndLineno
	if end <= 0 || end > len(lines) {
		end = len(lines)
	}
	for i := start; i < end && i < len(lines); i++ {
		if reNoRaiseComment.MatchString(lines[i]) {
			return true
		}
	}
	return false
}

func noAssertHeuristic(file scan.File) []scan.Finding {
	src := string(file.Content)
	if strings.Contains(src, "assert") ||
		strings.Contains(src, "pytest.raises") ||
		strings.Contains(src, "pytest.warns") {
		return nil
	}
	return []scan.Finding{{
		File:     file.Path,
		Line:     1,
		Rule:     "no-assert",
		Severity: "note",
		Message:  "no assert found in test file",
	}}
}

func NewNoAssert() scan.Rule {
	return noAssert{}
}
