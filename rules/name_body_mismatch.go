package rules

import (
	"regexp"
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

type nameBodyMismatch struct{}

func (nameBodyMismatch) ID() string { return "name-body-mismatch" }

func (nameBodyMismatch) NeedsAST() bool { return true }

var (
	reNameNegativeCue = regexp.MustCompile(`(?i)(^|_)(rejects?|fails?|invalid|raises?|blocked|denied|forbidden|missing|unauthorized|errors?|conflict|not_found|notfound)(_|$)|(^|_)(returns?_)?[45]\d{2}(_|$)|_[45]\d{2}(_|$)|(^|_)empty(_|$)`)
	reBodyHTTPNeg     = regexp.MustCompile(`(?i)\b([45]\d{2}|HTTP_[45]\d{2})\b`)
)

func (nameBodyMismatch) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return nil
	}
	src := string(file.Content)
	var findings []scan.Finding
	for _, t := range model.Tests {
		if !reNameNegativeCue.MatchString(t.Name) {
			continue
		}
		if testBodyHasNegativeSignal(t, src) {
			continue
		}
		findings = append(findings, scan.Finding{
			File:     file.Path,
			Line:     t.Lineno,
			Rule:     "name-body-mismatch",
			Severity: "note",
			Message:  "test name implies a negative/error case but body has no matching signal",
			QualName: qualName(t),
		})
	}
	return findings
}

func testBodyHasNegativeSignal(t parse.TestFunc, src string) bool {
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
	for _, a := range t.Assignments {
		if sideEffectAssignmentNegative(a) {
			return true
		}
	}
	for _, a := range t.Asserts {
		if assertLooksNegativeForNameBody(a) || reBodyHTTPNeg.MatchString(a.Text) ||
			reBodyHTTPNeg.MatchString(a.Left+" "+a.Right) {
			return true
		}
	}
	// HTTP status / error tokens in the test body (fixtures, mock responses).
	snippet := testSourceSnippet(t, src)
	if snippet != "" && reBodyHTTPNeg.MatchString(snippet) {
		return true
	}
	sl := strings.ToLower(snippet)
	for _, tok := range []string{"pytest.raises", "assertraises", "side_effect", "permissionerror",
		"validationerror", "httperror", "notfound", "forbidden", "unauthorized"} {
		if strings.Contains(sl, tok) {
			return true
		}
	}
	return false
}

// assertLooksNegativeForNameBody is broader than only-happy-path's assertLooksNegative:
// any != / empty-literal counts so negative-named tests are not false-positived.
func assertLooksNegativeForNameBody(a parse.Assert) bool {
	if assertLooksNegative(a) {
		return true
	}
	al := strings.ToLower(a.Text)
	if strings.Contains(al, "!=") && !strings.Contains(al, "!= none") &&
		!strings.Contains(al, "is not none") {
		return true
	}
	return false
}

func NewNameBodyMismatch() scan.Rule {
	return nameBodyMismatch{}
}
