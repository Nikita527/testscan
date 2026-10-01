package rules

import (
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

type assertInEmptyableLoop struct{}

func (assertInEmptyableLoop) ID() string { return "assert-in-emptyable-loop" }

func (assertInEmptyableLoop) NeedsAST() bool { return true }

func (assertInEmptyableLoop) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return nil
	}
	var findings []scan.Finding
	for _, t := range model.Tests {
		if len(t.Asserts) == 0 {
			continue
		}
		risky := make([]parse.ForLoop, 0)
		for _, fl := range t.ForLoops {
			if !fl.OnlyAsserts {
				continue
			}
			if !iterLooksEmptyable(fl) {
				continue
			}
			if hasPreLoopNonEmptyGuard(t, fl) {
				continue
			}
			risky = append(risky, fl)
		}
		if len(risky) == 0 {
			continue
		}
		onlyFors := make([]struct{ start, end int }, 0, len(risky))
		for _, fl := range risky {
			end := fl.EndLineno
			if end < fl.Lineno {
				end = fl.Lineno
			}
			onlyFors = append(onlyFors, struct{ start, end int }{fl.Lineno, end})
		}
		allInside := true
		firstLine := t.Asserts[0].Lineno
		for _, a := range t.Asserts {
			inside := false
			for _, fl := range onlyFors {
				if a.Lineno >= fl.start && a.Lineno <= fl.end {
					inside = true
					break
				}
			}
			if !inside {
				allInside = false
				break
			}
			if a.Lineno < firstLine {
				firstLine = a.Lineno
			}
		}
		if !allInside {
			continue
		}
		hint := "links"
		if it := strings.TrimSpace(risky[0].IterText); it != "" {
			hint = it
		}
		findings = append(findings, scan.Finding{
			File:     file.Path,
			Line:     firstLine,
			Rule:     "assert-in-emptyable-loop",
			Severity: "warning",
			Fix:      `add ` + "`assert " + hint + "`" + ` before the loop so an empty collection fails the test`,
			Message:  `only asserts are inside a for-loop over an emptyable collection; assert the collection is non-empty first (e.g. assert ` + hint + `, "expected items")`,
			QualName: qualName(t),
		})
	}
	return findings
}

func iterLooksEmptyable(fl parse.ForLoop) bool {
	switch fl.IterKind {
	case "literal_nonempty", "range_const", "upper_name", "attr_const":
		return false
	default:
		// "other" or missing/legacy — treat as emptyable.
		return true
	}
}

func hasPreLoopNonEmptyGuard(t parse.TestFunc, fl parse.ForLoop) bool {
	iter := strings.TrimSpace(fl.IterText)
	if iter == "" {
		return false
	}
	for _, a := range t.Asserts {
		if a.Lineno >= fl.Lineno {
			continue
		}
		if assertGuardsNonEmpty(a, iter) {
			return true
		}
	}
	return false
}

func assertGuardsNonEmpty(a parse.Assert, iter string) bool {
	text := strings.TrimSpace(a.Text)
	if text == "" {
		return false
	}
	// Drop optional message: `items, "msg"` → `items`
	core := text
	if i := strings.Index(core, ","); i >= 0 {
		core = strings.TrimSpace(core[:i])
	}
	if core == iter {
		return true
	}
	left := strings.TrimSpace(a.Left)
	right := strings.TrimSpace(a.Right)
	lenCall := "len(" + iter + ")"
	if left == lenCall || strings.HasPrefix(left, lenCall) {
		switch right {
		case "0":
			// len(x) != 0 / > 0 / >= 1 handled via op in text
			return strings.Contains(text, "!=") || strings.Contains(text, ">") ||
				strings.Contains(text, ">=")
		case "1":
			return strings.Contains(text, ">=") || strings.Contains(text, "==") ||
				strings.Contains(text, ">")
		default:
			// len(x) > N / >= N for N>=1
			return strings.Contains(text, ">") || strings.Contains(text, ">=")
		}
	}
	if strings.Contains(text, lenCall) &&
		(strings.Contains(text, "> 0") || strings.Contains(text, ">= 1") ||
			strings.Contains(text, "!= 0") || strings.Contains(text, ">0")) {
		return true
	}
	return false
}

func NewAssertInEmptyableLoop() scan.Rule {
	return assertInEmptyableLoop{}
}
