package rules_test

import (
	"path/filepath"
	"testing"

	"github.com/Nikita527/testscan/rules"
)

func TestExpectedRecomputed_CamelHelperStillHits(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test_camel.py"), `
def ComputeExpected(x):
    return x * 2


def test_camelcase_helper_recomputed():
    got = ComputeExpected(3)
    assert got == ComputeExpected(3)
`)
	res, err := testRun(t, []string{dir}, rules.NewExpectedRecomputed())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 1 || res.Findings[0].Rule != "expected-recomputed" {
		t.Fatalf("want 1 expected-recomputed for CamelCase helper, got %+v", res.Findings)
	}
}

func TestExpectedRecomputed_ValueObjectConstructorClean(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test_vo.py"), `
class RunUsage:
    def __init__(self, input_tokens=0):
        self.input_tokens = input_tokens

    def __eq__(self, other):
        return self.input_tokens == other.input_tokens


def test_usage_equals_constructor():
    got = RunUsage(input_tokens=7)
    assert got == RunUsage(input_tokens=7)
`)
	res, err := testRun(t, []string{dir}, rules.NewExpectedRecomputed())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 0 {
		t.Fatalf("value-object constructor must be clean, got %+v", res.Findings)
	}
}
