package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Nikita527/testscan/internal/config"
)

func TestLoad_DotFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".testscan.toml")
	content := `
fail-on = "warning"
disable = ["no-assert", "empty-test"]
paths = ["tests", "src"]
workers = 4
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.FailOn != "warning" || cfg.Workers != 4 {
		t.Fatalf("got %+v", cfg)
	}
	if len(cfg.Disable) != 2 || cfg.Disable[0] != "no-assert" {
		t.Fatalf("disable=%v", cfg.Disable)
	}
	if len(cfg.Paths) != 2 || cfg.Paths[0] != filepath.Join(dir, "tests") {
		t.Fatalf("paths=%v, want under %s", cfg.Paths, dir)
	}
	if cfg.Paths[1] != filepath.Join(dir, "src") {
		t.Fatalf("paths[1]=%v", cfg.Paths[1])
	}
	if cfg.Source != path {
		t.Fatalf("source=%q, want %q", cfg.Source, path)
	}
}

func TestLoad_FailOnSnake(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".testscan.toml"), []byte("fail_on = \"never\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.FailOn != "never" {
		t.Fatalf("fail-on=%q", cfg.FailOn)
	}
}

func TestLoad_Pyproject(t *testing.T) {
	dir := t.TempDir()
	content := `
[project]
name = "demo"

[tool.testscan]
fail-on = "error"
disable = ["todo-test"]
paths = ["unit"]
workers = 2
`
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.FailOn != "error" || cfg.Workers != 2 || len(cfg.Disable) != 1 || cfg.Disable[0] != "todo-test" {
		t.Fatalf("got %+v", cfg)
	}
	if len(cfg.Paths) != 1 || cfg.Paths[0] != filepath.Join(dir, "unit") {
		t.Fatalf("paths=%v", cfg.Paths)
	}
}

func TestLoad_DotPrefersOverPyproject(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".testscan.toml"), []byte("fail-on = \"never\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[tool.testscan]\nfail-on = \"warning\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.FailOn != "never" {
		t.Fatalf("want .testscan.toml to win, got %q", cfg.FailOn)
	}
}

func TestLoad_WalkParents(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".testscan.toml"), []byte("workers = 7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(child)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Workers != 7 {
		t.Fatalf("workers=%d, want 7", cfg.Workers)
	}
}

func TestLoad_Missing(t *testing.T) {
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.FailOn != "" || cfg.Workers != 0 || len(cfg.Disable) != 0 || cfg.Source != "" {
		t.Fatalf("want empty config, got %+v", cfg)
	}
}

func TestLoad_InvalidFailOn(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".testscan.toml"), []byte("fail-on = \"loud\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("want error")
	}
}

func TestLoad_PyprojectWithoutTool(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\nname = \"x\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Source != "" {
		t.Fatalf("want no config, got source %q", cfg.Source)
	}
}

func TestLoad_PathsRelativeToConfigDir(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".testscan.toml"), []byte("paths = [\"tests\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(root, "pkg", "sub")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(child)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "tests")
	if len(cfg.Paths) != 1 || cfg.Paths[0] != want {
		t.Fatalf("paths=%v, want [%s] (resolved from config dir, not cwd)", cfg.Paths, want)
	}
}

func TestLoad_AbsolutePathUnchanged(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(dir, "abs_tests")
	// forward slashes — portable in TOML string on Windows
	toml := "paths = [\"" + filepath.ToSlash(abs) + "\"]\n"
	if err := os.WriteFile(filepath.Join(dir, ".testscan.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Paths) != 1 {
		t.Fatalf("paths=%v", cfg.Paths)
	}
	got, err := filepath.Abs(cfg.Paths[0])
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(abs)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("paths[0]=%q, want %q", got, want)
	}
}
