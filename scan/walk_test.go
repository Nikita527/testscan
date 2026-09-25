package scan_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nikita527/testscan/scan"
)

func TestWalk(t *testing.T) {
	files, err := scan.Walk(context.Background(), []string{"testdata"}, scan.WalkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}

	got := map[string]bool{}
	for _, f := range files {
		got[filepath.Base(f.Path)] = true
		if len(f.Content) == 0 {
			t.Errorf("%s: empty content", f.Path)
		}
	}
	for _, name := range []string{"test_foo.py", "bar_test.py"} {
		if !got[name] {
			t.Errorf("missing %s", name)
		}
	}
}

func TestWalk_SkipVenv(t *testing.T) {
	files, err := scan.Walk(context.Background(), []string{"testdata"}, scan.WalkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		p := filepath.ToSlash(f.Path)
		if strings.Contains(p, "/.venv/") || strings.HasPrefix(p, ".venv/") {
			t.Fatalf("Walk must skip .venv, got %s", f.Path)
		}
		if filepath.Base(f.Path) == "test_skip.py" {
			t.Fatalf("Walk must skip .venv/test_skip.py, got %s", f.Path)
		}
	}
}

func TestWalk_Exclude(t *testing.T) {
	files, err := scan.Walk(context.Background(), []string{"testdata"}, scan.WalkOptions{
		Exclude: []string{"**/bar_test.py"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if filepath.Base(f.Path) == "bar_test.py" {
			t.Fatalf("exclude failed, got %s", f.Path)
		}
	}
}

func TestWalk_Gitignore(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored_test.py\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "test_ok.py"), []byte("def test_a():\n    assert 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignored_test.py"), []byte("def test_b():\n    assert 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := scan.Walk(context.Background(), []string{dir}, scan.WalkOptions{
		RespectGitignore: true,
		Root:             dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if filepath.Base(f.Path) == "ignored_test.py" {
			t.Fatalf("gitignore failed, got %s", f.Path)
		}
	}
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1", len(files))
	}
}
