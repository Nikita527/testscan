package scan_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nikita527/testscan/scan"
)

func TestWalk(t *testing.T) {
	files, err := scan.Walk(context.Background(), []string{"testdata"})
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
	files, err := scan.Walk(context.Background(), []string{"testdata"})
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
