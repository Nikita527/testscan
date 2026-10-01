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

// Heuristic: a test name is a "failure claim" only when a failure word describes
// the outcome (fails_with, raises, rejects_invalid, is_rejected, returns_error,
// errors_on, *_404). It is NOT a claim when the name is negated or resilient
// (does_not_fail, never_raises, fails_open, fallback, noop, survives, ...), when
// failure is only the precondition ("_when_x_fails"), or when "reject" is a domain
// verb (bulk_reject, reject_persists). Only the part of the name before the first
// when/if/given/after/during is inspected for the claim, unless that part is empty.
//
// A body "checks failure" if it has pytest.raises / 4xx tokens (original signals) or
// any of: comparison against FAILED/ERROR/REJECTED/... members or strings,
// error_code/.code comparisons, non-empty errors/issues/violations/failures,
// message/error/reason existence checks, failed counters, try/except/else-fail,
// assert_not_called, not x.is_valid() / assert not r.ok.
var (
	reNameNegativeCue = regexp.MustCompile(`(?i)(^|_)(fails?|invalid|raises?|forbidden|(is|was|gets?|be|are)_rejected|rejects?_(invalid|bad|malformed|missing|duplicate|unknown|empty|unauthori[sz]ed|anonymous|wrong|expired|non|nonexistent|without|with|if|when|too))(_|$)|(^|_)rejects$|(^|_)returns?_(an_?)?errors?(_|$)|(^|_)errors?_(on|for)(_|$)|(^|_)(returns?_)?[45]\d{2}(_|$)|_[45]\d{2}(_|$)|HTTP_[45]\d{2}`)
	reNameNegated     = regexp.MustCompile(`(?i)(^|_)(does_?n[o']?t|do_not|not_(fail|raise|reject|error)|never|fails?_(open|closed)|fail_(open|closed)|fallbacks?|falls?_back|no_?op|no_errors?|without_(error|fail|rais)|survives?|tolerates?|ignores?|still)(_|$)`)
	reNameSplit       = regexp.MustCompile(`(?i)_(when|if|given|after|during|while)_`)
	reBodyHTTPNeg     = regexp.MustCompile(`(?i)\b([45]\d{2}|HTTP_[45]\d{2})\b`)

	reAssertCmpOp    = regexp.MustCompile(`==|!=|>|<|\bis\b|\bin\b`)
	reFailStatusWord = regexp.MustCompile(`(?i)(^|[^a-z0-9])(failed|error|rejected|partial|skipped|denied|inactive|invalid[a-z_]*|[a-z0-9]+_error|[a-z0-9]+_failed)([^a-z0-9]|$)`)
	reFailCodeWord   = regexp.MustCompile(`(?i)error_?code|\.code\b|review_reason_code|last_error`)
	reFailCollection = regexp.MustCompile(`(?i)\b(errors|issues|violations|failures)\b`)
	reFailMessage    = regexp.MustCompile(`(?i)(message|error|issue|reason)`)
	reFailCounter    = regexp.MustCompile(`(?i)\bfailed(_count)?\b|\[["']failed["']\]`)
	reEmptyCheck     = regexp.MustCompile(`==\s*(0|\[\]|\(\)|\{\})|\bis\s+None\b|^\s*not\b|\bnot\s+(in\s+)?\w`)
	reIsValidNeg     = regexp.MustCompile(`(?i)(not\s+[\w.\[\]"']*is_valid)|(is_valid(\(\))?\s+is\s+False)|(is_valid(\(\))?\s*==\s*False)|^\s*not\s+[\w.]*\.ok\b`)
	reTryElseFail    = regexp.MustCompile(`(?s)\bexcept\b.*?\belse\s*:\s*\n?\s*(raise\s+AssertionError|pytest\.fail|self\.fail|raise\s)`)
)

// nameClaimsFailure reports whether the test name claims a failure outcome.
func nameClaimsFailure(name string) bool {
	n := strings.TrimPrefix(name, "test_")
	n = strings.TrimPrefix(n, "test")
	if reNameNegated.MatchString(n) {
		return false
	}
	claim := n
	if loc := reNameSplit.FindStringIndex(n); loc != nil && loc[0] > 0 {
		claim = n[:loc[0]]
	}
	return reNameNegativeCue.MatchString(claim)
}

func (nameBodyMismatch) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return nil
	}
	src := string(file.Content)
	var findings []scan.Finding
	for _, t := range model.Tests {
		if !nameClaimsFailure(t.Name) {
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
		if assertChecksFailure(a) {
			return true
		}
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
	if reTryElseFail.MatchString(snippet) {
		return true
	}
	for _, tok := range []string{"assert_not_called", "pytest.raises", "assertraises", "side_effect", "permissionerror",
		"validationerror", "httperror", "notfound", "forbidden", "unauthorized"} {
		if strings.Contains(sl, tok) {
			return true
		}
	}
	return false
}

// assertChecksFailure recognises asserts that check a failure outcome in domain
// terms (statuses, codes, error collections, messages, counters, validity).
func assertChecksFailure(a parse.Assert) bool {
	txt := strings.TrimSpace(a.Text)
	txt = strings.TrimSpace(strings.TrimPrefix(txt, "assert "))
	if txt == "" {
		return false
	}
	hasOp := reAssertCmpOp.MatchString(txt)
	negated := reEmptyCheck.MatchString(strings.ReplaceAll(txt, "is not None", "isnotnone"))
	if reIsValidNeg.MatchString(txt) {
		return true
	}
	if hasOp && reFailStatusWord.MatchString(txt) {
		return true
	}
	if hasOp && reFailCodeWord.MatchString(txt) {
		return true
	}
	if reFailCounter.MatchString(txt) && hasOp {
		return true
	}
	if !negated && reFailCollection.MatchString(txt) {
		return true
	}
	if !negated && reFailMessage.MatchString(txt) && (!hasOp || strings.Contains(txt, "is not None")) {
		return true
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
