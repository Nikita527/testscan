package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nikita527/testscan/rules"
)

// TestNearDuplicate_Semantics runs the rule through the real parser on small
// sources: only adjacent, input-only-different, same-assertion twins count.
func TestNearDuplicate_Semantics(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want int
	}{
		{
			name: "input literals differ only",
			src: `def test_a(client):
    r = client.get("/a")
    assert r.status_code == 404


def test_b(client):
    r = client.get("/b")
    assert r.status_code == 404
`,
			want: 1,
		},
		{
			name: "expected values differ",
			src: `def test_a():
    r = f("a")
    assert r == 1


def test_b():
    r = f("b")
    assert r == 2
`,
			want: 0,
		},
		{
			name: "status differs",
			src: `def test_a(client):
    r = client.get("/a")
    assert r.status_code == 404


def test_b(client):
    r = client.get("/b")
    assert r.status_code == 403
`,
			want: 0,
		},
		{
			name: "raises match differs",
			src: `def test_a():
    with pytest.raises(ValueError, match="one"):
        f("a")


def test_b():
    with pytest.raises(ValueError, match="two"):
        f("b")
`,
			want: 0,
		},
		{
			name: "raises match same",
			src: `def test_a():
    with pytest.raises(ValueError, match="bad"):
        f("a")


def test_b():
    with pytest.raises(ValueError, match="bad"):
        f("b")
`,
			want: 1,
		},
		{
			name: "decorators differ",
			src: `@pytest.mark.slow
def test_a():
    r = f("a")
    assert r


def test_b():
    r = f("b")
    assert r
`,
			want: 0,
		},
		{
			name: "docstrings differ",
			src: `def test_a():
    """Scenario one."""
    r = f("a")
    assert r


def test_b():
    """Scenario two."""
    r = f("b")
    assert r
`,
			want: 0,
		},
		{
			name: "docstring only on one side",
			src: `def test_a():
    """Scenario one."""
    r = f("a")
    assert r


def test_b():
    r = f("b")
    assert r
`,
			want: 1,
		},
		{
			name: "antonym names",
			src: `def test_job_demotes():
    r = f("a")
    assert r


def test_job_retries():
    r = f("b")
    assert r
`,
			want: 0,
		},
		{
			name: "negation in one name",
			src: `def test_col_is_null():
    r = f("a")
    assert r


def test_col_is_not_null():
    r = f("b")
    assert r
`,
			want: 0,
		},
		{
			name: "far apart",
			src: `def test_a():
    r = f("a")
    assert r


def test_x():
    assert g() == 1


def test_y():
    assert g() == 2


def test_b():
    r = f("b")
    assert r
`,
			want: 0,
		},
		{
			name: "one test in between",
			src: `def test_a():
    r = f("a")
    assert r


def test_x():
    assert g() == 1


def test_b():
    r = f("b")
    assert r
`,
			want: 1,
		},
		{
			name: "different classes",
			src: `class TestA:
    def test_one(self):
        r = f("a")
        assert r


class TestB:
    def test_two(self):
        r = f("b")
        assert r
`,
			want: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "test_x.py"), []byte(tc.src), 0o600); err != nil {
				t.Fatal(err)
			}
			res, err := testRun(t, []string{dir}, rules.NewNearDuplicateTest())
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Findings) != tc.want {
				t.Fatalf("got %d findings, want %d: %+v", len(res.Findings), tc.want, res.Findings)
			}
			for _, f := range res.Findings {
				if !strings.Contains(f.Message, "@pytest.mark.parametrize") {
					t.Fatalf("message should suggest parametrize: %q", f.Message)
				}
			}
		})
	}
}
