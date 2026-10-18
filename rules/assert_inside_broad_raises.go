package rules

import (
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

// assertInsideBroadRaises flags assert False / pytest.fail inside
// pytest.raises(Exception|BaseException|AssertionError) (including the tuple
// form pytest.raises((Exception, ...))). AssertionError from the assert is
// swallowed by the broad raises, so the test cannot fail.
type assertInsideBroadRaises struct{}

func (assertInsideBroadRaises) ID() string { return "assert-inside-broad-raises" }

func (assertInsideBroadRaises) NeedsAST() bool { return true }

func (assertInsideBroadRaises) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return nil
	}
	var findings []scan.Finding
	for _, t := range model.Tests {
		q := qualName(t)
		for _, r := range t.Raises {
			if !isTrapRaisesException(r.Exc) {
				continue
			}
			end := r.EndLineno
			if end <= 0 {
				end = r.Lineno
			}
			seen := map[int]struct{}{}
			for _, a := range t.Asserts {
				if a.Lineno <= r.Lineno || a.Lineno > end {
					continue
				}
				if !isFailingConstantAssert(a) {
					continue
				}
				if _, dup := seen[a.Lineno]; dup {
					continue
				}
				seen[a.Lineno] = struct{}{}
				findings = append(findings, scan.Finding{
					File:     file.Path,
					Line:     a.Lineno,
					Rule:     "assert-inside-broad-raises",
					Severity: "warning",
					Message:  "assert False / failing assert inside pytest.raises(" + trapExcLabel(r.Exc) + ") is swallowed by the broad catch",
					QualName: q,
				})
			}
			for _, c := range t.Calls {
				if c.Lineno <= r.Lineno || c.Lineno > end {
					continue
				}
				if !isPytestFailCall(c.Name) {
					continue
				}
				if _, dup := seen[c.Lineno]; dup {
					continue
				}
				seen[c.Lineno] = struct{}{}
				findings = append(findings, scan.Finding{
					File:     file.Path,
					Line:     c.Lineno,
					Rule:     "assert-inside-broad-raises",
					Severity: "warning",
					Message:  "pytest.fail inside pytest.raises(" + trapExcLabel(r.Exc) + ") is swallowed by the broad catch",
					QualName: q,
				})
			}
		}
	}
	return findings
}

func isTrapRaisesException(exc string) bool {
	e := strings.TrimSpace(exc)
	if e == "" {
		return false
	}
	// Tuple form: (Exception, OSError) / (Exception,)
	if strings.HasPrefix(e, "(") && strings.HasSuffix(e, ")") {
		inner := strings.TrimSpace(e[1 : len(e)-1])
		if inner == "" {
			return false
		}
		for _, part := range splitRaiseExcParts(inner) {
			if isSingleTrapException(part) {
				return true
			}
		}
		return false
	}
	return isSingleTrapException(e)
}

func splitRaiseExcParts(inner string) []string {
	var parts []string
	depth, quote := 0, byte(0)
	start := 0
	for i := 0; i < len(inner); i++ {
		c := inner[i]
		if quote != 0 {
			if c == '\\' {
				i++
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth != 0 {
				continue
			}
			parts = append(parts, strings.TrimSpace(inner[start:i]))
			start = i + 1
		}
	}
	parts = append(parts, strings.TrimSpace(inner[start:]))
	out := parts[:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func isSingleTrapException(exc string) bool {
	e := strings.TrimSpace(exc)
	if e == "" {
		return false
	}
	switch e {
	case "Exception", "BaseException", "AssertionError":
		return true
	}
	return strings.HasSuffix(e, ".Exception") ||
		strings.HasSuffix(e, ".BaseException") ||
		strings.HasSuffix(e, ".AssertionError")
}

func trapExcLabel(exc string) string {
	e := strings.TrimSpace(exc)
	if e == "" {
		return "Exception"
	}
	if strings.HasPrefix(e, "(") {
		return e
	}
	if i := strings.LastIndex(e, "."); i >= 0 {
		return e[i+1:]
	}
	return e
}

func isFailingConstantAssert(a parse.Assert) bool {
	t := strings.TrimSpace(a.Text)
	if i := strings.Index(t, ","); i >= 0 {
		t = strings.TrimSpace(t[:i])
	}
	switch t {
	case "False", "0", "None", `""`, "''":
		return true
	}
	return false
}

func isPytestFailCall(name string) bool {
	if name == "pytest.fail" {
		return true
	}
	return strings.HasSuffix(name, ".fail") && strings.HasPrefix(name, "pytest.")
}

func NewAssertInsideBroadRaises() scan.Rule {
	return assertInsideBroadRaises{}
}
