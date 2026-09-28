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
		names := privateNamesFromImport(imp)
		if len(names) == 0 {
			continue
		}
		findings = append(findings, privateImportFinding(path, imp.Lineno, moduleForPrivateImport(imp), names))
	}
	return findings
}

func moduleForPrivateImport(imp parse.Import) string {
	if imp.Module != "" {
		return imp.Module
	}
	if imp.Kind != "import" {
		return ""
	}
	for _, name := range imp.Names {
		if !strings.Contains(name, ".") || !hasPrivateSegment(name) {
			continue
		}
		if i := strings.LastIndex(name, "."); i > 0 {
			return name[:i]
		}
	}
	return ""
}

func privateNamesFromImport(imp parse.Import) []string {
	var names []string
	switch imp.Kind {
	case "from":
		if isTestHelperImport(imp.Module, imp.Level) {
			return nil
		}
		for _, name := range imp.Names {
			if isPrivateName(name) {
				names = append(names, name)
			}
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
			seg := privateSegment(name)
			if seg != "" {
				names = append(names, seg)
			}
		}
	}
	return names
}

func privateImportFinding(path string, line int, module string, names []string) scan.Finding {
	mod := strings.TrimSpace(module)
	if mod == "" && len(names) == 1 {
		// import pkg._internal — module inferred from dotted path segment parent.
		mod = "implementation"
	}
	n := len(names)
	var msg string
	switch {
	case mod != "" && mod != "implementation":
		msg = fmt.Sprintf(
			"module %s has %d private name(s) imported by tests (%s) — candidate for a public submodule",
			mod, n, strings.Join(names, ", "),
		)
	default:
		msg = fmt.Sprintf(
			"%d private name(s) imported by tests (%s) — candidate for a public submodule",
			n, strings.Join(names, ", "),
		)
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
		mod, names := privateNamesOnLine(line)
		if len(names) == 0 {
			continue
		}
		findings = append(findings, privateImportFinding(file.Path, i+1, mod, names))
	}
	return findings
}

// privateNamesOnLine returns module (may be empty) and private names on one import line.
func privateNamesOnLine(line string) (module string, names []string) {
	t := strings.TrimSpace(line)
	if t == "" || strings.HasPrefix(t, "#") {
		return "", nil
	}
	// from pkg import _foo / from pkg import a, _b
	if strings.HasPrefix(t, "from ") && strings.Contains(t, " import ") {
		parts := strings.SplitN(t, " import ", 2)
		if len(parts) != 2 {
			return "", nil
		}
		modPart := strings.TrimSpace(strings.TrimPrefix(parts[0], "from "))
		level := 0
		for strings.HasPrefix(modPart, ".") {
			level++
			modPart = strings.TrimPrefix(modPart, ".")
		}
		modPart = strings.TrimSpace(modPart)
		if isTestHelperImport(modPart, level) {
			return "", nil
		}
		var out []string
		for _, name := range strings.Split(parts[1], ",") {
			name = strings.TrimSpace(name)
			name = strings.TrimPrefix(name, "(")
			name = strings.TrimSuffix(name, ")")
			if idx := strings.Index(name, " as "); idx >= 0 {
				name = strings.TrimSpace(name[:idx])
			}
			if isPrivateName(name) {
				out = append(out, name)
			}
		}
		return modPart, out
	}
	// import pkg._priv — only submodule with a private segment.
	if strings.HasPrefix(t, "import ") {
		rest := strings.TrimSpace(strings.TrimPrefix(t, "import "))
		var out []string
		mod := ""
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
				if mod == "" {
					if i := strings.LastIndex(name, "."); i > 0 {
						mod = name[:i]
					}
				}
			}
		}
		return mod, out
	}
	return "", nil
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
	if !(unicode.IsLetter(r[1]) || r[1] == '_') {
		return false
	}
	// _UPPER_CASE constants (import instead of duplicating a literal) are fine.
	if isPrivateUpperConst(name) {
		return false
	}
	return true
}

// isPrivateUpperConst reports names like _FOO, _HTTP_STATUS (leading _ + UPPER/digits/_).
func isPrivateUpperConst(name string) bool {
	rest := strings.TrimPrefix(name, "_")
	if rest == "" {
		return false
	}
	hasLetter := false
	for _, r := range rest {
		switch {
		case unicode.IsUpper(r):
			hasLetter = true
		case unicode.IsDigit(r) || r == '_':
			// ok
		default:
			return false
		}
	}
	return hasLetter
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
