package rules

import (
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

var knownMockAssertLeaves = map[string]struct{}{
	"assert_called":           {},
	"assert_called_once":      {},
	"assert_called_with":      {},
	"assert_called_once_with": {},
	"assert_any_call":         {},
	"assert_has_calls":        {},
	"assert_not_called":       {},
}

var fakeMockAttrLeaves = map[string]struct{}{
	"called_once_with": {},
	"called_with":      {},
	"called_once":      {},
	"called":           {},
}

type fakeMockAssert struct{}

func (fakeMockAssert) ID() string { return "fake-mock-assert" }

func (fakeMockAssert) NeedsAST() bool { return true }

func (fakeMockAssert) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return nil
	}
	var findings []scan.Finding
	for _, t := range model.Tests {
		q := qualName(t)
		seen := map[int]struct{}{}
		for _, c := range t.Calls {
			leaf := leafName(c.Name)
			if _, ok := knownMockAssertLeaves[leaf]; ok {
				continue
			}
			if !isFakeMockCallLeaf(leaf) {
				continue
			}
			if _, dup := seen[c.Lineno]; dup {
				continue
			}
			seen[c.Lineno] = struct{}{}
			findings = append(findings, scan.Finding{
				File:     file.Path,
				Line:     c.Lineno,
				Rule:     "fake-mock-assert",
				Severity: "warning",
				Message:  "suspicious mock assert (typo or non-existent method)",
				QualName: q,
			})
		}
		for _, a := range t.Asserts {
			if a.Kind == "mock_method" || a.Kind != "truthy" {
				continue
			}
			if !isFakeMockTruthyText(a.Text) {
				continue
			}
			if _, dup := seen[a.Lineno]; dup {
				continue
			}
			seen[a.Lineno] = struct{}{}
			findings = append(findings, scan.Finding{
				File:     file.Path,
				Line:     a.Lineno,
				Rule:     "fake-mock-assert",
				Severity: "warning",
				Message:  "suspicious mock assert (typo or non-existent method)",
				QualName: q,
			})
		}
	}
	return findings
}

func isFakeMockCallLeaf(leaf string) bool {
	if _, ok := fakeMockAttrLeaves[leaf]; ok {
		return true
	}
	if !strings.HasPrefix(leaf, "assert_") {
		return false
	}
	if _, ok := knownMockAssertLeaves[leaf]; ok {
		return false
	}
	for known := range knownMockAssertLeaves {
		if editDistance(leaf, known) <= 2 {
			return true
		}
	}
	return strings.HasPrefix(leaf, "assert_call")
}

func isFakeMockTruthyText(text string) bool {
	t := strings.TrimSpace(text)
	t = strings.TrimPrefix(t, "not ")
	_, ok := fakeMockAttrLeaves[leafName(t)]
	return ok
}

func editDistance(a, b string) int {
	if a == b {
		return 0
	}
	la, lb := len(a), len(b)
	if absInt(la-lb) > 2 {
		return 3
	}
	prev := make([]int, lb+1)
	cur := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		cur[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = minInt(prev[j]+1, minInt(cur[j-1]+1, prev[j-1]+cost))
		}
		prev, cur = cur, prev
	}
	return prev[lb]
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func qualName(t parse.TestFunc) string {
	if t.QualName != "" {
		return t.QualName
	}
	return t.Name
}

func NewFakeMockAssert() scan.Rule {
	return fakeMockAssert{}
}
