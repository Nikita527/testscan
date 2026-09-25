package rules

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

type privateImport struct{}

func (privateImport) ID() string {
	return "test-imports-implementation-private"
}

func (privateImport) NeedsAST() bool { return true }

func (privateImport) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		if useHeuristic(file, err) {
			return privateImportHeuristic(file)
		}
		return nil
	}
	return privateImportFromModel(file.Path, model)
}

func privateImportFromModel(path string, model parse.Model) []scan.Finding {
	var findings []scan.Finding
	for _, imp := range model.Imports {
		switch imp.Kind {
		case "from":
			if isTestHelperImport(imp.Module, imp.Level) {
				continue
			}
			for _, name := range imp.Names {
				if !isPrivateName(name) {
					continue
				}
				findings = append(findings, privateImportFinding(path, imp.Lineno, name))
			}
		case "import":
			for _, name := range imp.Names {
				// Only submodule with a private segment.
				// Top-level `import _thread` / `_ast` (stdlib) is not a smell.
				if !strings.Contains(name, ".") || !hasPrivateSegment(name) {
					continue
				}
				if isTestHelperImport(name, 0) {
					continue
				}
				findings = append(findings, privateImportFinding(path, imp.Lineno, privateSegment(name)))
			}
		}
	}
	return findings
}

func privateImportFinding(path string, line int, name string) scan.Finding {
	msg := "test imports private implementation name"
	if name != "" {
		msg = fmt.Sprintf("test imports private implementation name %s", name)
	}
	return scan.Finding{
		File:     path,
		Line:     line,
		Rule:     "test-imports-implementation-private",
		Severity: "note",
		Message:  msg,
	}
}

func privateImportHeuristic(file scan.File) []scan.Finding {
	src := string(file.Content)
	var findings []scan.Finding
	for i, line := range strings.Split(src, "\n") {
		for _, name := range privateNamesOnLine(line) {
			findings = append(findings, privateImportFinding(file.Path, i+1, name))
		}
	}
	return findings
}

func privateNamesOnLine(line string) []string {
	t := strings.TrimSpace(line)
	if t == "" || strings.HasPrefix(t, "#") {
		return nil
	}
	var out []string
	// from pkg import _foo / from pkg import a, _b
	if strings.HasPrefix(t, "from ") && strings.Contains(t, " import ") {
		parts := strings.SplitN(t, " import ", 2)
		if len(parts) != 2 {
			return nil
		}
		modPart := strings.TrimSpace(strings.TrimPrefix(parts[0], "from "))
		level := 0
		for strings.HasPrefix(modPart, ".") {
			level++
			modPart = strings.TrimPrefix(modPart, ".")
		}
		modPart = strings.TrimSpace(modPart)
		if isTestHelperImport(modPart, level) {
			return nil
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
				out = append(out, name)
			}
		}
		return out
	}
	// import pkg._priv — only submodule with a private segment.
	// Top-level `import _thread` / `_ast` (stdlib) is not a smell.
	if strings.HasPrefix(t, "import ") {
		rest := strings.TrimSpace(strings.TrimPrefix(t, "import "))
		for _, name := range strings.Split(rest, ",") {
			name = strings.TrimSpace(name)
			if idx := strings.Index(name, " as "); idx >= 0 {
				name = strings.TrimSpace(name[:idx])
			}
			if strings.Contains(name, ".") && hasPrivateSegment(name) {
				if isTestHelperImport(name, 0) {
					continue
				}
				out = append(out, privateSegment(name))
			}
		}
	}
	return out
}

// isTestHelperImport skips private names pulled from tests.* packages or
// relative imports of sibling test modules / conftest helpers.
func isTestHelperImport(module string, level int) bool {
	if hasTestsPackageSegment(module) {
		return true
	}
	if level <= 0 {
		return false
	}
	if module == "" {
		return true
	}
	base := module
	if i := strings.LastIndex(module, "."); i >= 0 {
		base = module[i+1:]
	}
	return strings.HasPrefix(base, "test_") || base == "conftest" || strings.Contains(module, ".test_")
}

func hasTestsPackageSegment(mod string) bool {
	if mod == "" {
		return false
	}
	for _, seg := range strings.Split(mod, ".") {
		if seg == "tests" {
			return true
		}
	}
	return false
}

func privateSegment(mod string) string {
	for _, seg := range strings.Split(mod, ".") {
		if isPrivateName(seg) {
			return seg
		}
	}
	return mod
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

var _ scan.ASTRule = privateImport{}
