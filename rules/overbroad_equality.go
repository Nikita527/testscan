package rules

import (
	"regexp"
	"strings"

	"github.com/Nikita527/testscan/scan"
)

const (
	overbroadLiteralMinLen    = 180
	overbroadLiteralMinCommas = 12
)

type overbroadEquality struct{}

func (overbroadEquality) ID() string { return "overbroad-equality" }

func (overbroadEquality) NeedsAST() bool { return true }

func (overbroadEquality) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return nil
	}
	var findings []scan.Finding
	for _, t := range model.Tests {
		q := qualName(t)
		seen := map[int]struct{}{}
		for _, a := range t.Asserts {
			if a.Kind != "compare" {
				continue
			}
			if _, dup := seen[a.Lineno]; dup {
				continue
			}
			if !isOverbroadLiteral(a.Left) && !isOverbroadLiteral(a.Right) {
				continue
			}
			// API contract asserts against response body are intentional full-payload checks.
			if isResponseBodySide(a.Left) || isResponseBodySide(a.Right) {
				continue
			}
			// Exact equality on identifier/code string collections (permission sets,
			// error codes) is an intentional contract check.
			if isCodeStringCollection(a.Left) || isCodeStringCollection(a.Right) {
				continue
			}
			seen[a.Lineno] = struct{}{}
			findings = append(findings, scan.Finding{
				File:     file.Path,
				Line:     a.Lineno,
				Rule:     "overbroad-equality",
				Severity: "note",
				Message:  "assert compares against a huge literal; review whether focused field checks would be clearer",
				QualName: q,
			})
		}
	}
	return findings
}

func isOverbroadLiteral(side string) bool {
	s := strings.TrimSpace(side)
	if s == "" {
		return false
	}
	open := s[0]
	if open != '{' && open != '[' && open != '(' {
		return false
	}
	close := map[byte]byte{'{': '}', '[': ']', '(': ')'}[open]
	if s[len(s)-1] != close {
		return false
	}
	if len(s) >= overbroadLiteralMinLen {
		return true
	}
	return countCommasOutsideStrings(s) >= overbroadLiteralMinCommas
}

// isResponseBodySide is true when the side's root expression is an API response
// body: <response|resp|res|r>.data[...]..., anything ending in .json() followed by
// any subscripts/attributes, and ORM .values()/.values_list() / error_details results.
func isResponseBodySide(side string) bool {
	s := strings.TrimSpace(strings.ToLower(side))
	if s == "" {
		return false
	}
	for _, tok := range []string{".json()", ".values(", ".values_list(", "error_details"} {
		if strings.Contains(s, tok) {
			return true
		}
	}
	for _, root := range []string{"response", "resp", "res", "r"} {
		if strings.HasPrefix(s, root+".data") {
			rest := s[len(root+".data"):]
			if rest == "" || rest[0] == '[' || rest[0] == '.' || rest[0] == ' ' {
				return true
			}
		}
	}
	// Dotted base such as self.response.data / client_response.data.
	if i := strings.Index(s, ".data"); i > 0 {
		base := s[:i]
		leaf := base[strings.LastIndex(base, ".")+1:]
		if leaf == "response" || leaf == "resp" || strings.HasSuffix(leaf, "_response") {
			return true
		}
	}
	return false
}

var reCodeString = regexp.MustCompile(`^(?:"[A-Za-z0-9_.:\-]+"|'[A-Za-z0-9_.:\-]+')$`)

// isCodeStringCollection is true for a set/list/tuple literal made only of
// identifier-like string literals (permission names, error codes).
func isCodeStringCollection(side string) bool {
	s := strings.TrimSpace(side)
	if len(s) < 2 || (s[0] != '{' && s[0] != '[' && s[0] != '(') {
		return false
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	if inner == "" {
		return false
	}
	var parts []string
	var quote byte
	start := 0
	for i := 0; i < len(inner); i++ {
		c := inner[i]
		if quote != 0 {
			switch c {
			case 0x5c:
				i++
			case quote:
				quote = 0
			}
			continue
		}
		switch c {
		case '"', 0x27:
			quote = c
		case ',':
			parts = append(parts, strings.TrimSpace(inner[start:i]))
			start = i + 1
		}
	}
	parts = append(parts, strings.TrimSpace(inner[start:]))
	n := 0
	for _, p := range parts {
		if p == "" {
			continue
		}
		if !reCodeString.MatchString(p) {
			return false
		}
		n++
	}
	return n > 0
}

func countCommasOutsideStrings(s string) int {
	n := 0
	var quote rune
	escape := false
	for _, r := range s {
		if escape {
			escape = false
			continue
		}
		if quote != 0 {
			if r == '\\' {
				escape = true
				continue
			}
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '"' || r == '\'' {
			quote = r
			continue
		}
		if r == ',' {
			n++
		}
	}
	return n
}

func NewOverbroadEquality() scan.Rule {
	return overbroadEquality{}
}
