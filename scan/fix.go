package scan

// RuleFix is the default one-action fix hint per rule ID. Run applies it to
// findings whose rule left Fix empty; rules set a more concrete Fix themselves
// when they know specifics (the called function, the twin test, ...).
var RuleFix = map[string]string{
	"empty-test":                          "add the missing arrange/act/assert body, or delete the test",
	"no-assert":                           "add an `assert` on the observable result of the call (return value, state change or raised error)",
	"assert-true":                         "replace the constant assert with a comparison of the actual result to an expected value",
	"mock-only-assert":                    "assert on the return value or resulting state of the code under test, not only on mock calls",
	"todo-test":                           "implement the test or delete it; do not leave a bare todo/skip marker",
	"duplicate-test-name":                 "rename one of the tests so every test function name in the scope is unique",
	"only-happy-path":                     "add a test for the failure path, e.g. invalid input with `pytest.raises(SpecificError, match=...)`",
	"assert-equals-same":                  "compare the actual result to an independent expected value instead of to itself",
	"snapshot-only":                       "add an explicit assert on one or two key fields next to the snapshot assert",
	"overmocked-io":                       "assert on the return value or stored result of the code under test, not only on the patched IO mock",
	"test-imports-implementation-private": "import and test the public API instead of the underscore-prefixed implementation name",
	"no-behavior-change":                  "replace the isinstance/type check with an assert on a concrete value or behavior",
	"fake-mock-assert":                    "fix the mock assertion to a real method, e.g. `mock.assert_called_once_with(...)`",
	"assert-tuple":                        "remove the parentheses/comma so the assert compares values: `assert a == b`",
	"broad-raises":                        "replace `pytest.raises(Exception)` with the specific exception, e.g. `pytest.raises(ValueError, match=...)`, and keep only the failing call in the block",
	"swallowed-exception":                 "remove the bare `except` / `except Exception`, or re-raise after handling; use `pytest.raises` for expected errors",
	"assert-in-emptyable-loop":            "assert the collection is non-empty before the loop, e.g. `assert items`, or assert the whole collection at once",
	"weak-assert":                         "assert the exact expected value (`==`) instead of a truthiness/length/membership check",
	"mock-tautology":                      "remove the assert on the mock's own `return_value`; assert the result of the code under test",
	"sleep-in-test":                       "remove `time.sleep`; wait on the condition or use a fake clock",
	"wall-clock-in-test":                  "use `timezone.localdate()` instead of `datetime.now().date()`, or freeze time with freezegun/time-machine",
	"skip-without-reason":                 "add `reason=\"...\"` (and `strict=True` for xfail) to the skip/xfail marker",
	"near-duplicate-test":                 "merge into one test with `@pytest.mark.parametrize` over the differing inputs",
	"self-patched-sut":                    "stop patching the code under test; patch only its external collaborators",
	"expected-recomputed":                 "replace the recomputed expected value with a literal",
	"commented-assert":                    "restore the commented-out assert or delete it",
	"overbroad-equality":                  "assert the specific fields that matter instead of the whole huge literal",
	"error-contract-assert":               "assert the error contract: status code and error code/message of the response",
	"raises-without-check":                "add `match=...` to `pytest.raises` or assert on the caught exception's attributes",
	"missing-mirror-test":                 "create the mirror test file for this module with at least one behavior test",
	"rbac-mutation-guard":                 "assert the forbidden outcome: status 401/403 or `PermissionDenied`, and that nothing was mutated",
	"name-body-mismatch":                  "rename the test to match what it checks, or add the negative-path assertion its name promises",
	"parse-error":                         "fix the Python syntax error so the file can be analyzed",
}

// DefaultFix returns the default fix hint for a rule ID ("" if unknown).
func DefaultFix(rule string) string { return RuleFix[rule] }

// ApplyDefaultFixes fills Fix from RuleFix for findings that have none.
func ApplyDefaultFixes(findings []Finding) {
	for i := range findings {
		if findings[i].Fix == "" {
			findings[i].Fix = RuleFix[findings[i].Rule]
		}
	}
}
