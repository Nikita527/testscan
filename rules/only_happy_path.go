package rules

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

const onlyHappyPathMinTestsDefault = 3

var (
	defaultNegativeNames = []string{
		"invalid", "forbidden", "missing", "denied", "unauthorized",
		"not_", "_not", "reject", "rejects", "notfound", "not_found", "not_visible",
		"conflict", "timeout", "unprocessable",
		"403", "404", "409", "401", "400", "422", "500", "502", "503",
		"blocked", "gated", "fails", "raises", "errors", "skips", "ignores",
		"without", "empty", "none", "unknown", "stale",
		"degrad", "fallback", "falls_back", "fall_back",
	}
	reHTTPStatus      = regexp.MustCompile(`\b([45]\d{2})\b`)
	reHTTPStatusConst = regexp.MustCompile(`(?i)HTTP_([45]\d{2})`)
	reSideEffectExc   = regexp.MustCompile(`(?i)side_effect\s*=\s*[\w.]*(Error|Exception|Timeout|Fail)`)
	reCaplogLevel     = regexp.MustCompile(`(?i)\b(WARNING|ERROR|CRITICAL)\b`)
)

// OnlyHappyPathOpts configures only-happy-path (heuristic or coverage mode).
type OnlyHappyPathOpts struct {
	MinTests      int
	Mode          string // ""|"heuristic"|"coverage"
	CoveragePath  string
	NegativeNames []string
}

type onlyHappyPath struct {
	minTests      int
	mode          string
	coveragePath  string
	negativeNames []string
}

func (onlyHappyPath) ID() string {
	return "only-happy-path"
}

func (onlyHappyPath) NeedsAST() bool { return true }

func (o onlyHappyPath) min() int {
	if o.minTests > 0 {
		return o.minTests
	}
	return onlyHappyPathMinTestsDefault
}

func (o onlyHappyPath) coverageMode() bool {
	return o.mode == "coverage"
}

func (o onlyHappyPath) negNames() []string {
	if len(o.negativeNames) > 0 {
		return o.negativeNames
	}
	return defaultNegativeNames
}

func (o onlyHappyPath) Check(file scan.File) []scan.Finding {
	if o.coverageMode() {
		// Coverage analysis runs once via CheckProject.
		return nil
	}
	model, err := astModel(file)
	if err != nil {
		if useHeuristic(file, err) {
			return o.heuristic(file)
		}
		return nil
	}
	return o.fromAST(file, model)
}

// CheckProject implements scan.ProjectRule for coverage mode.
func (o onlyHappyPath) CheckProject(_ context.Context, info scan.ProjectInfo) []scan.Finding {
	if !o.coverageMode() {
		return nil
	}
	path := info.CoveragePath
	if path == "" {
		path = o.coveragePath
	}
	if path == "" {
		path = "coverage.json"
	}
	return findUncoveredErrorPaths(path, info.PathRoot)
}

func (o onlyHappyPath) fromAST(file scan.File, model parse.Model) []scan.Finding {
	src := string(file.Content)
	neg := o.negNames()
	minT := o.min()

	sctx := NewSUTContext(file, model)
	groups := map[string][]parse.TestFunc{}
	order := make([]string, 0)
	for _, t := range model.Tests {
		sut := primarySUT(t, sctx.Calls(t))
		if sut == "" {
			// No call into project code (only helpers / stdlib / data): nothing
			// to attribute an unhappy path to.
			continue
		}
		if _, ok := groups[sut]; !ok {
			order = append(order, sut)
		}
		groups[sut] = append(groups[sut], t)
	}

	var findings []scan.Finding
	for _, sut := range order {
		tests := groups[sut]
		if len(tests) <= minT {
			continue
		}
		if sutGroupHasNegative(tests, src, neg) {
			continue
		}
		// Pure mappers / enum helpers without branches: skip (no real unhappy path).
		if !sutLooksBranchy(file.Path, sut, model.Imports, src) {
			continue
		}
		line := tests[0].Lineno
		qual := qualName(tests[0])
		msg := fmt.Sprintf("SUT %s: more than %d tests without negative-path signals", sut, minT)
		findings = append(findings, scan.Finding{
			File:     file.Path,
			Line:     line,
			Rule:     "only-happy-path",
			Severity: "note",
			Message:  msg,
			QualName: qual,
		})
	}
	return findings
}

// primarySUT picks the test's subject: the last SUT call before the first
// assert (else the last SUT call). calls come from SUTContext.Calls, so only
// project code qualifies. Returns the resolved name, "" when there is none.
func primarySUT(t parse.TestFunc, calls []SUTCall) string {
	assertLine := 0
	for _, a := range t.Asserts {
		if assertLine == 0 || (a.Lineno > 0 && a.Lineno < assertLine) {
			assertLine = a.Lineno
		}
	}
	var lastBefore, lastAny string
	for _, c := range calls {
		lastAny = c.Resolved
		if assertLine == 0 || c.Lineno < assertLine {
			lastBefore = c.Resolved
		}
	}
	if lastBefore != "" {
		return lastBefore
	}
	return lastAny
}

// primarySUTCall is the legacy, project-agnostic subject guess (last
// non-framework call before the first assert). Kept for rules that do not
// have a SUTContext (near-duplicate-test); only-happy-path uses primarySUT.
func primarySUTCall(t parse.TestFunc) string {
	assertLine := 0
	for _, a := range t.Asserts {
		if assertLine == 0 || (a.Lineno > 0 && a.Lineno < assertLine) {
			assertLine = a.Lineno
		}
	}
	var lastBefore string
	var lastAny string
	for _, c := range t.Calls {
		if isFrameworkOrMockCall(c.Name) {
			continue
		}
		lastAny = c.Name
		if assertLine == 0 || c.Lineno < assertLine {
			lastBefore = c.Name
		}
	}
	if lastBefore != "" {
		return lastBefore
	}
	return lastAny
}

func isFrameworkOrMockCall(name string) bool {
	leaf := leafName(name)
	n := strings.ToLower(name)
	if strings.Contains(n, "patch") ||
		leaf == "MagicMock" || leaf == "Mock" || leaf == "AsyncMock" || leaf == "PropertyMock" ||
		leaf == "fixture" || leaf == "mark" || leaf == "param" {
		return true
	}
	if strings.HasPrefix(n, "pytest.") || strings.HasPrefix(n, "unittest.") ||
		strings.HasPrefix(n, "mock.") || strings.HasPrefix(n, "unittest.mock.") {
		return true
	}
	if strings.HasPrefix(leaf, "assert") {
		return true
	}
	if isMockOrPatchOrAssertCall(name) {
		return true
	}
	switch leaf {
	case "print", "len", "str", "int", "float", "bool", "list", "dict", "set", "tuple",
		"range", "enumerate", "zip", "sorted", "reversed", "isinstance", "hasattr",
		"getattr", "setattr", "type", "super", "open", "vars", "dir", "id", "hash",
		"min", "max", "sum", "any", "all", "map", "filter", "next", "iter",
		"repr", "format", "bytes", "bytearray", "frozenset", "complex", "object",
		"property", "staticmethod", "classmethod", "abs", "round", "pow", "divmod",
		"callable", "issubclass", "memoryview", "slice", "chr", "ord", "hex", "oct", "bin",
		"input", "help", "exit", "quit":
		return true
	}
	return false
}

func sutGroupHasNegative(tests []parse.TestFunc, src string, negName []string) bool {
	for _, t := range tests {
		if testHasNegativePath(t, negName) {
			return true
		}
	}
	// Shared file-level HTTP / side_effect / caplog signals still count.
	return statusLooksNegative(src) || sideEffectLooksNegative(src) || caplogLooksNegative(src)
}

func testHasNegativePath(t parse.TestFunc, negName []string) bool {
	if t.HasRaises || len(t.Raises) > 0 {
		return true
	}
	for _, c := range t.Calls {
		cl := strings.ToLower(c.Name)
		if strings.Contains(cl, "raises") || strings.Contains(cl, "warns") ||
			strings.Contains(cl, "assertraises") {
			return true
		}
	}
	nameLower := strings.ToLower(t.Name)
	for _, n := range negName {
		if nameHasNegToken(nameLower, n) {
			return true
		}
	}
	for _, d := range t.Decorators {
		dl := strings.ToLower(d)
		if strings.Contains(dl, "parametrize") {
			for _, n := range negName {
				if nameHasNegToken(dl, n) {
					return true
				}
			}
		}
	}
	for _, a := range t.Assignments {
		if sideEffectAssignmentNegative(a) {
			return true
		}
	}
	for _, a := range t.Asserts {
		if assertLooksNegative(a) {
			return true
		}
	}
	for _, f := range t.Fixtures {
		if strings.EqualFold(f, "caplog") {
			for _, a := range t.Asserts {
				if reCaplogLevel.MatchString(a.Text) {
					return true
				}
			}
		}
	}
	return false
}

func sideEffectAssignmentNegative(a parse.Assignment) bool {
	leaf := leafName(a.Target)
	if !strings.EqualFold(leaf, "side_effect") {
		return false
	}
	v := strings.TrimSpace(a.Value)
	if v == "" || v == "None" {
		return false
	}
	if reSideEffectExc.MatchString("side_effect=" + v) {
		return true
	}
	if strings.Contains(v, "Error") || strings.Contains(v, "Exception") ||
		strings.Contains(v, "Timeout") || strings.Contains(v, "Fail") {
		return true
	}
	return false
}

func assertLooksNegative(a parse.Assert) bool {
	al := strings.ToLower(a.Text)
	left := strings.ToLower(a.Left)
	right := strings.ToLower(a.Right)
	trim := strings.TrimSpace(al)

	// is not None / != None are weak positives, not negative-path signals.
	notNone := strings.Contains(al, "is not none") || strings.Contains(al, "!= none")
	if !notNone {
		if strings.Contains(al, "is none") || right == "none" || left == "none" {
			return true
		}
	}
	if strings.Contains(al, "is false") || right == "false" || left == "false" ||
		strings.Contains(al, "is_valid() is false") ||
		(strings.Contains(al, "is_valid") && right == "false") {
		return true
	}
	if strings.Contains(al, "errors") || strings.Contains(al, "detail") ||
		strings.Contains(left, "errors") || strings.Contains(left, "detail") {
		return true
	}
	// Leading `not x` only — do not treat `is not` / `is not None` as negative.
	if strings.HasPrefix(trim, "not ") {
		return true
	}
	if strings.Contains(al, " not in ") || strings.HasPrefix(trim, "not in ") {
		return true
	}
	// != only for status/bool/is_valid contexts — arbitrary inequality is not a negative path.
	if strings.Contains(al, "!=") && !notNone {
		if strings.Contains(al, "status") || strings.Contains(al, "code") ||
			strings.Contains(al, "is_valid") || strings.Contains(al, "http_") ||
			right == "false" || left == "false" || right == "true" || left == "true" ||
			looksLikeHTTPStatusNeg(al) {
			return true
		}
	}
	// Empty collection / empty string equality
	if right == "[]" || right == "{}" || right == "()" || right == `""` || right == "''" ||
		left == "[]" || left == "{}" || left == "()" || left == `""` || left == "''" {
		return true
	}
	if looksLikeHTTPStatusNeg(al) || looksLikeHTTPStatusNeg(left+" "+right) {
		return true
	}
	if reHTTPStatusConst.MatchString(a.Text) {
		return true
	}
	if reCaplogLevel.MatchString(a.Text) &&
		(strings.Contains(al, "caplog") || strings.Contains(al, "record") ||
			strings.Contains(al, "level") || strings.Contains(al, "message")) {
		return true
	}
	return false
}

func looksLikeHTTPStatusNeg(s string) bool {
	if reHTTPStatusConst.MatchString(s) {
		return true
	}
	if !strings.Contains(s, "status") && !strings.Contains(s, "code") &&
		!strings.Contains(strings.ToLower(s), "http_") {
		return false
	}
	return reHTTPStatus.MatchString(s)
}

func statusLooksNegative(src string) bool {
	srcLower := strings.ToLower(src)
	if reHTTPStatusConst.MatchString(src) {
		return true
	}
	if !strings.Contains(srcLower, "status") && !strings.Contains(srcLower, "status_code") &&
		!strings.Contains(srcLower, "http_") {
		return false
	}
	return reHTTPStatus.MatchString(src)
}

func sideEffectLooksNegative(src string) bool {
	return reSideEffectExc.MatchString(src)
}

func caplogLooksNegative(src string) bool {
	if !strings.Contains(strings.ToLower(src), "caplog") {
		return false
	}
	return reCaplogLevel.MatchString(src)
}

// sutLooksBranchy reports whether the SUT likely has unhappy paths worth testing.
// Unknown / unresolvable SUT is treated as branchy (keep the finding).
func sutLooksBranchy(testPath, sut string, imports []parse.Import, testSrc string) bool {
	leaf := leafName(sut)
	if leaf == "" {
		return true
	}
	// Same-file helper.
	if fnSrc := extractPythonFunc(testSrc, leaf); fnSrc != "" {
		return pythonFuncLooksBranchy(fnSrc)
	}
	modPath := resolveImportedModule(testPath, sut, imports)
	if modPath == "" {
		return true
	}
	data, err := os.ReadFile(modPath)
	if err != nil {
		return true
	}
	fnSrc := extractPythonFunc(string(data), leaf)
	if fnSrc == "" {
		// Module-level / class method not found — assume branchy.
		return true
	}
	if pureScalarFunc(fnSrc) {
		return false
	}
	return pythonFuncLooksBranchy(fnSrc)
}

var (
	reScalarReturn = regexp.MustCompile(`\)\s*->\s*(str|int|float|bool|bytes)\s*:`)
	reRaiseStmt    = regexp.MustCompile(`(?m)^\s*raise\b`)
)

// pureScalarFunc: annotated to return a bare scalar and never raises, so it has
// no error contract to test (input->output mapper such as a quoting helper).
func pureScalarFunc(fnSrc string) bool {
	return reScalarReturn.MatchString(fnSrc) && !reRaiseStmt.MatchString(fnSrc)
}

func resolveImportedModule(testPath, sut string, imports []parse.Import) string {
	leaf := leafName(sut)
	base := ""
	if i := strings.Index(sut, "."); i >= 0 {
		base = sut[:i]
	}
	dir := filepath.Dir(testPath)
	for _, imp := range imports {
		switch imp.Kind {
		case "from":
			for _, n := range imp.Names {
				if n == leaf || n == base {
					return moduleFileCandidates(dir, imp.Module)
				}
			}
			if base != "" && (imp.Module == base || strings.HasSuffix(imp.Module, "."+base)) {
				return moduleFileCandidates(dir, imp.Module)
			}
		case "import":
			for _, n := range imp.Names {
				if n == base || strings.HasPrefix(sut, n+".") {
					return moduleFileCandidates(dir, n)
				}
			}
		}
	}
	return ""
}

func moduleFileCandidates(fromDir, module string) string {
	if module == "" {
		return ""
	}
	parts := strings.Split(module, ".")
	rel := filepath.Join(parts...) + ".py"
	pkg := filepath.Join(append(parts, "__init__.py")...)
	var candidates []string
	cur := fromDir
	for i := 0; i < 8; i++ {
		candidates = append(candidates,
			filepath.Join(cur, rel),
			filepath.Join(cur, pkg),
		)
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}

func extractPythonFunc(src, name string) string {
	lines := strings.Split(src, "\n")
	sig := "def " + name + "("
	asyncSig := "async def " + name + "("
	start := -1
	indent := ""
	for i, line := range lines {
		trim := strings.TrimLeftFunc(line, unicode.IsSpace)
		if strings.HasPrefix(trim, sig) || strings.HasPrefix(trim, asyncSig) {
			start = i
			indent = line[:len(line)-len(trim)]
			break
		}
	}
	if start < 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(lines[start])
	b.WriteByte('\n')
	for i := start + 1; i < len(lines); i++ {
		line := lines[i]
		trim := strings.TrimLeftFunc(line, unicode.IsSpace)
		if trim == "" {
			b.WriteByte('\n')
			continue
		}
		lead := line[:len(line)-len(trim)]
		if !strings.HasPrefix(lead, indent) || lead == indent {
			// Dedent to same or less → end of function (class method body uses deeper indent).
			if len(lead) <= len(indent) {
				break
			}
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func pythonFuncLooksBranchy(fnSrc string) bool {
	if len(errorPathLineSet(fnSrc)) > 0 {
		return true
	}
	for _, line := range strings.Split(fnSrc, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		if idx := strings.Index(trim, " #"); idx >= 0 {
			trim = strings.TrimSpace(trim[:idx])
		}
		if strings.HasPrefix(trim, "if ") || strings.HasPrefix(trim, "elif ") ||
			trim == "else:" || strings.HasPrefix(trim, "match ") ||
			strings.HasPrefix(trim, "case ") || strings.HasPrefix(trim, "for ") ||
			strings.HasPrefix(trim, "while ") || strings.HasPrefix(trim, "try:") ||
			strings.HasPrefix(trim, "with ") {
			return true
		}
	}
	return false
}

func (o onlyHappyPath) heuristic(file scan.File) []scan.Finding {
	src := string(file.Content)
	n := countTestFuncs(src)
	minT := o.min()
	if n <= minT {
		return nil
	}
	hasNeg := strings.Contains(src, "pytest.raises") ||
		strings.Contains(src, "pytest.warns") ||
		strings.Contains(src, "assertRaises")
	if hasNeg {
		return nil
	}
	if heuristicHasNegativeName(src, o.negNames()) {
		return nil
	}
	if statusLooksNegative(src) || sideEffectLooksNegative(src) || caplogLooksNegative(src) {
		return nil
	}
	return []scan.Finding{{
		File:     file.Path,
		Line:     1,
		Rule:     "only-happy-path",
		Severity: "note",
		Message:  fmt.Sprintf("more than %d tests without negative-path signals", minT),
	}}
}

func countTestFuncs(src string) int {
	n := 0
	for _, line := range strings.Split(src, "\n") {
		if _, ok := testDefName(line); ok {
			n++
		}
	}
	return n
}

func nameHasNegToken(name, tok string) bool {
	tok = strings.ToLower(strings.TrimSpace(tok))
	if tok == "" {
		return false
	}
	name = strings.ToLower(name)
	// Ambiguous short tokens must be underscore-bounded.
	switch tok {
	case "without", "empty", "none", "ok", "not":
		return nameHasBoundedToken(name, tok)
	}
	// Numeric / already-specific tokens: substring is fine.
	if strings.Contains(name, tok) {
		return true
	}
	return false
}

// nameHasBoundedToken reports whether tok appears as a full underscore-separated segment.
func nameHasBoundedToken(name, tok string) bool {
	for _, p := range strings.Split(name, "_") {
		if p == tok {
			return true
		}
	}
	return false
}

func heuristicHasNegativeName(src string, negName []string) bool {
	for _, line := range strings.Split(src, "\n") {
		name, ok := testDefName(line)
		hay := strings.ToLower(line)
		if ok {
			hay = strings.ToLower(name)
		} else if !strings.Contains(hay, "parametrize") {
			continue
		}
		for _, n := range negName {
			if nameHasNegToken(hay, n) {
				return true
			}
		}
	}
	return false
}

func NewOnlyHappyPath() scan.Rule {
	return onlyHappyPath{}
}

// NewOnlyHappyPathMin returns only-happy-path with a custom min-tests threshold.
func NewOnlyHappyPathMin(minTests int) scan.Rule {
	return onlyHappyPath{minTests: minTests}
}

// NewOnlyHappyPathOpts returns only-happy-path with full options.
func NewOnlyHappyPathOpts(opts OnlyHappyPathOpts) scan.Rule {
	return onlyHappyPath{
		minTests:      opts.MinTests,
		mode:          opts.Mode,
		coveragePath:  opts.CoveragePath,
		negativeNames: append([]string{}, opts.NegativeNames...),
	}
}

var (
	_ scan.Rule        = onlyHappyPath{}
	_ scan.ASTRule     = onlyHappyPath{}
	_ scan.ProjectRule = onlyHappyPath{}
)
