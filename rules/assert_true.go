package rules

import (
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
	var findings []scan.Finding
	for _, t := range model.Tests {
		q := qualName(t)
		for _, a := range t.Asserts {
			switch a.Kind {
			case "truthy":
				if !isConstantTruthyAssert(a.Text) {
					continue
				}
				findings = append(findings, scan.Finding{
					File:     file.Path,
					Line:     a.Lineno,
					Rule:     "assert-true",
					Severity: "warning",
					Message:  "assert of a constant (True/1/\"…\") found in test",
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
