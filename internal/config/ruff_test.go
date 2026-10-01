package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRuffEnables(t *testing.T) {
	cases := []struct {
		name string
		file string
		body string
		want bool
	}{
		{"pyproject PT", "pyproject.toml", "[tool.ruff.lint]\nselect = [\"E\", \"PT\"]\n", true},
		{"pyproject PT0", "pyproject.toml", "[tool.ruff.lint]\nextend-select = [\"PT0\"]\n", true},
		{"pyproject PT01", "pyproject.toml", "[tool.ruff.lint]\nselect = [\"PT01\"]\n", true},
		{"pyproject PT011", "pyproject.toml", "[tool.ruff.lint]\nselect = [\"PT011\"]\n", true},
		{"pyproject ALL", "pyproject.toml", "[tool.ruff.lint]\nselect = [\"ALL\"]\n", true},
		{"legacy select", "pyproject.toml", "[tool.ruff]\nselect = [\"PT\"]\n", true},
		{"ignore PT011", "pyproject.toml", "[tool.ruff.lint]\nselect = [\"PT\"]\nignore = [\"PT011\"]\n", false},
		{"ignore PT prefix", "pyproject.toml", "[tool.ruff.lint]\nselect = [\"ALL\"]\nextend-ignore = [\"PT\"]\n", false},
		{"ignore other PT", "pyproject.toml", "[tool.ruff.lint]\nselect = [\"PT\"]\nignore = [\"PT012\"]\n", true},
		{"mp-be like", "pyproject.toml", "[tool.ruff.lint]\nselect = [\"E4\",\"E7\",\"E9\",\"F\",\"I\"]\n", false},
		{"no select", "pyproject.toml", "[tool.ruff]\nline-length = 100\n", false},
		{"no ruff section", "pyproject.toml", "[tool.other]\nx = 1\n", false},
		{"ruff.toml lint", "ruff.toml", "[lint]\nselect = [\"PT\"]\n", true},
		{"ruff.toml top-level", "ruff.toml", "select = [\"PT011\"]\n", true},
		{".ruff.toml ignore", ".ruff.toml", "[lint]\nselect = [\"ALL\"]\nignore = [\"PT011\"]\n", false},
		{"specific extend-select beats broader ignore", "pyproject.toml", `
[tool.ruff.lint]
select = ["ALL"]
ignore = ["PT"]
extend-select = ["PT011"]
`, true},
		{"specific ignore beats broader select", "pyproject.toml", `
[tool.ruff.lint]
select = ["PT"]
ignore = ["PT011"]
`, false},
		{"tie goes to ignore", "pyproject.toml", `
[tool.ruff.lint]
select = ["PT"]
ignore = ["PT"]
`, false},
		{"ALL ignore loses to specific select", "pyproject.toml", `
[tool.ruff.lint]
select = ["PT"]
ignore = ["ALL"]
`, true},
		{"per-file-ignores PT011", "pyproject.toml", `
[tool.ruff.lint]
select = ["PT"]
[tool.ruff.lint.per-file-ignores]
"tests/*" = ["PT011"]
`, false},
		{"per-file-ignores PT prefix", "ruff.toml", `
[lint]
select = ["PT"]
[lint.per-file-ignores]
"tests/*" = ["PT"]
`, false},
		{"extend-per-file-ignores top-level", "ruff.toml", `
select = ["PT"]
[extend-per-file-ignores]
"tests/*" = ["PT011"]
`, false},
		{"per-file-ignores other code", "ruff.toml", `
[lint]
select = ["PT"]
[lint.per-file-ignores]
"tests/*" = ["S101"]
`, true},
		{"malformed ruff.toml", "ruff.toml", "select = [\"PT\"\n", false},
		{"malformed pyproject", "pyproject.toml", "[tool.ruff.lint\nselect = [\"PT\"]\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, tc.file), []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			if got := RuffEnables(dir, "PT011"); got != tc.want {
				t.Fatalf("RuffEnables = %v, want %v", got, tc.want)
			}
		})
	}
	t.Run("malformed stops the walk", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "ruff.toml"), []byte("[lint]\nselect = [\"PT\"]\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		sub := filepath.Join(root, "sub")
		if err := os.Mkdir(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sub, "pyproject.toml"), []byte("[tool.ruff\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if RuffEnables(sub, "PT011") {
			t.Fatal("malformed child config must stop the walk (not enabled)")
		}
	})
	t.Run("none", func(t *testing.T) {
		if RuffEnables(t.TempDir(), "PT011") {
			t.Fatal("want false without ruff config")
		}
	})
}

func TestDeferToRuffConfig(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(dir)
	if err != nil || !cfg.DeferToRuff {
		t.Fatalf("default: err=%v defer=%v", err, cfg.DeferToRuff)
	}
	for body, want := range map[string]bool{
		"defer_to_ruff = false\n": false,
		"defer-to-ruff = false\n": false,
		"defer_to_ruff = true\n":  true,
		"fail-on = \"never\"\n":   true,
	} {
		d := t.TempDir()
		if err := os.WriteFile(filepath.Join(d, ".testscan.toml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		c, err := Load(d)
		if err != nil || c.DeferToRuff != want {
			t.Fatalf("%q: err=%v defer=%v want %v", body, err, c.DeferToRuff, want)
		}
	}
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "pyproject.toml"), []byte("[tool.testscan]\ndefer_to_ruff = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if c, err := Load(d); err != nil || c.DeferToRuff {
		t.Fatalf("pyproject: err=%v defer=%v", err, c.DeferToRuff)
	}
}
