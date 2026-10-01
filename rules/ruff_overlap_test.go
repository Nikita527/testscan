package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Nikita527/testscan/internal/config"
)

func TestDeferToRuff(t *testing.T) {
	dir := t.TempDir()
	body := "[tool.ruff.lint]\nselect = [\"PT\"]\n"
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	has := func(rs []string) bool {
		for _, id := range rs {
			if id == "broad-raises" {
				return true
			}
		}
		return false
	}
	ids := func(cfg config.Config, explicit []string) []string {
		var out []string
		for _, r := range DeferToRuff(All(), cfg, dir, explicit) {
			out = append(out, r.ID())
		}
		return out
	}
	if !has(ids(config.Config{}, nil)) {
		t.Fatal("defer off (zero config) must keep broad-raises")
	}
	if has(ids(config.Config{DeferToRuff: true}, nil)) {
		t.Fatal("PT selected: broad-raises must be dropped")
	}
	if !has(ids(config.Config{DeferToRuff: true}, []string{"broad-raises"})) {
		t.Fatal("explicit --rule/--enable must keep broad-raises")
	}
	other := t.TempDir()
	if !has(ids2(other)) {
		t.Fatal("no ruff config must keep broad-raises")
	}
}

func ids2(dir string) []string {
	var out []string
	for _, r := range DeferToRuff(All(), config.Config{DeferToRuff: true}, dir, nil) {
		out = append(out, r.ID())
	}
	return out
}
