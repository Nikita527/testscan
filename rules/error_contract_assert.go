package rules

import (
	"regexp"
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

const defaultErrorCodePath = "errors[].code"

var (
	reStatusCodeSide = regexp.MustCompile(`(?i)(status_code|status\.HTTP_[45]\d{2}|HTTP_[45]\d{2})`)
	reNon2xxLiteral  = regexp.MustCompile(`(?i)\b([45]\d{2}|HTTP_[45]\d{2})\b`)
)

// ErrorContractAssertOpts configures error-contract-assert.
type ErrorContractAssertOpts struct {
	ErrorCodePath   string
	ErrorStatusOnly *bool // nil → true
}

type errorContractAssert struct {
	errorCodePath   string
	errorStatusOnly bool
}

func (errorContractAssert) ID() string { return "error-contract-assert" }

func (errorContractAssert) NeedsAST() bool { return true }

func (e errorContractAssert) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return nil
	}
	path := e.errorCodePath
	if path == "" {
		path = defaultErrorCodePath
	}
	needles := errorCodePathNeedles(path)
	var findings []scan.Finding
	for _, t := range model.Tests {
		hasNon2xx := testHasNon2xxStatusAssert(t)
		if e.errorStatusOnly {
			if !hasNon2xx {
				continue
			}
		} else if !hasNon2xx && !testLooksLikeErrorContract(t) {
			// error-status-only=false: also cover error-shaped tests without a status assert.
			continue
		}
		if testHasErrorCodeAssert(t, needles) {
			continue
		}
		msg := "error-shaped test without checking error code path " + path
		if hasNon2xx {
			msg = "non-2xx status asserted without checking error code path " + path
		}
		findings = append(findings, scan.Finding{
			File:     file.Path,
			Line:     t.Lineno,
			Rule:     "error-contract-assert",
			Severity: "warning",
			Message:  msg,
			QualName: qualName(t),
		})
	}
	return findings
}

// testLooksLikeErrorContract is the gate when error_status_only is false:
// name/fixtures suggest an error path, or asserts mention an "errors" payload key.
func testLooksLikeErrorContract(t parse.TestFunc) bool {
	parts := []string{t.Name, t.QualName}
	parts = append(parts, t.Fixtures...)
	padded := "_" + strings.ToLower(strings.Join(parts, "_")) + "_"
	for _, cue := range []string{
		"error", "errors", "invalid", "forbidden", "unauthorized", "reject", "rejects",
		"fail", "fails", "failure", "failures",
	} {
		if strings.Contains(padded, "_"+cue+"_") {
			return true
		}
	}
	if reBodyHTTPNeg.MatchString(padded) {
		return true
	}
	for _, a := range t.Asserts {
		low := strings.ToLower(a.Text + " " + a.Left + " " + a.Right)
		if strings.Contains(low, "errors") {
			return true
		}
	}
	return false
}

func testHasNon2xxStatusAssert(t parse.TestFunc) bool {
	for _, a := range t.Asserts {
		blob := a.Text + " " + a.Left + " " + a.Right
		if reNon2xxLiteral.MatchString(blob) {
			if reStatusCodeSide.MatchString(blob) ||
				strings.Contains(strings.ToLower(a.Left), "status") ||
				strings.Contains(strings.ToLower(a.Right), "status") ||
				reNon2xxLiteral.MatchString(a.Right) ||
				reNon2xxLiteral.MatchString(a.Left) {
				return true
			}
		}
	}
	return false
}

func testHasErrorCodeAssert(t parse.TestFunc, needles []string) bool {
	for _, a := range t.Asserts {
		blob := strings.ToLower(a.Text + " " + a.Left + " " + a.Right)
		hits := 0
		for _, n := range needles {
			if n != "" && strings.Contains(blob, n) {
				hits++
			}
		}
		// Require at least two path segments when available (e.g. errors + code).
		need := 1
		if len(needles) >= 2 {
			need = 2
		}
		if hits >= need {
			return true
		}
	}
	return false
}

// errorCodePathNeedles turns "errors[].code" into lowercase substrings to find in asserts.
func errorCodePathNeedles(path string) []string {
	path = strings.TrimSpace(path)
	if path == "" {
		path = defaultErrorCodePath
	}
	parts := strings.Split(path, ".")
	var segs []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.ReplaceAll(p, "[]", "")
		p = strings.Trim(p, "[]")
		if p != "" {
			segs = append(segs, strings.ToLower(p))
		}
	}
	if len(segs) == 0 {
		return []string{"errors", "code"}
	}
	return segs
}

func NewErrorContractAssert(opts ErrorContractAssertOpts) scan.Rule {
	statusOnly := true
	if opts.ErrorStatusOnly != nil {
		statusOnly = *opts.ErrorStatusOnly
	}
	path := opts.ErrorCodePath
	if path == "" {
		path = defaultErrorCodePath
	}
	return errorContractAssert{
		errorCodePath:   path,
		errorStatusOnly: statusOnly,
	}
}
