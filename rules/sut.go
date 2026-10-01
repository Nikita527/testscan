package rules

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

// ---------------------------------------------------------------------------
// SUT (system under test) definition: the ONE place that decides which calls
// in a test exercise project code. Reused by only-happy-path today; the
// data-flow milestone (depends_on_sut for no-assert / mock-only-assert /
// mock-tautology) must call SUTContext.Calls instead of re-deriving it.
//
// A call is a SUT call when ALL hold:
//
//  1. It is not a framework/mock/builtin call (pytest.*, patch, MagicMock,
//     assert*, len/str/..., see isFrameworkOrMockCall).
//  2. Its receiver is not a literal of a builtin type ("x".join, [].append).
//  3. Its root name (first dotted segment) resolves, through at most three
//     hops of local `name = <callee>(...)` assignments, to a name bound by an
//     import. Fixtures/parameters (client, mocker, self), literal- or
//     expression-valued locals (json_payload = json.dumps(...)), module-level
//     defs of the test file (private helpers like `_rows`) and unresolved
//     names (builtins) are never SUT.
//  4. That import is absolute (not `from .helpers import x`), not stdlib
//     (sys.stdlib_module_names, computed by the Python helper), not on the
//     never-SUT list (pytest, django, rest_framework, unittest, mock, ...) and
//     its top-level package is a PROJECT package: a directory with .py files
//     (or a .py module) under the project root, <root>/src, or an ancestor
//     directory of the test file, outside tests/test dirs and conftest/test_*.
//     Anything else is a third-party / test-support import.
//  5. It is not a CapWords constructor whose result is only passed into
//     another call (DTO / dataclass built as an argument, e.g.
//     `resolve(meta={k: SourceFieldMeta(...)})`): the subject is the outer call.
// ---------------------------------------------------------------------------

// SUTCall is one call that exercises project code.
type SUTCall struct {
	// Name is the call as written (`svc.run`).
	Name string
	// Resolved replaces a local variable root by the callee it was assigned
	// from (`svc = Service()` => `Service.run`); equals Name otherwise.
	Resolved string
	Lineno   int
	// Module is the absolute project module the root name was imported from.
	Module string
}

// neverSUTPackages are top-level packages that are never the system under test
// even if a same-named directory exists under the project root.
var neverSUTPackages = map[string]bool{
	"pytest": true, "_pytest": true, "django": true, "rest_framework": true,
	"unittest": true, "mock": true, "pytest_mock": true, "pytest_django": true,
	"factory": true, "faker": true, "freezegun": true, "time_machine": true,
	"hypothesis": true, "responses": true, "requests_mock": true,
	"tests": true, "test": true, "testing": true, "conftest": true,
}

// testDirNames are directory names that mark test code (never a project package).
var testDirNames = map[string]bool{"tests": true, "test": true, "testing": true, "__tests__": true}

// SUTContext resolves SUT calls for one parsed file.
type SUTContext struct {
	file     scan.File
	bindings map[string]parse.ImportBinding
	level    map[string]int // import level per local name
	defs     map[string]bool
	root     string
	absDir   string
}

// NewSUTContext prepares SUT resolution for file/model.
func NewSUTContext(file scan.File, model parse.Model) *SUTContext {
	c := &SUTContext{
		file:     file,
		bindings: map[string]parse.ImportBinding{},
		level:    map[string]int{},
		defs:     map[string]bool{},
	}
	for _, imp := range model.Imports {
		for _, b := range imp.Bindings {
			if _, ok := c.bindings[b.Local]; ok {
				continue
			}
			c.bindings[b.Local] = b
			c.level[b.Local] = imp.Level
		}
	}
	for _, d := range model.ModuleDefs {
		c.defs[d] = true
	}
	c.root = file.ProjectRoot
	if c.root == "" {
		c.root, _ = os.Getwd()
	}
	if abs, err := filepath.Abs(c.root); err == nil {
		c.root = abs
	}
	p := file.Path
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	c.absDir = filepath.Dir(p)
	return c
}

// Calls returns the SUT calls of t ordered by line (stable for equal lines).
func (c *SUTContext) Calls(t parse.TestFunc) []SUTCall {
	var out []SUTCall
	for _, call := range t.Calls {
		if sc, ok := c.classify(t, call); ok {
			out = append(out, sc)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Lineno < out[j].Lineno })
	return out
}

func firstSegment(name string) (head, rest string) {
	if i := strings.Index(name, "."); i >= 0 {
		return name[:i], name[i:]
	}
	return name, ""
}

func (c *SUTContext) classify(t parse.TestFunc, call parse.Call) (SUTCall, bool) {
	if isFrameworkOrMockCall(call.Name) || call.RecvLiteral {
		return SUTCall{}, false
	}
	resolved := call.Name
	for hop := 0; ; hop++ {
		root, rest := firstSegment(resolved)
		origin, local := t.VarOrigins[root]
		if !local {
			if containsString(t.Fixtures, root) {
				return SUTCall{}, false
			}
			break
		}
		if hop >= 3 || strings.HasPrefix(origin, "<") {
			return SUTCall{}, false
		}
		oroot, _ := firstSegment(origin)
		if oroot == root {
			return SUTCall{}, false
		}
		resolved = origin + rest
	}
	root, _ := firstSegment(resolved)
	b, ok := c.bindings[root]
	if !ok || c.level[root] > 0 || b.Stdlib || b.Module == "" {
		return SUTCall{}, false
	}
	top, _ := firstSegment(b.Module)
	if neverSUTPackages[top] || !c.isProjectPackage(top) {
		return SUTCall{}, false
	}
	leaf := leafName(call.Name)
	if call.ArgOfCall && looksLikeClassName(leaf) {
		return SUTCall{}, false
	}
	return SUTCall{Name: call.Name, Resolved: resolved, Lineno: call.Lineno, Module: b.Module}, true
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// looksLikeClassName reports CapWords (Foo, SourceFieldMeta), not UPPER_CASE.
func looksLikeClassName(name string) bool {
	name = strings.TrimLeft(name, "_")
	if name == "" || !unicode.IsUpper(rune(name[0])) {
		return false
	}
	for _, r := range name {
		if unicode.IsLower(r) {
			return true
		}
	}
	return false
}

// isProjectPackage reports whether top is a package/module of the project.
func (c *SUTContext) isProjectPackage(top string) bool {
	for _, d := range c.searchDirs() {
		if packageExists(d, top) {
			return true
		}
	}
	return false
}

// searchDirs lists import roots: project root, <root>/src, and every ancestor
// of the test file up to the root (outside test directories).
func (c *SUTContext) searchDirs() []string {
	dirs := []string{c.root, filepath.Join(c.root, "src")}
	d := c.absDir
	for i := 0; i < 10; i++ {
		if !inTestDir(c.root, d) {
			dirs = append(dirs, d, filepath.Join(d, "src"))
		}
		if d == c.root {
			break
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return dirs
}

// inTestDir reports whether dir (relative to root when under it) has a tests component.
func inTestDir(root, dir string) bool {
	rel := dir
	if r, err := filepath.Rel(root, dir); err == nil && !strings.HasPrefix(r, "..") {
		rel = r
	}
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if testDirNames[strings.ToLower(part)] {
			return true
		}
	}
	return false
}

var pkgCache sync.Map // "dir\x00top" -> bool

// packageExists: dir/top is a directory holding .py files, or dir/top.py exists.
func packageExists(dir, top string) bool {
	if top == "" || strings.HasPrefix(top, "test_") {
		return false
	}
	key := dir + "\x00" + top
	if v, ok := pkgCache.Load(key); ok {
		return v.(bool)
	}
	ok := false
	if st, err := os.Stat(filepath.Join(dir, top+".py")); err == nil && !st.IsDir() {
		ok = true
	} else if entries, err := os.ReadDir(filepath.Join(dir, top)); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".py") {
				ok = true
				break
			}
			// namespace-ish package: a sub-package with python inside
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && e.Name() != "__pycache__" {
				if sub, err := os.ReadDir(filepath.Join(dir, top, e.Name())); err == nil {
					for _, s := range sub {
						if !s.IsDir() && strings.HasSuffix(s.Name(), ".py") {
							ok = true
							break
						}
					}
				}
				if ok {
					break
				}
			}
		}
	}
	pkgCache.Store(key, ok)
	return ok
}
