package scan

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFilterTestPaths(t *testing.T) {
	got := FilterTestPaths([]string{
		"src/app.py",
		"tests/test_foo.py",
		"pkg/bar_test.py",
		"README.md",
	}, nil)
	if len(got) != 2 {
		t.Fatalf("got %v, want 2 test paths", got)
	}
}

func TestDiffChangedPaths(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("checkout", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "test_a.py"), []byte("def test_a():\n    assert 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.py"), []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "init")

	if err := os.WriteFile(filepath.Join(dir, "test_b.py"), []byte("def test_b():\n    assert 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "test_b.py")
	run("commit", "-m", "add test_b")

	changed, err := DiffChangedPaths("main~1", dir)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range changed {
		if filepath.Base(p) == "test_b.py" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected test_b.py in %v", changed)
	}
	tests := FilterTestPaths(changed, nil)
	if len(tests) != 1 || filepath.Base(tests[0]) != "test_b.py" {
		t.Fatalf("FilterTestPaths=%v", tests)
	}
}

func TestWalk_OnlyPaths(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test_keep.py"), []byte("def test_a():\n    assert 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "test_skip.py"), []byte("def test_b():\n    assert 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := Walk(context.Background(), []string{dir}, WalkOptions{
		Root:      dir,
		OnlyPaths: []string{"test_keep.py"}, // suffix match against rel/abs
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || filepath.Base(files[0].Path) != "test_keep.py" {
		t.Fatalf("got %+v", files)
	}
}

func TestDiffChangedPaths_RejectsDashRef(t *testing.T) {
	_, err := DiffChangedPaths("--output=/tmp/x", ".")
	if err == nil {
		t.Fatal("expected error for dash-prefixed ref")
	}
}

func TestPathInOnly_NoBasenameOnly(t *testing.T) {
	// Same basename in different dirs must not both match a path that is only the basename
	// when that basename is used as a full relative path elsewhere — suffix "/test_keep.py"
	// still matches intentionally for git-relative paths.
	if pathInOnly("/repo/a/test_x.py", "a/test_x.py", "test_x.py", []string{"b/test_x.py"}) {
		t.Fatal("unrelated path must not match")
	}
	if !pathInOnly("/repo/b/test_x.py", "b/test_x.py", "test_x.py", []string{"b/test_x.py"}) {
		t.Fatal("exact rel must match")
	}
}
