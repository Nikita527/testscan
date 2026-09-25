package rules

import (
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

type weakAssert struct{}

func (weakAssert) ID() string { return "weak-assert" }

func (weakAssert) NeedsAST() bool { return true }

func (weakAssert) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return nil
	}
	var findings []scan.Finding
	for _, t := range model.Tests {
		if t.HasRaises || len(t.Asserts) == 0 {
			continue
		}
		allWeak := true
		firstLine := t.Asserts[0].Lineno
		for _, a := range t.Asserts {
			if !isWeakAssert(a) {
				allWeak = false
				break
			}
			if a.Lineno < firstLine {
				firstLine = a.Lineno
			}
		}
		if !allWeak {
			continue
		}
		findings = append(findings, scan.Finding{
			File:     file.Path,
			Line:     firstLine,
			Rule:     "weak-assert",
			Severity: "note",
			Message:  "only weak asserts (bare truthy / is not None / len vs 0)",
			QualName: qualName(t),
		})
	}
	return findings
}

func isWeakAssert(a parse.Assert) bool {
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
	// len(...) >/>=/!=/== 0
	if (strings.HasPrefix(left, "len(") || strings.Contains(text, "len(")) &&
		(right == "0" || strings.Contains(text, " 0") || strings.HasSuffix(text, "0")) {
		return true
	}
	return false
}

func NewWeakAssert() scan.Rule {
	return weakAssert{}
}
