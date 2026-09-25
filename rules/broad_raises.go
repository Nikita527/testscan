package rules

import (
	"strings"

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
			if broadExc && !r.HasMatch {
				findings = append(findings, scan.Finding{
					File:     file.Path,
					Line:     r.Lineno,
					Rule:     "broad-raises",
					Severity: "warning",
					Message:  "pytest.raises(Exception) without match= is too broad",
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

func NewBroadRaises() scan.Rule {
	return broadRaises{}
}
