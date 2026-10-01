package rules_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/rules"
	"github.com/Nikita527/testscan/scan"
)

// minCorpusPrecision is the CI gate for labeled hit/clean fixtures.
// A rule that starts flagging clean/ cases fails the build.
const minCorpusPrecision = 0.95

func TestCorpus(t *testing.T) {
	cases := []struct {
		name     string
		rule     scan.Rule
		root     string
		wantFile string
		wantRule string
		wantLine int
	}{
		{"empty-test", rules.NewEmptyTest(), "testdata/empty", "test_empty.py", "empty-test", 1},
		{"no-assert", rules.NewNoAssert(), "testdata/no_assert", "test_no_assert.py", "no-assert", 1},
		{"assert-true", rules.NewAssertTrue(), "testdata/assert_true", "test_assert_true.py", "assert-true", 2},
		{"mock-only-assert", rules.NewMockOnlyAssert(), "testdata/mock_only_assert", "test_mock_only.py", "mock-only-assert", 3},
		{"todo-test", rules.NewTodoTest(), "testdata/todo", "test_todo.py", "todo-test", 2},
		{"duplicate-test-name", rules.NewDuplicateTestName(), "testdata/duplicate_test_name", "test_dup.py", "duplicate-test-name", 5},
		{"only-happy-path", rules.NewOnlyHappyPath(), "testdata/only_happy_path", "test_happy.py", "only-happy-path", 4},
		{"assert-equals-same", rules.NewAssertEqualsSame(), "testdata/assert_equals_same", "test_same.py", "assert-equals-same", 2},
		{"snapshot-only", rules.NewSnapshotOnly(), "testdata/snapshot_only", "test_snap.py", "snapshot-only", 1},
		{"overmocked-io", rules.NewOvermockedIO(), "testdata/overmocked_io", "test_io.py", "overmocked-io", 4},
		{"test-imports-implementation-private", rules.NewPrivateImport(), "testdata/test_imports_implementation_private", "test_priv.py", "test-imports-implementation-private", 1},
		{"no-behavior-change", rules.NewNoBehaviorChange(), "testdata/no_behavior_change", "test_type.py", "no-behavior-change", 3},
		{"fake-mock-assert", rules.NewFakeMockAssert(), "testdata/fake_mock_assert", "test_fake.py", "fake-mock-assert", 2},
		{"assert-tuple", rules.NewAssertTuple(), "testdata/assert_tuple", "test_tuple.py", "assert-tuple", 3},
		{"broad-raises", rules.NewBroadRaises(), "testdata/broad_raises", "test_broad.py", "broad-raises", 5},
		{"swallowed-exception", rules.NewSwallowedException(), "testdata/swallowed_exception", "test_swallowed.py", "swallowed-exception", 4},
		{"assert-in-emptyable-loop", rules.NewAssertInEmptyableLoop(), "testdata/assert_in_emptyable_loop", "test_loop.py", "assert-in-emptyable-loop", 4},
		{"weak-assert", rules.NewWeakAssert(), "testdata/weak_assert", "test_weak.py", "weak-assert", 3},
		{"mock-tautology", rules.NewMockTautology(), "testdata/mock_tautology", "test_tautology.py", "mock-tautology", 3},
		{"sleep-in-test", rules.NewSleepInTest(), "testdata/sleep_in_test", "test_sleep.py", "sleep-in-test", 5},
		{"wall-clock-in-test", rules.NewWallClockInTest(), "testdata/wall_clock_in_test", "test_now.py", "wall-clock-in-test", 5},
		{"skip-without-reason", rules.NewSkipWithoutReason(), "testdata/skip_without_reason", "test_skip.py", "skip-without-reason", 5},
		{"near-duplicate-test", rules.NewNearDuplicateTest(), "testdata/near_duplicate_test", "test_dup.py", "near-duplicate-test", 1},
		{"name-body-mismatch", rules.NewNameBodyMismatch(), "testdata/name_body_mismatch", "test_mismatch.py", "name-body-mismatch", 1},
		{"self-patched-sut", rules.NewSelfPatchedSUT(), "testdata/self_patched_sut", "test_self.py", "self-patched-sut", 6},
		{"expected-recomputed", rules.NewExpectedRecomputed(), "testdata/expected_recomputed", "test_recomputed.py", "expected-recomputed", 3},
		{"commented-assert", rules.NewCommentedAssert(), "testdata/commented_assert", "test_commented.py", "commented-assert", 3},
		{"overbroad-equality", rules.NewOverbroadEquality(), "testdata/overbroad_equality", "test_huge.py", "overbroad-equality", 3},
		{"error-contract-assert", rules.NewErrorContractAssert(rules.ErrorContractAssertOpts{}), "testdata/error_contract_assert", "test_status_only.py", "error-contract-assert", 1},
		{"raises-without-check", rules.NewRaisesWithoutCheck(rules.RaisesWithoutCheckOpts{}), "testdata/raises_without_check", "test_bare_raises.py", "raises-without-check", 5},
		{"rbac-mutation-guard", rules.NewRBACMutationGuard(rules.RBACMutationGuardOpts{}), "testdata/rbac_mutation_guard", "test_rbac_open.py", "rbac-mutation-guard", 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := testRun(t, []string{tc.root}, tc.rule)
			if err != nil {
				t.Fatal(err)
			}
			findings := res.Findings
			if len(findings) < 1 {
				t.Fatalf("got 0 findings, want ≥1 hit")
			}
			var tp, fp int
			matched := false
			for _, f := range findings {
				parent := filepath.Base(filepath.Dir(f.File))
				if parent != "hit" {
					t.Errorf("finding outside hit/: %s", f.File)
				}
				ruleMatch := f.Rule == tc.wantRule || strings.HasPrefix(f.Rule, tc.wantRule)
				switch parent {
				case "hit":
					if ruleMatch {
						tp++
					}
					if filepath.Base(f.File) == tc.wantFile && f.Rule == tc.wantRule && f.Line == tc.wantLine {
						matched = true
					}
				case "clean":
					if ruleMatch {
						fp++
					}
				}
			}
			if !matched {
				t.Fatalf("missing expected hit %s:%d %s among %+v", tc.wantFile, tc.wantLine, tc.wantRule, findings)
			}
			if tp < 1 {
				t.Fatalf("expected ≥1 true positive in %s/hit", tc.root)
			}
			total := tp + fp
			precision := float64(tp) / float64(total)
			if precision < minCorpusPrecision {
				t.Fatalf("precision=%.2f (tp=%d fp=%d) below %.2f on %s",
					precision, tp, fp, minCorpusPrecision, tc.root)
			}
			t.Logf("precision=%.2f (tp=%d fp=%d)", precision, tp, fp)
		})
	}
}

func TestDefault(t *testing.T) {
	got := rules.Default()
	if len(got) != 27 {
		t.Fatalf("got %d rules, want 27", len(got))
	}
}

func TestEmptyTest_Heuristics(t *testing.T) {
	rule := rules.NewEmptyTest()
	cases := []struct {
		name string
		src  string
		hit  bool
	}{
		{"empty", "", true},
		{"pass_only", "def test_x():\n    pass\n", true},
		{"docstring_only", "def test_x():\n    \"\"\"only docs\"\"\"\n", true},
		{"with_assert", "def test_x():\n    assert 1 == 1\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rule.Check(scan.File{
				Path:                   "t.py",
				Content:                []byte(tc.src),
				AllowHeuristicFallback: true,
			})
			if tc.hit && len(got) != 1 {
				t.Fatalf("want hit, got %d findings", len(got))
			}
			if !tc.hit && len(got) != 0 {
				t.Fatalf("want clean, got %v", got)
			}
		})
	}
}

func TestTodoTest_NoBareAssertFalse(t *testing.T) {
	rule := rules.NewTodoTest()
	bare := rule.Check(scan.File{
		Path:                   "t.py",
		Content:                []byte("def test_x():\n    assert False\n"),
		AllowHeuristicFallback: true,
	})
	if len(bare) != 0 {
		t.Fatalf("bare assert False must not hit, got %v", bare)
	}
	withTODO := rule.Check(scan.File{
		Path:                   "t.py",
		Content:                []byte("def test_x():\n    assert False, \"TODO\"\n"),
		AllowHeuristicFallback: true,
	})
	if len(withTODO) != 1 {
		t.Fatalf("assert False, TODO must hit, got %d", len(withTODO))
	}
}

func TestTodoTest_SkipWithReasonClean(t *testing.T) {
	rule := rules.NewTodoTest()
	src := "@pytest.mark.skip(reason=\"flaky\")\ndef test_ok():\n    assert True\n"
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte(src),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name:       "test_ok",
				QualName:   "test_ok",
				Lineno:     2,
				Decorators: []string{`pytest.mark.skip(reason='flaky')`},
				Asserts:    []parse.Assert{{Kind: "truthy", Lineno: 3, Text: "True"}},
			}},
		},
	})
	if len(got) != 0 {
		t.Fatalf("skip with reason must not hit todo-test, got %v", got)
	}
}

func TestTodoTest_SkipDecoratorWithTODO(t *testing.T) {
	rule := rules.NewTodoTest()
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("@pytest.mark.skip(reason=\"TODO\")\ndef test_x():\n    pass\n"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name:       "test_x",
				QualName:   "test_x",
				Lineno:     2,
				Decorators: []string{`pytest.mark.skip(reason='TODO')`},
			}},
		},
	})
	if len(got) != 1 {
		t.Fatalf("skip reason TODO must hit, got %d", len(got))
	}
}

func TestOnlyHappyPath_IsNotNoneDoesNotCountAsNegative(t *testing.T) {
	rule := rules.NewOnlyHappyPathMin(3)
	tests := make([]parse.TestFunc, 0, 4)
	for i, name := range []string{"test_a", "test_b", "test_c", "test_d"} {
		tests = append(tests, parse.TestFunc{
			Name:     name,
			QualName: name,
			Lineno:   i*3 + 1,
			Asserts: []parse.Assert{{
				Kind:   "compare",
				Text:   "result is not None",
				Left:   "result",
				Right:  "None",
				Lineno: i*3 + 2,
			}},
		})
	}
	got := rule.Check(sutFile(t, tests, "def test_a():\n    assert result is not None\n"))
	if len(got) != 1 {
		t.Fatalf("is not None alone must still be only-happy-path hit, got %d (%v)", len(got), got)
	}
}

func TestOnlyHappyPath_IsNoneIsNegative(t *testing.T) {
	rule := rules.NewOnlyHappyPathMin(3)
	tests := make([]parse.TestFunc, 0, 4)
	for i, name := range []string{"test_a", "test_b", "test_c", "test_d"} {
		a := parse.Assert{Kind: "compare", Text: "1 == 1", Left: "1", Right: "1", Lineno: i*3 + 2}
		if i == 3 {
			a = parse.Assert{Kind: "compare", Text: "result is None", Left: "result", Right: "None", Lineno: i*3 + 2}
		}
		tests = append(tests, parse.TestFunc{
			Name: name, QualName: name, Lineno: i*3 + 1, Asserts: []parse.Assert{a},
		})
	}
	got := rule.Check(sutFile(t, tests, "x"))
	if len(got) != 0 {
		t.Fatalf("is None must count as negative, got %v", got)
	}
}

func TestWeakAssert_IsNoneIsNotWeak(t *testing.T) {
	rule := rules.NewWeakAssert()
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("def test_missing():\n    assert result is None\n"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_missing", QualName: "test_missing", Lineno: 1,
				Asserts: []parse.Assert{{
					Kind: "compare", Text: "result is None", Left: "result", Right: "None", Lineno: 2,
				}},
			}},
		},
	})
	if len(got) != 0 {
		t.Fatalf("is None must not be weak-assert, got %v", got)
	}
}

func TestMockTautology_UnrelatedTrueClean(t *testing.T) {
	rule := rules.NewMockTautology()
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("def test_ok():\n    m.return_value = True\n    assert flag is True\n"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_ok", QualName: "test_ok", Lineno: 1,
				Assignments: []parse.Assignment{{Target: "m.return_value", Value: "True", Lineno: 2}},
				Asserts: []parse.Assert{{
					Kind: "compare", Text: "flag is True", Left: "flag", Right: "True", Lineno: 3,
				}},
			}},
		},
	})
	if len(got) != 0 {
		t.Fatalf("unrelated True assert must not hit mock-tautology, got %v", got)
	}
}

func TestMockTautology_SUTEchoClean(t *testing.T) {
	rule := rules.NewMockTautology()
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("x"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_ok", QualName: "test_ok", Lineno: 1,
				Assignments: []parse.Assignment{{Target: "m.return_value", Value: "None", Lineno: 2}},
				Asserts: []parse.Assert{{
					Kind: "compare", Text: "ensure.execute() is None",
					Left: "ensure.execute()", Right: "None", LeftIsCall: true, Lineno: 3,
				}},
			}},
		},
	})
	if len(got) != 0 {
		t.Fatalf("SUT call matching mock return_value must not hit, got %v", got)
	}
}

func TestMockTautology_PatchedSelf(t *testing.T) {
	// Decorator patch(... return_value=) + assert target() == X is owned by self-patched-sut.
	rule := rules.NewMockTautology()
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("x"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_x", QualName: "test_x", Lineno: 1,
				Decorators: []string{`patch("mod.func", return_value=42)`},
				Asserts: []parse.Assert{{
					Kind: "compare", Text: "mod.func() == 42",
					Left: "mod.func()", Right: "42", LeftIsCall: true, Lineno: 3,
				}},
			}},
		},
	})
	if len(got) != 0 {
		t.Fatalf("decorator patch echo belongs to self-patched-sut, got %v", got)
	}
}

func TestNoAssert_SeverityNote(t *testing.T) {
	rule := rules.NewNoAssert()
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("def test_x():\n    do_something()\n"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_x", QualName: "test_x", Lineno: 1,
				Calls: []parse.Call{{Name: "do_something", Lineno: 2, Bare: true}},
			}},
		},
	})
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %v", got)
	}
	if got[0].Severity != "note" {
		t.Fatalf("severity=%q, want note", got[0].Severity)
	}
}

func TestNoAssert_NoRaiseNameClean(t *testing.T) {
	rule := rules.NewNoAssert()
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("x"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_does_not_raise", QualName: "test_does_not_raise", Lineno: 1,
				Calls: []parse.Call{{Name: "process", Lineno: 2, Bare: true}},
			}},
		},
	})
	if len(got) != 0 {
		t.Fatalf("no-raise name must be clean, got %v", got)
	}
}

func TestNoAssert_PrivateHelperFollow(t *testing.T) {
	rule := rules.NewNoAssert()
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("x"),
		ModelOK: true,
		Model: parse.Model{
			Helpers: []parse.Helper{{Name: "_assert_ok", QualName: "_assert_ok", HasAssert: true}},
			Tests: []parse.TestFunc{{
				Name: "test_x", QualName: "test_x", Lineno: 1,
				Calls: []parse.Call{{Name: "_assert_ok", Lineno: 2}},
			}},
		},
	})
	if len(got) != 0 {
		t.Fatalf("helper with assert must be clean, got %v", got)
	}
}

func TestMockOnlyAssert_ProceduralClean(t *testing.T) {
	rule := rules.NewMockOnlyAssert()
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("x"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_x", QualName: "test_x", Lineno: 1,
				Calls: []parse.Call{
					{Name: "do_work", Lineno: 2, Bare: true},
					{Name: "mock.assert_called", Lineno: 3, Bare: true},
				},
				Asserts: []parse.Assert{{Kind: "mock_method", Lineno: 3, Text: "mock.assert_called()"}},
			}},
		},
	})
	if len(got) != 0 {
		t.Fatalf("procedural SUT must be clean, got %v", got)
	}
}

func TestMockOnlyAssert_BoundaryPathClean(t *testing.T) {
	rule := rules.NewMockOnlyAssert()
	got := rule.Check(scan.File{
		Path:    "tests/clients/test_foo.py",
		Content: []byte("x"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_x", QualName: "test_x", Lineno: 1,
				Calls: []parse.Call{
					{Name: "compute", Lineno: 2, Bare: false},
					{Name: "mock.assert_called", Lineno: 3, Bare: true},
				},
				Asserts: []parse.Assert{{Kind: "mock_method", Lineno: 3}},
			}},
		},
	})
	if len(got) != 0 {
		t.Fatalf("clients/ path must be clean, got %v", got)
	}
}

func TestBroadRaises_SpecificWithAttrClean(t *testing.T) {
	rule := rules.NewBroadRaises()
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("x"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_x", QualName: "test_x", Lineno: 1,
				Raises: []parse.Raise{{
					Exc: "ValueError", HasMatch: false, BodyStmtCount: 1,
					Lineno: 2, EndLineno: 3, AsName: "exc_info",
				}},
				Asserts: []parse.Assert{{
					Kind: "compare", Text: "exc_info.value.code == 'x'",
					Left: "exc_info.value.code", Right: "'x'", Lineno: 4,
				}},
			}},
		},
	})
	if len(got) != 0 {
		t.Fatalf("specific exc + value.attr must be clean, got %v", got)
	}
}

func TestBroadRaises_ExceptionWithAttrStillHits(t *testing.T) {
	rule := rules.NewBroadRaises()
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("x"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_x", QualName: "test_x", Lineno: 1,
				Raises: []parse.Raise{{
					Exc: "Exception", HasMatch: false, BodyStmtCount: 1,
					Lineno: 2, EndLineno: 3, AsName: "exc_info",
				}},
				Asserts: []parse.Assert{{
					Kind: "compare", Text: "exc_info.value.code == 'x'",
					Left: "exc_info.value.code", Right: "'x'", Lineno: 4,
				}},
			}},
		},
	})
	if len(got) != 1 {
		t.Fatalf("broad Exception must still hit even with attr check, got %v", got)
	}
	if !strings.Contains(got[0].Message, "concrete exception") {
		t.Fatalf("message=%q, want concrete exception advice", got[0].Message)
	}
}

func TestNoAssert_BodyHelperWithoutPrefix(t *testing.T) {
	rule := rules.NewNoAssert()
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("x"),
		ModelOK: true,
		Model: parse.Model{
			Helpers: []parse.Helper{{Name: "verify_result", QualName: "verify_result", HasAssert: true}},
			Tests: []parse.TestFunc{{
				Name: "test_x", QualName: "test_x", Lineno: 1,
				Calls: []parse.Call{{Name: "verify_result", Lineno: 2}},
			}},
		},
	})
	if len(got) != 0 {
		t.Fatalf("helper with assert body must be clean, got %v", got)
	}
}

func TestMockOnlyAssert_AdaptersPathClean(t *testing.T) {
	rule := rules.NewMockOnlyAssert()
	got := rule.Check(scan.File{
		Path:    "tests/adapters/azure/test_cred.py",
		Content: []byte("x"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_x", QualName: "test_x", Lineno: 1,
				Calls: []parse.Call{
					{Name: "compute", Lineno: 2, Bare: false},
					{Name: "mock.assert_called", Lineno: 3, Bare: true},
				},
				Asserts: []parse.Assert{{Kind: "mock_method", Lineno: 3}},
			}},
		},
	})
	if len(got) != 0 {
		t.Fatalf("adapters/ path must be clean, got %v", got)
	}
}

func TestPrivateImport_Heuristics(t *testing.T) {
	rule := rules.NewPrivateImport()
	cases := []struct {
		name string
		src  string
		hit  int
	}{
		{"from_private_symbol", "from mymodule import _helper\n", 1},
		{"from_multi_private", "from mymodule import _a, helper, _b\n", 1},
		{"import_private_submodule", "import mymodule._internal\n", 1},
		{"import_stdlib_private_toplevel", "import _thread\n", 0},
		{"import_ast_stdlib", "import _ast\n", 0},
		{"from_public", "from mymodule import helper\n", 0},
		{"dunder", "from mymodule import __version__\n", 0},
		{"upper_const", "from mymodule import _FOO_BAR, _HTTP_STATUS\n", 0},
		{"multi_line_file", "from mymodule import _helper\nimport pkg._internal\n", 2},
		{"tests_package_helper", "from tests.blueprint.test_api import _build\n", 0},
		{"relative_test_helper", "from .test_documents_api import _build\n", 0},
		{"relative_impl_still_hits", "from ..pkg.models import _Secret\n", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rule.Check(scan.File{
				Path:                   "t.py",
				Content:                []byte(tc.src),
				AllowHeuristicFallback: true,
			})
			if len(got) != tc.hit {
				t.Fatalf("want %d findings, got %d (%v)", tc.hit, len(got), got)
			}
			for _, f := range got {
				if f.Severity != "note" {
					t.Fatalf("severity=%q, want note", f.Severity)
				}
				if !strings.Contains(f.Message, "submodule") {
					t.Fatalf("message should mention submodule candidate: %q", f.Message)
				}
			}
		})
	}
}

func TestPrivateImport_AllFromModel(t *testing.T) {
	rule := rules.NewPrivateImport()
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("from m import _a, _b\nimport pkg._x\nfrom tests.x import _h\nfrom .test_y import _z\n"),
		ModelOK: true,
		Model: parse.Model{
			Imports: []parse.Import{
				{Kind: "from", Module: "m", Names: []string{"_a", "_b"}, Lineno: 1},
				{Kind: "import", Names: []string{"pkg._x"}, Lineno: 2},
				{Kind: "import", Names: []string{"_thread"}, Lineno: 3},
				{Kind: "from", Module: "tests.x", Names: []string{"_h"}, Lineno: 4},
				{Kind: "from", Module: "test_y", Names: []string{"_z"}, Level: 1, Lineno: 5},
			},
		},
	})
	if len(got) != 2 {
		t.Fatalf("want 2 findings (one per import stmt), got %d (%v)", len(got), got)
	}
	for _, f := range got {
		if f.Severity != "note" {
			t.Fatalf("severity=%q, want note", f.Severity)
		}
		if !strings.Contains(f.Message, "submodule") {
			t.Fatalf("message should mention submodule candidate: %q", f.Message)
		}
	}
	if !strings.Contains(got[0].Message, "_a") || !strings.Contains(got[0].Message, "_b") {
		t.Fatalf("aggregated finding should list both names: %q", got[0].Message)
	}
}

func TestWeakAssert_BoolCallsClean(t *testing.T) {
	rule := rules.NewWeakAssert()
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("def test_x():\n    assert obj.is_valid()\n"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_x", QualName: "test_x", Lineno: 1,
				Asserts: []parse.Assert{{
					Kind: "truthy", Text: "obj.is_valid()", Lineno: 2,
				}},
			}},
		},
	})
	if len(got) != 0 {
		t.Fatalf("method/call truthy must not be weak-assert, got %v", got)
	}
}

func TestWeakAssert_NotCollectionClean(t *testing.T) {
	rule := rules.NewWeakAssert()
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("def test_x():\n    assert not errors\n"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_x", QualName: "test_x", Lineno: 1,
				Asserts: []parse.Assert{{
					Kind: "truthy", Text: "not errors", Lineno: 2,
				}},
			}},
		},
	})
	if len(got) != 0 {
		t.Fatalf("assert not collection must not be weak-assert, got %v", got)
	}
}

func TestWeakAssert_BareTruthyIsNote(t *testing.T) {
	rule := rules.NewWeakAssert()
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("def test_x():\n    assert result\n"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_x", QualName: "test_x", Lineno: 1,
				Asserts: []parse.Assert{{
					Kind: "truthy", Text: "result", Lineno: 2,
				}},
			}},
		},
	})
	if len(got) != 1 {
		t.Fatalf("bare truthy should hit, got %v", got)
	}
	if got[0].Severity != "note" {
		t.Fatalf("severity=%q, want note", got[0].Severity)
	}
}

func TestNearDuplicate_DefaultNote(t *testing.T) {
	rule := rules.NewNearDuplicateTest()
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("def test_a():\n    assert x == 1\ndef test_b():\n    assert x == 2\n"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{
				{Name: "test_a", QualName: "test_a", Lineno: 1, BodyNorm: "assert x == N"},
				{Name: "test_b", QualName: "test_b", Lineno: 3, BodyNorm: "assert x == N"},
			},
		},
	})
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %v", got)
	}
	if got[0].Severity != "note" {
		t.Fatalf("severity=%q, want note", got[0].Severity)
	}
	if got[0].RelatedQualName != "test_a" || got[0].RelatedLine != 1 {
		t.Fatalf("related=%s:%d, want test_a:1", got[0].RelatedQualName, got[0].RelatedLine)
	}
	if !strings.Contains(got[0].Message, "test_a:1") {
		t.Fatalf("message=%q, want twin qual:line", got[0].Message)
	}
}

func TestNearDuplicate_OppositePolarityClean(t *testing.T) {
	rule := rules.NewNearDuplicateTest()
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("def test_accepts_item():\n    assert out\ndef test_rejects_item():\n    assert out\n"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{
				{Name: "test_accepts_item", QualName: "test_accepts_item", Lineno: 1, BodyNorm: "out = process(item)\nassert out"},
				{Name: "test_rejects_item", QualName: "test_rejects_item", Lineno: 3, BodyNorm: "out = process(item)\nassert out"},
			},
		},
	})
	if len(got) != 0 {
		t.Fatalf("opposite polarity must be clean, got %v", got)
	}
}

func TestNearDuplicate_DifferentEnumClean(t *testing.T) {
	rule := rules.NewNearDuplicateTest()
	got := rule.Check(scan.File{
		Path:    "t.py",
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{
				{
					Name: "test_managed", QualName: "test_managed", Lineno: 1,
					BodyNorm: `mode = resolve_auth()\nassert mode == "STR"`,
					Asserts:  []parse.Assert{{Kind: "compare", Text: `mode == "MANAGED_IDENTITY"`, Right: `"MANAGED_IDENTITY"`}},
				},
				{
					Name: "test_default", QualName: "test_default", Lineno: 4,
					BodyNorm: `mode = resolve_auth()\nassert mode == "STR"`,
					Asserts:  []parse.Assert{{Kind: "compare", Text: `mode == "DEFAULT_CHAIN"`, Right: `"DEFAULT_CHAIN"`}},
				},
			},
		},
	})
	if len(got) != 0 {
		t.Fatalf("different enum literals must be clean, got %v", got)
	}
}

func TestNearDuplicate_DifferentSUTClean(t *testing.T) {
	rule := rules.NewNearDuplicateTest()
	got := rule.Check(scan.File{
		Path:    "t.py",
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{
				{
					Name: "test_a", QualName: "test_a", Lineno: 1,
					BodyNorm: "assert resolve_product_field(row) == expected",
					Calls:    []parse.Call{{Name: "resolve_product_field"}},
				},
				{
					Name: "test_b", QualName: "test_b", Lineno: 3,
					BodyNorm: "assert resolve_product_field(row) == expected", // same BodyNorm for the filter under test
					Calls:    []parse.Call{{Name: "detect_product_filter_mode"}},
				},
			},
		},
	})
	if len(got) != 0 {
		t.Fatalf("different SUT must be clean, got %v", got)
	}
}

func TestNearDuplicate_ClusterParametrize(t *testing.T) {
	rule := rules.NewNearDuplicateTest()
	got := rule.Check(scan.File{
		Path:    "t.py",
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{
				{
					Name: "test_copy_name_prefix", QualName: "test_copy_name_prefix", Lineno: 1,
					BodyNorm: `assert copy_name(STR) == STR`,
					Asserts:  []parse.Assert{{Kind: "compare", Text: `copy_name(x) == "copy_of_x"`, Right: `"copy_of_x"`}},
					Calls:    []parse.Call{{Name: "copy_name"}},
				},
				{
					Name: "test_copy_name_suffix", QualName: "test_copy_name_suffix", Lineno: 4,
					BodyNorm: `assert copy_name(STR) == STR`,
					Asserts:  []parse.Assert{{Kind: "compare", Text: `copy_name(x) == "copy_of_x"`, Right: `"copy_of_x"`}},
					Calls:    []parse.Call{{Name: "copy_name"}},
				},
				{
					Name: "test_copy_name_middle", QualName: "test_copy_name_middle", Lineno: 7,
					BodyNorm: `assert copy_name(STR) == STR`,
					Asserts:  []parse.Assert{{Kind: "compare", Text: `copy_name(x) == "copy_of_x"`, Right: `"copy_of_x"`}},
					Calls:    []parse.Call{{Name: "copy_name"}},
				},
			},
		},
	})
	if len(got) != 1 {
		t.Fatalf("want 1 cluster finding, got %v", got)
	}
	if got[0].Line != 1 {
		t.Fatalf("cluster finding line=%d, want 1 (first test)", got[0].Line)
	}
	if !strings.Contains(got[0].Message, "parametrize") {
		t.Fatalf("message should suggest parametrize: %q", got[0].Message)
	}
	if !strings.Contains(got[0].Message, "3 tests") {
		t.Fatalf("message should mention cluster size: %q", got[0].Message)
	}
}

func TestWeakAssert_LenExactNotWeak(t *testing.T) {
	rule := rules.NewWeakAssert()
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("def test_len():\n    assert len(ids) == 200\n"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_len", QualName: "test_len", Lineno: 1,
				Asserts: []parse.Assert{{
					Kind: "compare", Text: "len(ids) == 200", Left: "len(ids)", Right: "200", Lineno: 2,
				}},
			}},
		},
	})
	if len(got) != 0 {
		t.Fatalf("len == 200 must not be weak-assert, got %v", got)
	}
}

func TestWeakAssert_AcceptsIsValidClean(t *testing.T) {
	rule := rules.NewWeakAssert()
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("def test_accepts_x():\n    assert result.is_valid\n"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_accepts_x", QualName: "test_accepts_x", Lineno: 1,
				Asserts: []parse.Assert{{
					Kind: "truthy", Text: "result.is_valid", Lineno: 2,
				}},
			}},
		},
	})
	if len(got) != 0 {
		t.Fatalf("accepts_* is_valid must not be weak-assert, got %v", got)
	}
}

func TestWeakAssert_RejectsShallowHit(t *testing.T) {
	rule := rules.NewWeakAssert()
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("def test_rejects_x():\n    assert not result.is_valid\n    assert result.errors\n"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_rejects_x", QualName: "test_rejects_x", Lineno: 1,
				Asserts: []parse.Assert{
					{Kind: "truthy", Text: "not result.is_valid", Lineno: 2},
					{Kind: "truthy", Text: "result.errors", Lineno: 3},
				},
			}},
		},
	})
	if len(got) != 1 {
		t.Fatalf("shallow rejects_* should hit, got %v", got)
	}
}

func TestWeakAssert_StatusGETHit(t *testing.T) {
	rule := rules.NewWeakAssert()
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("def test_get_item():\n    client.get('/x')\n    assert response.status_code == 200\n"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_get_item", QualName: "test_get_item", Lineno: 1, EndLineno: 3,
				Calls: []parse.Call{{Name: "client.get", Lineno: 2, Bare: true}},
				Asserts: []parse.Assert{{
					Kind: "compare", Text: "response.status_code == 200",
					Left: "response.status_code", Right: "200", Lineno: 3,
				}},
			}},
		},
	})
	if len(got) != 1 {
		t.Fatalf("status-only GET should hit, got %v", got)
	}
}

func TestOnlyHappyPath_HTTPConstClean(t *testing.T) {
	rule := rules.NewOnlyHappyPathMin(3)
	tests := make([]parse.TestFunc, 0, 4)
	for i, name := range []string{"test_a", "test_b", "test_c", "test_d"} {
		a := parse.Assert{Kind: "compare", Text: "1 == 1", Left: "1", Right: "1", Lineno: i*3 + 2}
		if i == 3 {
			a = parse.Assert{
				Kind: "compare", Text: "resp.status_code == status.HTTP_403_FORBIDDEN",
				Left: "resp.status_code", Right: "status.HTTP_403_FORBIDDEN", Lineno: i*3 + 2,
			}
		}
		tests = append(tests, parse.TestFunc{
			Name: name, QualName: name, Lineno: i*3 + 1, Asserts: []parse.Assert{a},
		})
	}
	got := rule.Check(sutFile(t, tests, "status.HTTP_403_FORBIDDEN"))
	if len(got) != 0 {
		t.Fatalf("HTTP_403 const must count as negative, got %v", got)
	}
}

func TestOnlyHappyPath_BlockedNameClean(t *testing.T) {
	rule := rules.NewOnlyHappyPathMin(3)
	tests := []parse.TestFunc{
		{Name: "test_a", QualName: "test_a", Lineno: 1, Asserts: []parse.Assert{{Kind: "compare", Text: "1 == 1", Left: "1", Right: "1", Lineno: 2}}},
		{Name: "test_b", QualName: "test_b", Lineno: 3, Asserts: []parse.Assert{{Kind: "compare", Text: "1 == 1", Left: "1", Right: "1", Lineno: 4}}},
		{Name: "test_c", QualName: "test_c", Lineno: 5, Asserts: []parse.Assert{{Kind: "compare", Text: "1 == 1", Left: "1", Right: "1", Lineno: 6}}},
		{Name: "test_returns_403_when_blocked", QualName: "test_returns_403_when_blocked", Lineno: 7,
			Asserts: []parse.Assert{{Kind: "compare", Text: "1 == 1", Left: "1", Right: "1", Lineno: 8}}},
	}
	got := rule.Check(sutFile(t, tests, "x"))
	if len(got) != 0 {
		t.Fatalf("blocked/403 name must count as negative, got %v", got)
	}
}

func TestAssertInEmptyableLoop_RangeConstClean(t *testing.T) {
	rule := rules.NewAssertInEmptyableLoop()
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("def test_x():\n    for i in range(10):\n        assert i >= 0\n"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_x", QualName: "test_x", Lineno: 1,
				ForLoops: []parse.ForLoop{{
					Lineno: 2, EndLineno: 3, OnlyAsserts: true,
					IterKind: "range_const", IterText: "range(10)",
				}},
				Asserts: []parse.Assert{{Kind: "compare", Text: "i >= 0", Left: "i", Right: "0", Lineno: 3}},
			}},
		},
	})
	if len(got) != 0 {
		t.Fatalf("range(10) must not hit emptyable-loop, got %v", got)
	}
}
