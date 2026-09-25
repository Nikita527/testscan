package scan_test

import (
	"testing"

	"github.com/Nikita527/testscan/scan"
)

func TestFilterInlineIgnores_LineAndDef(t *testing.T) {
	src := []byte(`# testscan: ignore-file[snapshot-only]

def test_a():
    assert True  # testscan: ignore[assert-true]

def test_b():  # testscan: ignore[empty-test]
    pass

def test_c():
    pass
`)
	findings := []scan.Finding{
		{File: "t.py", Line: 4, Rule: "assert-true"},
		{File: "t.py", Line: 6, Rule: "empty-test"},
		{File: "t.py", Line: 9, Rule: "empty-test"},
		{File: "t.py", Line: 1, Rule: "snapshot-only"},
	}
	got := scan.FilterInlineIgnores(findings, map[string][]byte{"t.py": src})
	if len(got) != 1 {
		t.Fatalf("got %d, want 1 remaining: %v", len(got), got)
	}
	if got[0].Line != 9 || got[0].Rule != "empty-test" {
		t.Fatalf("got %+v", got[0])
	}
}

func TestFilterInlineIgnores_MultiRule(t *testing.T) {
	src := []byte("x = 1  # testscan: ignore[a, b]\n")
	findings := []scan.Finding{
		{File: "t.py", Line: 1, Rule: "a"},
		{File: "t.py", Line: 1, Rule: "b"},
		{File: "t.py", Line: 1, Rule: "c"},
	}
	got := scan.FilterInlineIgnores(findings, map[string][]byte{"t.py": src})
	if len(got) != 1 || got[0].Rule != "c" {
		t.Fatalf("got %v", got)
	}
}

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pat, name string
		want      bool
	}{
		{"test_*.py", "test_foo.py", true},
		{"*_test.py", "bar_test.py", true},
		{"**/conftest.py", "tests/conftest.py", true},
		{"**/conftest.py", "conftest.py", true},
		{"tests/integration/**", "tests/integration/a.py", true},
		{"tests/integration/**", "tests/unit/a.py", false},
	}
	for _, tc := range cases {
		if got := scan.MatchGlob(tc.pat, tc.name); got != tc.want {
			t.Errorf("MatchGlob(%q,%q)=%v want %v", tc.pat, tc.name, got, tc.want)
		}
	}
}

func TestComputeFingerprint_Stable(t *testing.T) {
	content := []byte("def test_a():\n    pass\n")
	f := scan.Finding{File: "t.py", Line: 1, Rule: "empty-test", QualName: "test_a"}
	a := scan.ComputeFingerprint(f, content, "")
	b := scan.ComputeFingerprint(f, content, "")
	if a == "" || a != b {
		t.Fatalf("unstable fingerprint %q vs %q", a, b)
	}
	f2 := f
	f2.QualName = "test_b"
	if scan.ComputeFingerprint(f2, content, "") == a {
		t.Fatal("qualname should change fingerprint")
	}
}
