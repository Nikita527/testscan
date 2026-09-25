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
	if !cfg.RespectGitignore {
		t.Fatal("RespectGitignore should default true")
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

func TestLoad_ExtendedFields(t *testing.T) {
	dir := t.TempDir()
	content := `
fail-on = "warning"
exclude = ["**/conftest.py"]
assert-helpers = ["assert_*", "check_*"]
python-files = ["test_*.py", "*_test.py"]
python-functions = ["test_*"]
python-classes = ["Test*"]
respect-gitignore = false

[rules.only-happy-path]
severity = "note"
min-tests = 5
mode = "coverage"
coverage = "coverage.json"
negative-names = ["invalid", "forbidden"]

[rules.todo-test]
severity = "error"

[[overrides]]
path = "tests/integration/**"
disable = ["only-happy-path"]
`
	if err := os.WriteFile(filepath.Join(dir, ".testscan.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RespectGitignore {
		t.Fatal("want respect-gitignore false")
	}
	if len(cfg.Exclude) != 1 || cfg.Exclude[0] != "**/conftest.py" {
		t.Fatalf("exclude=%v", cfg.Exclude)
	}
	if len(cfg.AssertHelpers) != 2 {
		t.Fatalf("assert-helpers=%v", cfg.AssertHelpers)
	}
	oh := cfg.Rules["only-happy-path"]
	if oh.Severity != "note" || oh.MinTests != 5 {
		t.Fatalf("only-happy-path=%+v", oh)
	}
	if oh.Mode != "coverage" {
		t.Fatalf("mode=%q, want coverage", oh.Mode)
	}
	wantCov := filepath.Join(dir, "coverage.json")
	if oh.Coverage != wantCov {
		t.Fatalf("coverage=%q, want %q", oh.Coverage, wantCov)
	}
	if len(oh.NegativeNames) != 2 || oh.NegativeNames[0] != "invalid" {
		t.Fatalf("negative-names=%v", oh.NegativeNames)
	}
	if cfg.Rules["todo-test"].Severity != "error" {
		t.Fatalf("todo-test=%+v", cfg.Rules["todo-test"])
	}
	if len(cfg.Overrides) != 1 || cfg.Overrides[0].Path != "tests/integration/**" {
		t.Fatalf("overrides=%v", cfg.Overrides)
	}
}

func TestLoad_PytestIniOptionsFallback(t *testing.T) {
	dir := t.TempDir()
	content := `
[tool.testscan]
fail-on = "error"

[tool.pytest.ini_options]
python_files = ["check_*.py"]
python_functions = ["check_*"]
python_classes = ["Check*"]
`
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.PythonFiles) != 1 || cfg.PythonFiles[0] != "check_*.py" {
		t.Fatalf("python-files=%v", cfg.PythonFiles)
	}
	if len(cfg.PythonFunctions) != 1 || cfg.PythonFunctions[0] != "check_*" {
		t.Fatalf("python-functions=%v", cfg.PythonFunctions)
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

func TestLoad_InvalidOnlyHappyPathMode(t *testing.T) {
	dir := t.TempDir()
	content := `
[rules.only-happy-path]
mode = "fast"
`
	if err := os.WriteFile(filepath.Join(dir, ".testscan.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("want error for invalid mode")
	}
}
