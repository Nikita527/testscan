package rules

import (
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

// RaisesWithoutCheckOpts configures raises-without-check.
type RaisesWithoutCheckOpts struct {
	ErrorAttr        string
	ExceptionClasses []string
}

type raisesWithoutCheck struct {
	errorAttr        string
	exceptionClasses []string
}

func (raisesWithoutCheck) ID() string { return "raises-without-check" }

func (raisesWithoutCheck) NeedsAST() bool { return true }

func (r raisesWithoutCheck) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return nil
	}
	attr := r.errorAttr
	if attr == "" {
		attr = "code"
	}
	var findings []scan.Finding
	for _, t := range model.Tests {
		q := qualName(t)
		for _, raise := range t.Raises {
			if raise.HasMatch {
				continue
			}
			if isBroadException(raise.Exc) {
				continue
			}
			if len(r.exceptionClasses) > 0 && !exceptionClassAllowed(raise.Exc, r.exceptionClasses) {
				continue
			}
			if hasExcInfoNamedAttrCheck(t, raise, attr) {
				continue
			}
			findings = append(findings, scan.Finding{
				File:     file.Path,
				Line:     raise.Lineno,
				Rule:     "raises-without-check",
				Severity: "warning",
				Message:  "pytest.raises without match= or ." + attr + " check on the exception",
				QualName: q,
			})
		}
	}
	return findings
}

func exceptionClassAllowed(exc string, allow []string) bool {
	e := strings.TrimSpace(exc)
	leaf := leafName(e)
	for _, a := range allow {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if e == a || leaf == a || leaf == leafName(a) {
			return true
		}
	}
	return false
}

func hasExcInfoNamedAttrCheck(t parse.TestFunc, r parse.Raise, attr string) bool {
	attr = strings.TrimSpace(attr)
	if attr == "" {
		return false
	}
	asName := strings.TrimSpace(r.AsName)
	candidates := []string{}
	if asName != "" {
		candidates = append(candidates,
			asName+".value."+attr,
			asName+"."+attr,
		)
	} else {
		candidates = append(candidates,
			"exc_info.value."+attr,
			"exc.value."+attr,
		)
	}
	end := r.EndLineno
	if end <= 0 {
		end = r.Lineno
	}
	for _, a := range t.Asserts {
		if a.Lineno < r.Lineno {
			continue
		}
		if a.Lineno > end+12 {
			continue
		}
		text := a.Text + " " + a.Left + " " + a.Right
		for _, needle := range candidates {
			if strings.Contains(text, needle) {
				return true
			}
		}
		// getattr(exc_info.value, "code") style
		if strings.Contains(text, `"`+attr+`"`) || strings.Contains(text, `'`+attr+`'`) {
			low := strings.ToLower(text)
			if !strings.Contains(low, "getattr") {
				continue
			}
			excOK := strings.Contains(low, "exc_info") || strings.Contains(low, "exc.value")
			// Require as-name length ≥ 2 so `as e` does not match every "e" in the assert.
			if len(asName) >= 2 && strings.Contains(low, strings.ToLower(asName)) {
				excOK = true
			}
			if excOK {
				return true
			}
		}
	}
	return false
}

func NewRaisesWithoutCheck(opts RaisesWithoutCheckOpts) scan.Rule {
	attr := opts.ErrorAttr
	if attr == "" {
		attr = "code"
	}
	classes := append([]string{}, opts.ExceptionClasses...)
	return raisesWithoutCheck{
		errorAttr:        attr,
		exceptionClasses: classes,
	}
}
