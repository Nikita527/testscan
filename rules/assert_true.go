package rules

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

type assertTrue struct{}

func (assertTrue) ID() string { return "assert-true" }

func (assertTrue) NeedsAST() bool { return true }

func (assertTrue) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		if useHeuristic(file, err) {
			return assertTrueHeuristic(file)
		}
		return nil
	}
	return assertTrueFromAST(file, model)
}

func assertTrueFromAST(file scan.File, model parse.Model) []scan.Finding {
	lines := strings.Split(string(file.Content), "\n")
	var findings []scan.Finding
	for _, t := range model.Tests {
		q := qualName(t)
		for _, a := range t.Asserts {
			switch a.Kind {
			case "truthy":
				msg, ok := alwaysTrueMessage(a.Text)
				if !ok {
					continue
				}
				// Owned by assert-inside-broad-raises — avoid duplicate on the same line.
				if isFailingConstantAssert(a) && assertInsideTrapRaises(t, a.Lineno) {
					continue
				}
				// Exhaustiveness guard: `else: assert False` is intentionally unreachable.
				if isFailingConstantAssert(a) && assertInElseBranch(lines, a.Lineno) {
					continue
				}
				findings = append(findings, scan.Finding{
					File:     file.Path,
					Line:     a.Lineno,
					Rule:     "assert-true",
					Severity: "warning",
					Message:  msg,
					QualName: q,
				})
			case "unittest_bool":
				findings = append(findings, scan.Finding{
					File:     file.Path,
					Line:     a.Lineno,
					Rule:     "assert-true",
					Severity: "warning",
					Message:  "assertTrue/assertFalse with a comparison; prefer assertEqual",
					QualName: q,
				})
			}
		}
	}
	return findings
}

// assertInsideTrapRaises is true when lineno sits in a pytest.raises body that
// catches Exception/BaseException/AssertionError (tuple form included).
func assertInsideTrapRaises(t parse.TestFunc, lineno int) bool {
	for _, r := range t.Raises {
		if !isTrapRaisesException(r.Exc) {
			continue
		}
		end := r.EndLineno
		if end <= 0 {
			end = r.Lineno
		}
		if lineno > r.Lineno && lineno <= end {
			return true
		}
	}
	return false
}

// assertInElseBranch reports whether lineno sits under an if/elif `else:`
// (or a match `case _:`) exhaustiveness guard — not for/while/try else-clauses,
// which are reachable. Handles same-line forms (`else: assert False`,
// `case _: assert False`) as well as indented bodies.
func assertInElseBranch(lines []string, lineno int) bool {
	if lineno < 1 || lineno > len(lines) {
		return false
	}
	cur := lines[lineno-1]
	trim := strings.TrimSpace(cur)
	indent := leadingSpaces(cur)

	// Same-line: assert shares the line with else: / case _[ if …]:
	if body, ok := sameLineCompoundBody(trim, "else:"); ok && body != "" {
		return elseBelongsToIf(lines, lineno-1, indent)
	}
	if body, ok := parseMatchWildcardCase(trim); ok && body != "" {
		return true
	}

	for i := lineno - 2; i >= 0; i-- {
		prev := strings.TrimSpace(lines[i])
		if prev == "" || strings.HasPrefix(prev, "#") {
			continue
		}
		ind := leadingSpaces(lines[i])
		if ind >= indent {
			continue
		}
		if prev == "else:" || strings.HasPrefix(prev, "else:") {
			return elseBelongsToIf(lines, i, ind)
		}
		if _, ok := parseMatchWildcardCase(prev); ok {
			return true
		}
		return false
	}
	return false
}

// sameLineCompoundBody returns the suite after a leading keyword like "else:"
// when it appears on the same line (non-empty body).
func sameLineCompoundBody(trim, keyword string) (string, bool) {
	if !strings.HasPrefix(trim, keyword) {
		return "", false
	}
	return strings.TrimSpace(trim[len(keyword):]), true
}

// parseMatchWildcardCase recognizes exhaustiveness wildcards:
//   case _:
//   case _: <body>
//   case _ if <guard>:
//   case _ if <guard>: <body>
// Non-wildcard patterns (case X if cond:) return ok=false — those branches are reachable.
func parseMatchWildcardCase(trim string) (body string, ok bool) {
	if !strings.HasPrefix(trim, "case ") {
		return "", false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(trim, "case "))
	switch {
	case rest == "_", rest == "_:":
		return "", true
	case strings.HasPrefix(rest, "_:"):
		return strings.TrimSpace(rest[len("_:"):]), true
	case strings.HasPrefix(rest, "_ if ") || strings.HasPrefix(rest, "_ if("):
		idx := strings.LastIndex(rest, ":")
		if idx < 0 {
			return "", false
		}
		return strings.TrimSpace(rest[idx+1:]), true
	default:
		return "", false
	}
}

// elseBelongsToIf is true when the else at elseIdx (indent elseInd) closes an
// if/elif chain, not for/while/try.
func elseBelongsToIf(lines []string, elseIdx, elseInd int) bool {
	for j := elseIdx - 1; j >= 0; j-- {
		trim := strings.TrimSpace(lines[j])
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		ind := leadingSpaces(lines[j])
		if ind > elseInd {
			continue
		}
		if ind < elseInd {
			return false
		}
		// Same indent as else — compound statement head.
		if isIfOrElifHeader(trim) {
			return true
		}
		return false
	}
	return false
}

func isIfOrElifHeader(trim string) bool {
	return strings.HasPrefix(trim, "if ") || strings.HasPrefix(trim, "if(") ||
		strings.HasPrefix(trim, "elif ") || strings.HasPrefix(trim, "elif(")
}

// isMatchDefaultCase matches wildcard `case _:` / `case _ if …:` defaults.
func isMatchDefaultCase(trim string) bool {
	_, ok := parseMatchWildcardCase(trim)
	return ok
}

// reOrTruthyConstant matches "... or True" / "... or 1" / `... or "x"` style
// agent hedged asserts that can never fail.
var reOrTruthyConstant = regexp.MustCompile(`(?i)\bor\s+(True|1|1\.0|"[^"]+"|'[^']+')\s*(,|$)`)

// alwaysTrueMessage classifies constant / tautology asserts.
func alwaysTrueMessage(text string) (string, bool) {
	t := strings.TrimSpace(text)
	if t == "" {
		return "", false
	}
	if reOrTruthyConstant.MatchString(t) {
		return "assert is always true (`… or <truthy constant>`)", true
	}
	if !isConstantTruthyAssert(t) {
		return "", false
	}
	core := t
	if i := strings.Index(core, ","); i >= 0 {
		core = strings.TrimSpace(core[:i])
	}
	switch core {
	case "True":
		return "assert of a constant (True) found in test", true
	case "False":
		return "assert of a constant (False) found in test", true
	case "None":
		return "assert of a constant (None) found in test", true
	}
	if isQuotedStringLiteral(core) {
		return "assert of a non-empty string literal is always true; did you mean `in <result>`?", true
	}
	return "assert of a constant (True/1/\"…\") found in test", true
}

// isConstantTruthyAssert reports assert True / False / None / 1 / "str" style constants.
func isConstantTruthyAssert(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return false
	}
	// Drop optional assert message: "True, 'msg'"
	if i := strings.Index(t, ","); i >= 0 {
		t = strings.TrimSpace(t[:i])
	}
	switch t {
	case "True", "False", "None":
		return true
	}
	if _, err := strconv.ParseInt(t, 0, 64); err == nil {
		return true
	}
	if _, err := strconv.ParseFloat(t, 64); err == nil {
		return true
	}
	if isQuotedStringLiteral(t) {
		return true
	}
	return false
}

func isQuotedStringLiteral(t string) bool {
	if len(t) < 2 {
		return false
	}
	quote := rune(t[0])
	if quote != '"' && quote != '\'' {
		return false
	}
	if rune(t[len(t)-1]) != quote {
		return false
	}
	// Reject f/b/r prefixes handled separately; bare quotes only.
	for _, r := range t[1 : len(t)-1] {
		if r == quote {
			return false // unescaped inner quote — not a simple literal for our purposes
		}
		if !unicode.IsPrint(r) && r != '\t' {
			return false
		}
	}
	return true
}

func assertTrueHeuristic(file scan.File) []scan.Finding {
	findings := []scan.Finding{}
	src := string(file.Content)
	if strings.Contains(src, "assert True") {
		findings = append(findings, scan.Finding{
			File:     file.Path,
			Line:     lineOf(src, "assert True"),
			Rule:     "assert-true",
			Severity: "warning",
			Message:  "assert True found in test file",
		})
	}
	return findings
}

func NewAssertTrue() scan.Rule {
	return assertTrue{}
}
