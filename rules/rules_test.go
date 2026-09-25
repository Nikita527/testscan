package rules_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/rules"
	"github.com/Nikita527/testscan/scan"
)

func TestRules_HitClean(t *testing.T) {
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
		{"mock-only-assert", rules.NewMockOnlyAssert(), "testdata/mock_only_assert", "test_mock_only.py", "mock-only-assert", 2},
		{"todo-test", rules.NewTodoTest(), "testdata/todo", "test_todo.py", "todo-test", 2},
		{"duplicate-test-name", rules.NewDuplicateTestName(), "testdata/duplicate_test_name", "test_dup.py", "duplicate-test-name", 5},
		{"only-happy-path", rules.NewOnlyHappyPath(), "testdata/only_happy_path", "test_happy.py", "only-happy-path", 1},
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
		{"skip-without-reason", rules.NewSkipWithoutReason(), "testdata/skip_without_reason", "test_skip.py", "skip-without-reason", 5},
		{"near-duplicate-test", rules.NewNearDuplicateTest(), "testdata/near_duplicate_test", "test_dup.py", "near-duplicate-test", 6},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := scan.Run(context.Background(), []string{tc.root}, scan.Options{
				Rules: []scan.Rule{tc.rule},
			})
			if err != nil {
				t.Fatal(err)
			}
			findings := res.Findings
			if len(findings) != 1 {
				t.Fatalf("got %d findings, want 1 (hit only): %+v", len(findings), findings)
			}
			if filepath.Base(findings[0].File) != tc.wantFile {
				t.Errorf("got file %s, want %s", findings[0].File, tc.wantFile)
			}
			if findings[0].Rule != tc.wantRule {
				t.Errorf("got rule %q, want %q", findings[0].Rule, tc.wantRule)
			}
			if findings[0].Line != tc.wantLine {
				t.Errorf("got line %d, want %d", findings[0].Line, tc.wantLine)
			}
		})
	}
}

func TestDefault(t *testing.T) {
	got := rules.Default()
	if len(got) != 22 {
		t.Fatalf("got %d rules, want 22", len(got))
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
				Kind:  "compare",
				Text:  "result is not None",
				Left:  "result",
				Right: "None",
				Lineno: i*3 + 2,
			}},
		})
	}
	got := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("def test_a():\n    assert result is not None\n"),
		ModelOK: true,
		Model:   parse.Model{Tests: tests},
	})
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
	got := rule.Check(scan.File{
		Path: "t.py", Content: []byte("x"), ModelOK: true, Model: parse.Model{Tests: tests},
	})
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


func TestPrivateImport_Heuristics(t *testing.T) {
	rule := rules.NewPrivateImport()
	cases := []struct {
		name string
		src  string
		hit  int
	}{
		{"from_private_symbol", "from mymodule import _helper\n", 1},
		{"from_multi_private", "from mymodule import _a, helper, _b\n", 2},
		{"import_private_submodule", "import mymodule._internal\n", 1},
		{"import_stdlib_private_toplevel", "import _thread\n", 0},
		{"import_ast_stdlib", "import _ast\n", 0},
		{"from_public", "from mymodule import helper\n", 0},
		{"dunder", "from mymodule import __version__\n", 0},
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
				if !strings.Contains(f.Message, "_") {
					t.Fatalf("message should include private name: %q", f.Message)
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
	if len(got) != 3 {
		t.Fatalf("want 3 findings, got %d (%v)", len(got), got)
	}
	for _, f := range got {
		if f.Severity != "note" {
			t.Fatalf("severity=%q, want note", f.Severity)
		}
		if !strings.Contains(f.Message, "_") {
			t.Fatalf("message should include private name: %q", f.Message)
		}
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
}
