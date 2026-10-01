package rules

import (
	"regexp"
	"strings"

	"github.com/Nikita527/testscan/scan"
)

type commentedAssert struct{}

func (commentedAssert) ID() string { return "commented-assert" }

func (commentedAssert) NeedsAST() bool { return false }

var (
	reCommentLine    = regexp.MustCompile(`^\s*#(.*)$`)
	reCommentAssert  = regexp.MustCompile(`^assert[\s(]`)
	reCommentAssertC = regexp.MustCompile(`^(self\.assert\w*|assert_\w+)\s*\(`)
)

// Heuristic: a comment is a commented-out assert only if its text (after '#')
// looks like Python code, not prose: it must start with lowercase `assert ...`,
// `self.assert*(` or `assert_*(`, and the expression must be statement-shaped
// (no two adjacent operands such as "assert on_commit invalidate ran", brackets
// and quotes not broken, no trailing sentence punctuation). Unclosed opening
// brackets are allowed (multi-line commented asserts).
func (commentedAssert) Check(file scan.File) []scan.Finding {
	src := string(file.Content)
	var findings []scan.Finding
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		m := reCommentLine.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if m == nil {
			continue
		}
		if !commentLooksLikeAssertStmt(strings.TrimSpace(m[1])) {
			continue
		}
		findings = append(findings, scan.Finding{
			File:     file.Path,
			Line:     i + 1,
			Rule:     "commented-assert",
			Severity: "note",
			Message:  "commented-out assert; remove or restore the check",
		})
	}
	return findings
}

func commentLooksLikeAssertStmt(text string) bool {
	switch {
	case reCommentAssert.MatchString(text):
		text = strings.TrimSpace(strings.TrimPrefix(text, "assert"))
	case reCommentAssertC.MatchString(text):
	default:
		return false
	}
	if text == "" || strings.HasSuffix(text, ".") || strings.HasSuffix(text, ":") {
		return false
	}
	return exprLooksLikeCode(text)
}

// exprLooksLikeCode is a tiny tokenizer-level check: balanced quotes, no
// closing bracket without opener, and no two adjacent operands.
func exprLooksLikeCode(s string) bool {
	depth := 0
	prevOperand := false
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == ' ' || c == '\t':
			i++
		case c == '"' || c == '\'' || isStringPrefixAt(s, i):
			// A string prefix (f, b, r, u, rb, br, fr, rf) is part of the literal.
			for s[i] != '"' && s[i] != '\'' {
				i++
			}
			c = s[i]
			j := i + 1
			for j < len(s) && s[j] != c {
				if s[j] == 0x5c {
					j++
				}
				j++
			}
			if j >= len(s) {
				return false
			}
			if prevOperand {
				return false
			}
			prevOperand = true
			i = j + 1
		case isIdentByte(c):
			j := i
			for j < len(s) && (isIdentByte(s[j]) || s[j] == '.' && j+1 < len(s) && isIdentByte(s[j+1])) {
				j++
			}
			word := s[i:j]
			i = j
			switch word {
			case "not", "and", "or", "in", "is", "if", "else", "for", "lambda":
				prevOperand = false
			default:
				if prevOperand {
					return false
				}
				prevOperand = true
			}
		case c == '(' || c == '[' || c == '{':
			// A call/subscript directly after an operand is fine.
			depth++
			prevOperand = false
			i++
		case c == ')' || c == ']' || c == '}':
			depth--
			if depth < 0 {
				return false
			}
			prevOperand = true
			i++
		default:
			prevOperand = false
			i++
		}
	}
	return true
}

// isStringPrefixAt reports whether s[i:] starts a string literal prefix
// (f, b, r, u, rb, br, fr, rf; case-insensitive) directly followed by a quote,
// and s[i] is not in the middle of a longer identifier.
func isStringPrefixAt(s string, i int) bool {
	if i > 0 && (isIdentByte(s[i-1]) || s[i-1] == '.') {
		return false
	}
	for n := 1; n <= 2 && i+n < len(s); n++ {
		if s[i+n] != '"' && s[i+n] != '\'' {
			continue
		}
		switch strings.ToLower(s[i : i+n]) {
		case "f", "b", "r", "u", "rb", "br", "fr", "rf":
			return true
		}
		return false
	}
	return false
}

func isIdentByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func NewCommentedAssert() scan.Rule {
	return commentedAssert{}
}
