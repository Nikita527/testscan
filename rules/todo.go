package rules

import (
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

type todoTest struct{}

func (todoTest) ID() string {
	return "todo-test"
}

func (todoTest) NeedsAST() bool { return true }

func (todoTest) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		if useHeuristic(file, err) {
			return todoTestHeuristic(file)
		}
		return nil
	}
	return todoTestFromAST(file, model)
}

func todoTestFromAST(file scan.File, model parse.Model) []scan.Finding {
	var findings []scan.Finding
	src := string(file.Content)
	for _, t := range model.Tests {
		q := t.QualName
		if q == "" {
			q = t.Name
		}
		if line, ok := todoFromDecorators(t); ok {
			findings = append(findings, scan.Finding{
				File:     file.Path,
				Line:     line,
				Rule:     "todo-test",
				Severity: "warning",
				Message:  "todo/skip marker found in test",
				QualName: q,
			})
			continue
		}
		if line, ok := todoFromCalls(t, src); ok {
			findings = append(findings, scan.Finding{
				File:     file.Path,
				Line:     line,
				Rule:     "todo-test",
				Severity: "warning",
				Message:  "todo/skip marker found in test",
				QualName: q,
			})
		}
	}
	return findings
}

func todoFromDecorators(t parse.TestFunc) (int, bool) {
	// Decorators like @pytest.mark.skip(reason=...) belong to skip-without-reason.
	// todo-test only hits when the decorator itself is an explicit TODO/FIXME placeholder.
	for _, d := range t.Decorators {
		base := d
		if i := strings.Index(d, "("); i >= 0 {
			base = d[:i]
		}
		isSkipDec := strings.HasSuffix(base, "mark.skip") || base == "pytest.mark.skip" ||
			strings.HasSuffix(base, "mark.xfail") || base == "pytest.mark.xfail" ||
			base == "unittest.skip" || strings.HasSuffix(base, "unittest.skip")
		if !isSkipDec {
			continue
		}
		if containsTODOLiteral(d) {
			return t.Lineno, true
		}
	}
	return 0, false
}

func todoFromCalls(t parse.TestFunc, src string) (int, bool) {
	lines := strings.Split(src, "\n")
	for _, c := range t.Calls {
		leaf := leafName(c.Name)
		line := lineAt(lines, c.Lineno)
		switch {
		case c.Name == "pytest.fail" || strings.HasSuffix(c.Name, ".fail"):
			if containsTODOLiteral(line) {
				return c.Lineno, true
			}
		case c.Name == "pytest.skip" || c.Name == "unittest.skip" ||
			(leaf == "skip" && (strings.HasPrefix(c.Name, "pytest.") || strings.HasPrefix(c.Name, "unittest."))):
			// Bare skip() or skip("TODO") — hit. Dynamic f-string reasons — not.
			if isPlaceholderSkip(line) {
				return c.Lineno, true
			}
		}
	}
	for _, a := range t.Asserts {
		if containsTODOLiteral(a.Text) && (strings.Contains(a.Text, "False") || a.Kind == "truthy") {
			return a.Lineno, true
		}
		line := lineAt(lines, a.Lineno)
		if strings.Contains(line, "assert False") && containsTODOLiteral(line) {
			return a.Lineno, true
		}
	}
	return 0, false
}

func lineAt(lines []string, lineno int) string {
	if lineno <= 0 || lineno > len(lines) {
		return ""
	}
	return lines[lineno-1]
}

func containsTODOLiteral(s string) bool {
	return strings.Contains(s, "TODO") || strings.Contains(s, "FIXME") || strings.Contains(s, "NotImplemented")
}

// isPlaceholderSkip: pytest.skip() / skip("TODO") — not f"..." or long dynamic reasons.
func isPlaceholderSkip(line string) bool {
	t := strings.TrimSpace(line)
	if !strings.Contains(t, "skip") {
		return false
	}
	// f-string or .format dynamic → not a todo placeholder
	if strings.Contains(t, "skip(f\"") || strings.Contains(t, "skip(f'") ||
		strings.Contains(t, "skip(F\"") || strings.Contains(t, ".format(") {
		return false
	}
	// bare skip() / skip()
	if strings.Contains(t, "skip()") || strings.Contains(t, "skip( )") {
		return true
	}
	if containsTODOLiteral(t) {
		return true
	}
	// skip with a short constant that looks like placeholder
	for _, p := range []string{`"TODO"`, `'TODO'`, `"todo"`, `'todo'`, `"not implemented"`, `'not implemented'`} {
		if strings.Contains(t, p) {
			return true
		}
	}
	return false
}

func todoTestHeuristic(file scan.File) []scan.Finding {
	src := string(file.Content)
	markers := []string{
		`pytest.fail("TODO")`,
		`pytest.fail('TODO')`,
		`assert False, "TODO"`,
		`assert False, 'TODO'`,
		"pytest.skip()",
	}
	for _, m := range markers {
		if strings.Contains(src, m) {
			return []scan.Finding{{
				File:     file.Path,
				Line:     lineOf(src, m),
				Rule:     "todo-test",
				Severity: "warning",
				Message:  "todo/skip marker found in test",
			}}
		}
	}
	// Decorator with explicit TODO/FIXME in reason (not bare skip — see skip-without-reason).
	for i, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "@") {
			continue
		}
		if !(strings.Contains(t, "mark.skip") || strings.Contains(t, "mark.xfail") ||
			strings.Contains(t, "unittest.skip")) {
			continue
		}
		if containsTODOLiteral(t) {
			return []scan.Finding{{
				File:     file.Path,
				Line:     i + 1,
				Rule:     "todo-test",
				Severity: "warning",
				Message:  "todo/skip marker found in test",
			}}
		}
	}
	return nil
}

func NewTodoTest() scan.Rule {
	return todoTest{}
}
