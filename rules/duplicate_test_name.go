package rules

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

type duplicateTestName struct{}

func (duplicateTestName) ID() string {
	return "duplicate-test-name"
}

func (duplicateTestName) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return duplicateTestNameHeuristic(file)
	}
	return duplicateTestNameFromAST(file, model)
}

func duplicateTestNameFromAST(file scan.File, model parse.Model) []scan.Finding {
	var findings []scan.Finding
	seen := map[string]int{}
	for _, t := range model.Tests {
		if _, dup := seen[t.Name]; dup {
			findings = append(findings, scan.Finding{
				File:     file.Path,
				Line:     t.Lineno,
				Rule:     "duplicate-test-name",
				Severity: "error",
				Message:  "duplicate test function name: " + t.Name,
			})
			continue
		}
		seen[t.Name] = t.Lineno
	}
	return findings
}

func duplicateTestNameHeuristic(file scan.File) []scan.Finding {
	var findings []scan.Finding
	seen := map[string]int{}
	src := string(file.Content)

	for i, line := range strings.Split(src, "\n") {
		name, ok := testDefName(line)
		if !ok {
			continue
		}
		lineNo := i + 1
		if _, dup := seen[name]; dup {
			findings = append(findings, scan.Finding{
				File:     file.Path,
				Line:     lineNo,
				Rule:     "duplicate-test-name",
				Severity: "error",
				Message:  "duplicate test function name: " + name,
			})
			continue
		}
		seen[name] = lineNo
	}
	return findings
}

// testDefName:^\s*(async )?def test_\w+
func testDefName(line string) (string, bool) {
	t := strings.TrimLeftFunc(line, unicode.IsSpace)
	if strings.HasPrefix(t, "async def ") {
		t = strings.TrimPrefix(t, "async def ")
	} else if strings.HasPrefix(t, "def ") {
		t = strings.TrimPrefix(t, "def ")
	} else {
		return "", false
	}
	t = strings.TrimLeftFunc(t, unicode.IsSpace)
	if !strings.HasPrefix(t, "test_") {
		return "", false
	}
	end := 0
	for i, r := range t {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			end = i + utf8.RuneLen(r)
			continue
		}
		break
	}
	if end == 0 {
		return "", false
	}
	return t[:end], true
}

func NewDuplicateTestName() scan.Rule {
	return duplicateTestName{}
}
