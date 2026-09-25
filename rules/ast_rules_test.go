package rules_test

import (
	"testing"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/rules"
	"github.com/Nikita527/testscan/scan"
)

func TestEmptyTest_ASTMock(t *testing.T) {
	got := rules.NewEmptyTest().Check(scan.File{
		Path:    "t.py",
		Content: []byte("def test_vacuous():\n    pass\n"),
		ModelOK: true,
		Model: parse.Model{Tests: []parse.TestFunc{
			{Name: "test_vacuous", QualName: "test_vacuous", Lineno: 1, IsEmpty: true},
			{Name: "test_ok", QualName: "test_ok", Lineno: 5, IsEmpty: false, HasAssert: true},
		}},
	})
	if len(got) != 1 || got[0].Line != 1 || got[0].Rule != "empty-test" {
		t.Fatalf("got %v", got)
	}
}

func TestDuplicateTestName_ASTMock(t *testing.T) {
	got := rules.NewDuplicateTestName().Check(scan.File{
		Path:    "t.py",
		Content: []byte("x"),
		ModelOK: true,
		Model: parse.Model{Tests: []parse.TestFunc{
			{Name: "test_foo", QualName: "test_foo", Lineno: 1},
			{Name: "test_foo", QualName: "test_foo", Lineno: 5},
		}},
	})
	if len(got) != 1 || got[0].Line != 5 {
		t.Fatalf("got %v", got)
	}
}

func TestDuplicateTestName_DifferentClassesClean(t *testing.T) {
	got := rules.NewDuplicateTestName().Check(scan.File{
		Path:    "t.py",
		Content: []byte("x"),
		ModelOK: true,
		Model: parse.Model{Tests: []parse.TestFunc{
			{Name: "test_anonymous_is_denied", QualName: "TestA.test_anonymous_is_denied", Lineno: 2},
			{Name: "test_anonymous_is_denied", QualName: "TestB.test_anonymous_is_denied", Lineno: 6},
		}},
	})
	if len(got) != 0 {
		t.Fatalf("want clean, got %v", got)
	}
}

func TestEmptyTest_FallbackOnUnavailable(t *testing.T) {
	got := rules.NewEmptyTest().Check(scan.File{
		Path:                   "t.py",
		Content:                []byte("def test_x():\n    pass\n\ndef test_y():\n    assert 1 == 1\n"),
		AllowHeuristicFallback: true,
		ModelOK:                true,
		ModelErr:               parse.ErrUnavailable,
	})
	if len(got) != 1 || got[0].Line != 1 {
		t.Fatalf("heuristic fallback want 1 hit at L1, got %v", got)
	}
}

func TestEmptyTest_NoFallbackByDefault(t *testing.T) {
	got := rules.NewEmptyTest().Check(scan.File{
		Path:     "t.py",
		Content:  []byte("def test_x():\n    pass\n"),
		ModelOK:  true,
		ModelErr: parse.ErrUnavailable,
	})
	if len(got) != 0 {
		t.Fatalf("without AllowHeuristicFallback want 0, got %v", got)
	}
}
