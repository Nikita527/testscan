package rules_test

import (
	"context"
	"path/filepath"
	"testing"

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
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings, err := scan.Run(context.Background(), []string{tc.root}, scan.Options{
				Rules: []scan.Rule{tc.rule},
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(findings) != 1 {
				t.Fatalf("got %d findings, want 1 (hit only)", len(findings))
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
	if len(got) != 8 {
		t.Fatalf("got %d rules, want 8", len(got))
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
			got := rule.Check(scan.File{Path: "t.py", Content: []byte(tc.src)})
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
		Path:    "t.py",
		Content: []byte("def test_x():\n    assert False\n"),
	})
	if len(bare) != 0 {
		t.Fatalf("bare assert False must not hit, got %v", bare)
	}
	withTODO := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("def test_x():\n    assert False, \"TODO\"\n"),
	})
	if len(withTODO) != 1 {
		t.Fatalf("assert False, TODO must hit, got %d", len(withTODO))
	}
}
