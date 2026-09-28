package rules

import (
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

type broadRaises struct{}

func (broadRaises) ID() string { return "broad-raises" }

func (broadRaises) NeedsAST() bool { return true }

func (broadRaises) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return nil
	}
	var findings []scan.Finding
	for _, t := range model.Tests {
		q := qualName(t)
		for _, r := range t.Raises {
			broadExc := isBroadException(r.Exc)
			multiBody := r.BodyStmtCount > 1
			excAttr := hasExcInfoAttrCheck(t, r)

			if !broadExc && excAttr {
				// Specific exception + exc_info.value.<attr> — not broad.
				continue
			}

			if broadExc && !r.HasMatch {
				findings = append(findings, scan.Finding{
					File:     file.Path,
					Line:     r.Lineno,
					Rule:     "broad-raises",
					Severity: "warning",
					Message:  "pytest.raises(Exception) is too broad; specify a concrete exception class",
					QualName: q,
				})
				continue
			}
			if multiBody {
				findings = append(findings, scan.Finding{
					File:     file.Path,
					Line:     r.Lineno,
					Rule:     "broad-raises",
					Severity: "warning",
					Message:  "raises body has multiple statements; narrow the scope",
					QualName: q,
				})
			}
		}
	}
	return findings
}

func isBroadException(exc string) bool {
	e := strings.TrimSpace(exc)
	if e == "" {
		return false
	}
	if e == "Exception" || e == "BaseException" {
		return true
	}
	return strings.HasSuffix(e, ".Exception") || strings.HasSuffix(e, ".BaseException")
}

func hasExcInfoAttrCheck(t parse.TestFunc, r parse.Raise) bool {
	asName := strings.TrimSpace(r.AsName)
	candidates := []string{}
	if asName != "" {
		candidates = append(candidates, asName+".value.")
	} else {
		candidates = append(candidates, "exc_info.value.", "exc.value.")
	}
	end := r.EndLineno
	if end <= 0 {
		end = r.Lineno
	}
	for _, a := range t.Asserts {
		if a.Lineno < r.Lineno {
			continue
		}
		if a.Lineno > end+8 {
			continue
		}
		text := a.Text + " " + a.Left + " " + a.Right
		for _, needle := range candidates {
			if strings.Contains(text, needle) {
				return true
			}
		}
	}
	return false
}

func NewBroadRaises() scan.Rule {
	return broadRaises{}
}
