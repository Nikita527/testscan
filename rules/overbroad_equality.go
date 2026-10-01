package rules

import (
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

// isResponseBodySide is true for response/resp .data / .json() API body attributes.
func isResponseBodySide(side string) bool {
	s := strings.TrimSpace(strings.ToLower(side))
	if s == "" {
		return false
	}
	if strings.HasSuffix(s, ".json()") {
		return true
	}
	switch s {
	case "response.data", "resp.data":
		return true
	}
	if strings.HasSuffix(s, ".data") {
		base := strings.TrimSuffix(s, ".data")
		leaf := base
		if i := strings.LastIndex(base, "."); i >= 0 {
			leaf = base[i+1:]
		}
		return leaf == "response" || leaf == "resp"
	}
	return false
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
