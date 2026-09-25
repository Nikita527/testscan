package rules

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

const onlyHappyPathMinTestsDefault = 3

var (
	onlyHappyPathSignals = []string{
		"pytest.raises / assertRaises / pytest.warns",
		"status 4xx/5xx",
		"is None / is False / not",
		"errors/detail keys",
		"is_valid() is False",
		"negative test name (invalid/forbidden/missing/...)",
		"negative parametrize id",
	}
	defaultNegativeNames = []string{
		"invalid", "forbidden", "missing", "denied", "unauthorized",
		"not_", "_not", "reject", "notfound", "not_found", "not_visible",
		"conflict", "timeout", "unprocessable",
	}
	reHTTPStatus = regexp.MustCompile(`\b([45]\d{2})\b`)
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
	n := len(model.Tests)
	minT := o.min()
	if n <= minT {
		return nil
	}
	src := string(file.Content)
	if hasNegativePath(model, src, o.negNames()) {
		return nil
	}
	line := 1
	qual := ""
	if n > 0 {
		line = model.Tests[0].Lineno
		qual = model.Tests[0].QualName
		if qual == "" {
			qual = model.Tests[0].Name
		}
	}
	return []scan.Finding{{
		File:     file.Path,
		Line:     line,
		Rule:     "only-happy-path",
		Severity: "note",
		Message:  fmt.Sprintf("more than %d tests and no negative-path signals (%s)", minT, strings.Join(onlyHappyPathSignals, "; ")),
		QualName: qual,
	}}
}

func hasNegativePath(model parse.Model, src string, negName []string) bool {
	for _, t := range model.Tests {
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
			if n != "" && strings.Contains(nameLower, strings.ToLower(n)) {
				return true
			}
		}
		for _, d := range t.Decorators {
			dl := strings.ToLower(d)
			if strings.Contains(dl, "parametrize") {
				for _, n := range negName {
					if n != "" && strings.Contains(dl, strings.ToLower(n)) {
						return true
					}
				}
			}
		}
		for _, a := range t.Asserts {
			if assertLooksNegative(a) {
				return true
			}
		}
	}
	if statusLooksNegative(src) {
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
	if strings.Contains(al, "is false") || right == "false" ||
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
	if looksLikeHTTPStatusNeg(al) || looksLikeHTTPStatusNeg(left+" "+right) {
		return true
	}
	return false
}

func looksLikeHTTPStatusNeg(s string) bool {
	if !strings.Contains(s, "status") && !strings.Contains(s, "code") {
		return false
	}
	return reHTTPStatus.MatchString(s)
}

func statusLooksNegative(src string) bool {
	srcLower := strings.ToLower(src)
	if !strings.Contains(srcLower, "status") && !strings.Contains(srcLower, "status_code") {
		return false
	}
	return reHTTPStatus.MatchString(src)
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
	if statusLooksNegative(src) {
		return nil
	}
	return []scan.Finding{{
		File:     file.Path,
		Line:     1,
		Rule:     "only-happy-path",
		Severity: "note",
		Message:  fmt.Sprintf("more than %d tests and no negative-path signals (%s)", minT, strings.Join(onlyHappyPathSignals, "; ")),
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
			if n != "" && strings.Contains(hay, strings.ToLower(n)) {
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
