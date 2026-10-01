package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// ruffLint holds the subset of ruff lint settings relevant to rule overlap.
type ruffLint struct {
	Select               []string            `toml:"select"`
	ExtendSelect         []string            `toml:"extend-select"`
	Ignore               []string            `toml:"ignore"`
	ExtendIgnore         []string            `toml:"extend-ignore"`
	PerFileIgnores       map[string][]string `toml:"per-file-ignores"`
	ExtendPerFileIgnores map[string][]string `toml:"extend-per-file-ignores"`
}

// ruffFile is a ruff settings table: legacy top-level keys plus [lint].
type ruffFile struct {
	ruffLint
	Lint ruffLint `toml:"lint"`
}

func (r ruffFile) selects() []string {
	return concat(r.Select, r.ExtendSelect, r.Lint.Select, r.Lint.ExtendSelect)
}

func (r ruffFile) ignores() []string {
	return concat(r.Ignore, r.ExtendIgnore, r.Lint.Ignore, r.Lint.ExtendIgnore)
}

// perFileIgnores returns every per-file-ignores selector regardless of glob.
func (r ruffFile) perFileIgnores() []string {
	var out []string
	for _, m := range []map[string][]string{
		r.PerFileIgnores, r.ExtendPerFileIgnores,
		r.Lint.PerFileIgnores, r.Lint.ExtendPerFileIgnores,
	} {
		for _, codes := range m {
			out = append(out, codes...)
		}
	}
	return out
}

func concat(parts ...[]string) []string {
	var out []string
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// RuffEnables reports whether the ruff configuration found from startDir
// (walking up parents; per directory: .ruff.toml, ruff.toml, then pyproject.toml
// with [tool.ruff]) enables the given code (e.g. "PT011") by prefix selection
// and does not ignore it. Among selectors matching the code the most specific
// (longest prefix; "ALL" is least specific) wins and a tie goes to ignore.
// Any per-file-ignores entry covering the code makes it NOT enabled (testscan
// keeps its own rule — conservative). No ruff config, or a malformed config
// candidate (the walk stops there), → false.
func RuffEnables(startDir, code string) bool {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return false
	}
	for {
		for _, name := range []string{".ruff.toml", "ruff.toml"} {
			p := filepath.Join(dir, name)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				var rf ruffFile
				if _, err := toml.DecodeFile(p, &rf); err != nil {
					return false
				}
				return ruffCodeEnabled(rf, code)
			}
		}
		p := filepath.Join(dir, "pyproject.toml")
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			var py struct {
				Tool struct {
					Ruff *ruffFile `toml:"ruff"`
				} `toml:"tool"`
			}
			if _, err := toml.DecodeFile(p, &py); err != nil {
				return false
			}
			if py.Tool.Ruff != nil {
				return ruffCodeEnabled(*py.Tool.Ruff, code)
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

func ruffCodeEnabled(rf ruffFile, code string) bool {
	// specificity returns the match length (ALL = 0) or -1 for no match.
	specificity := func(sel string) int {
		sel = strings.TrimSpace(sel)
		switch {
		case sel == "ALL":
			return 0
		case sel != "" && strings.HasPrefix(code, sel):
			return len(sel)
		}
		return -1
	}
	for _, c := range rf.perFileIgnores() {
		if specificity(c) >= 0 {
			return false
		}
	}
	bestSel, bestIgn := -1, -1
	for _, s := range rf.selects() {
		if n := specificity(s); n > bestSel {
			bestSel = n
		}
	}
	for _, i := range rf.ignores() {
		if n := specificity(i); n > bestIgn {
			bestIgn = n
		}
	}
	return bestSel >= 0 && bestSel > bestIgn
}
