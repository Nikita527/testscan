package rules

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

type weakAssert struct{}

func (weakAssert) ID() string { return "weak-assert" }

func (weakAssert) NeedsAST() bool { return true }

var (
	reAcceptName  = regexp.MustCompile(`(?i)(^|_)(accepts?|passes?|valid)(_|$)`)
	reRejectName  = regexp.MustCompile(`(?i)(^|_)(rejects?|fails?|invalid)(_|$)`)
	reStatusOK    = regexp.MustCompile(`(?i)\b(status_code|status)\b.*\b(200|HTTP_200(_OK)?)\b|\b(200|HTTP_200(_OK)?)\b.*\b(status_code|status)\b`)
	reStatusToken = regexp.MustCompile(`(?i)\b(status_code|status|HTTP_200(_OK)?)\b`)
)

func (weakAssert) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return nil
	}
	src := string(file.Content)
	var findings []scan.Finding
	for _, t := range model.Tests {
		if t.HasRaises || len(t.Asserts) == 0 {
			continue
		}
		reason := ""
		switch {
		case isShallowRejectAsserts(t):
			reason = "shallow reject asserts without error code/message"
		case isWeakStatusOnly(t, src):
			reason = "only status_code == 200 without response body check"
		case allAssertsWeak(t):
			reason = "only weak asserts (bare truthy / is not None / len vs 0)"
		default:
			continue
		}
		firstLine := t.Asserts[0].Lineno
		for _, a := range t.Asserts {
			if a.Lineno < firstLine {
				firstLine = a.Lineno
			}
		}
		findings = append(findings, scan.Finding{
			File:     file.Path,
			Line:     firstLine,
			Rule:     "weak-assert",
			Severity: "note",
			Message:  reason,
			QualName: qualName(t),
		})
	}
	return findings
}

func allAssertsWeak(t parse.TestFunc) bool {
	for _, a := range t.Asserts {
		if !isWeakAssert(a, t) {
			return false
		}
	}
	return true
}

func isWeakAssert(a parse.Assert, t parse.TestFunc) bool {
	// accepts_*/passes_*/valid*: is_valid / .errors presence is the contract.
	if nameLooksAccept(t.Name) && isValidContractAssert(a) {
		return false
	}
	switch a.Kind {
	case "mock_method", "tuple", "isinstance", "unittest_bool", "other":
		return false
	case "truthy":
		return isBareTruthyWeak(a)
	case "compare":
		return isWeakCompare(a)
	default:
		return false
	}
}

// isBareTruthyWeak is true only for bare Name/Attribute truthy checks
// (assert x / assert obj.flag). Calls, "not …", bool-ops, and subscripts
// are treated as explicit boolean contracts, not weak asserts.
func isBareTruthyWeak(a parse.Assert) bool {
	text := strings.TrimSpace(a.Text)
	if text == "" {
		return false
	}
	// Drop optional assert message: "x, 'msg'" → "x"
	if i := strings.Index(text, ","); i >= 0 {
		text = strings.TrimSpace(text[:i])
	}
	if text == "" {
		return false
	}
	if strings.Contains(text, "(") {
		return false
	}
	if strings.HasPrefix(text, "not ") || text == "not" {
		return false
	}
	if strings.Contains(text, " and ") || strings.Contains(text, " or ") {
		return false
	}
	if strings.Contains(text, "[") {
		return false
	}
	return true
}

func isWeakCompare(a parse.Assert) bool {
	right := strings.TrimSpace(a.Right)
	left := strings.TrimSpace(a.Left)
	text := strings.TrimSpace(a.Text)
	// is not None is weak; bare is None / == None is a real negative check.
	if strings.Contains(text, " is not None") || strings.Contains(text, " != None") {
		return true
	}
	if (strings.Contains(text, " is not ") || strings.Contains(text, " != ")) &&
		(right == "None" || left == "None") {
		return true
	}
	// len(...) compared to exactly 0 (not len == 200).
	if isLenExpr(left) && right == "0" {
		return true
	}
	if isLenExpr(right) && left == "0" {
		return true
	}
	return false
}

func isLenExpr(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "len(") && strings.HasSuffix(s, ")")
}

func nameLooksAccept(name string) bool {
	return reAcceptName.MatchString(name)
}

func nameLooksReject(name string) bool {
	return reRejectName.MatchString(name)
}

func isValidContractAssert(a parse.Assert) bool {
	al := strings.ToLower(strings.TrimSpace(a.Text))
	if i := strings.Index(al, ","); i >= 0 {
		al = strings.TrimSpace(al[:i])
	}
	if strings.Contains(al, "is_valid") {
		return true
	}
	// assert result.errors / assert not result.errors alongside is_valid
	if strings.HasSuffix(al, ".errors") || al == "errors" ||
		strings.HasSuffix(al, "not result.errors") || strings.HasPrefix(al, "not ") && strings.Contains(al, "errors") {
		return true
	}
	return false
}

// isShallowRejectAsserts: rejects_*/fails_*/invalid* with only "has errors" /
// "not is_valid" / "len(errors)>0" and no error code/message check.
func isShallowRejectAsserts(t parse.TestFunc) bool {
	if !nameLooksReject(t.Name) {
		return false
	}
	if len(t.Asserts) == 0 {
		return false
	}
	for _, a := range t.Asserts {
		if assertChecksErrorDetail(a) {
			return false
		}
	}
	for _, a := range t.Asserts {
		if !isShallowRejectAssert(a) {
			return false
		}
	}
	return true
}

func isShallowRejectAssert(a parse.Assert) bool {
	al := strings.ToLower(strings.TrimSpace(a.Text))
	core := al
	if i := strings.Index(core, ","); i >= 0 {
		core = strings.TrimSpace(core[:i])
	}
	left := strings.ToLower(strings.TrimSpace(a.Left))
	right := strings.ToLower(strings.TrimSpace(a.Right))

	if core == "errors" || strings.HasSuffix(core, ".errors") {
		return true
	}
	if strings.HasPrefix(core, "not ") && strings.Contains(core, "is_valid") {
		return true
	}
	if strings.Contains(core, "is_valid") && (right == "false" || strings.Contains(al, "is false")) {
		return true
	}
	if isLenExpr(left) && (right == "0") &&
		(strings.Contains(al, ">") || strings.Contains(al, "!=")) &&
		strings.Contains(left, "error") {
		return true
	}
	if isLenExpr(left) && right != "0" && right != "" {
		// len(errors) > 0 already covered; exact count is stronger — not shallow
		if n, err := strconv.Atoi(right); err == nil && n > 0 {
			return false
		}
	}
	if isLenExpr(left) && strings.Contains(left, "error") &&
		(right == "0" || strings.Contains(al, "> 0") || strings.Contains(al, "!= 0")) {
		return true
	}
	return false
}

func assertChecksErrorDetail(a parse.Assert) bool {
	al := strings.ToLower(a.Text)
	left := strings.ToLower(a.Left)
	markers := []string{
		".code", "[\"code\"]", "['code']", ".message", "[\"message\"]", "['message']",
		".detail", "[\"detail\"]", "['detail']", "error_code", "errcode",
		"strerror", ".reason",
	}
	for _, m := range markers {
		if strings.Contains(al, m) || strings.Contains(left, m) {
			return true
		}
	}
	// Equality against a specific error string/code literal.
	if a.Kind == "compare" && (strings.Contains(left, "error") || strings.Contains(left, "detail")) {
		right := strings.TrimSpace(a.Right)
		if strings.HasPrefix(right, "\"") || strings.HasPrefix(right, "'") ||
			(right != "" && right != "None" && right != "True" && right != "False" && right != "0") {
			return true
		}
	}
	return false
}

func isWeakStatusOnly(t parse.TestFunc, src string) bool {
	if len(t.Asserts) == 0 {
		return false
	}
	for _, a := range t.Asserts {
		if !isStatusOKAssert(a) {
			return false
		}
	}
	if hasResponseBodyAssert(t) {
		return false
	}
	return looksLikeGET(t, src)
}

func isStatusOKAssert(a parse.Assert) bool {
	text := strings.TrimSpace(a.Text)
	if !reStatusToken.MatchString(text) {
		return false
	}
	if reStatusOK.MatchString(text) {
		return true
	}
	right := strings.TrimSpace(a.Right)
	left := strings.TrimSpace(a.Left)
	if (right == "200" || strings.Contains(strings.ToUpper(right), "HTTP_200")) &&
		reStatusToken.MatchString(left) {
		return true
	}
	if (left == "200" || strings.Contains(strings.ToUpper(left), "HTTP_200")) &&
		reStatusToken.MatchString(right) {
		return true
	}
	return false
}

func hasResponseBodyAssert(t parse.TestFunc) bool {
	for _, a := range t.Asserts {
		al := strings.ToLower(a.Text)
		if strings.Contains(al, ".json(") || strings.Contains(al, ".text") ||
			strings.Contains(al, ".content") || strings.Contains(al, ".body") ||
			strings.Contains(al, "json()[") {
			return true
		}
	}
	return false
}

func looksLikeGET(t parse.TestFunc, src string) bool {
	name := strings.ToLower(t.Name)
	name = strings.TrimPrefix(name, "test_")
	if nameHasBoundedToken(name, "get") || nameHasBoundedToken(name, "list") ||
		nameHasBoundedToken(name, "fetch") || nameHasBoundedToken(name, "read") ||
		strings.HasPrefix(name, "get_") || strings.HasSuffix(name, "_get") ||
		strings.HasPrefix(name, "list_") || strings.HasSuffix(name, "_list") {
		return true
	}
	for _, c := range t.Calls {
		cl := strings.ToLower(c.Name)
		leaf := strings.ToLower(leafName(c.Name))
		if leaf == "get" || strings.HasSuffix(cl, ".get") ||
			strings.Contains(cl, "client.get") || strings.Contains(cl, "api.get") {
			return true
		}
	}
	// Fallback: GET in the test body snippet.
	lines := strings.Split(src, "\n")
	start := t.Lineno - 1
	if start < 0 {
		start = 0
	}
	end := t.EndLineno
	if end <= 0 || end > len(lines) {
		end = len(lines)
	}
	snippet := strings.ToLower(strings.Join(lines[start:end], "\n"))
	return strings.Contains(snippet, ".get(") || strings.Contains(snippet, "\"get\"") ||
		strings.Contains(snippet, "'get'")
}

func NewWeakAssert() scan.Rule {
	return weakAssert{}
}
