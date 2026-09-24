package rules

import (
	"strings"
	"unicode"

	"github.com/Nikita527/testscan/scan"
)

type privateImport struct{}

func (privateImport) ID() string {
	return "test-imports-implementation-private"
}

func (privateImport) Check(file scan.File) []scan.Finding {
	src := string(file.Content)
	for i, line := range strings.Split(src, "\n") {
		if privateImportLine(line) {
			return []scan.Finding{{
				File:     file.Path,
				Line:     i + 1,
				Rule:     "test-imports-implementation-private",
				Severity: "warning",
				Message:  "test imports private implementation name",
			}}
		}
	}
	return nil
}

func privateImportLine(line string) bool {
	t := strings.TrimSpace(line)
	if t == "" || strings.HasPrefix(t, "#") {
		return false
	}
	// from pkg import _foo / from pkg import a, _b
	if strings.HasPrefix(t, "from ") && strings.Contains(t, " import ") {
		parts := strings.SplitN(t, " import ", 2)
		if len(parts) != 2 {
			return false
		}
		for _, name := range strings.Split(parts[1], ",") {
			name = strings.TrimSpace(name)
			name = strings.TrimPrefix(name, "(")
			name = strings.TrimSuffix(name, ")")
			// as-alias: "_foo as bar" → check left
			if idx := strings.Index(name, " as "); idx >= 0 {
				name = strings.TrimSpace(name[:idx])
			}
			if isPrivateName(name) {
				return true
			}
		}
		return false
	}
	// import pkg._priv — только submodule с private-сегментом.
	// Top-level `import _thread` / `_ast` (stdlib) не считаем запахом.
	if strings.HasPrefix(t, "import ") {
		rest := strings.TrimSpace(strings.TrimPrefix(t, "import "))
		for _, name := range strings.Split(rest, ",") {
			name = strings.TrimSpace(name)
			if idx := strings.Index(name, " as "); idx >= 0 {
				name = strings.TrimSpace(name[:idx])
			}
			if strings.Contains(name, ".") && hasPrivateSegment(name) {
				return true
			}
		}
	}
	return false
}

func isPrivateName(name string) bool {
	if name == "" || !strings.HasPrefix(name, "_") {
		return false
	}
	// dunder / wildcard not private-impl smell
	if strings.HasPrefix(name, "__") || name == "_" {
		return false
	}
	r := []rune(name)
	if len(r) < 2 {
		return false
	}
	return unicode.IsLetter(r[1]) || r[1] == '_'
}

func hasPrivateSegment(mod string) bool {
	for _, seg := range strings.Split(mod, ".") {
		if isPrivateName(seg) {
			return true
		}
	}
	return false
}

func NewPrivateImport() scan.Rule {
	return privateImport{}
}
