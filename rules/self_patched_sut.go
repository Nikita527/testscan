package rules

import (
	"regexp"
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

type selfPatchedSUT struct{}

func (selfPatchedSUT) ID() string { return "self-patched-sut" }

func (selfPatchedSUT) NeedsAST() bool { return true }

var rePatchTarget = regexp.MustCompile(
	`(?i)(?:mock\.)?patch(?:\.object)?\s*\(\s*["']([^"']+)["']`,
)

func (selfPatchedSUT) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return nil
	}
	src := string(file.Content)
	var findings []scan.Finding
	for _, t := range model.Tests {
		targets := patchedTargetsInTest(t, src)
		if len(targets) == 0 {
			continue
		}
		q := qualName(t)
		seen := map[int]struct{}{}
		for _, target := range targets {
			if !testCallsPatchedTarget(t, target) {
				continue
			}
			for _, a := range t.Asserts {
				if _, dup := seen[a.Lineno]; dup {
					continue
				}
				if !assertOnSelfPatch(a, target) {
					continue
				}
				seen[a.Lineno] = struct{}{}
				findings = append(findings, scan.Finding{
					File:     file.Path,
					Line:     a.Lineno,
					Rule:     "self-patched-sut",
					Severity: "warning",
					Message:  "test patches the SUT under test and asserts the patch",
					QualName: q,
				})
			}
		}
	}
	return findings
}

func patchedTargetsInTest(t parse.TestFunc, src string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(target string) {
		target = strings.TrimSpace(target)
		if target == "" {
			return
		}
		if _, ok := seen[target]; ok {
			return
		}
		seen[target] = struct{}{}
		out = append(out, target)
	}
	for _, d := range t.Decorators {
		if m := rePatchTarget.FindStringSubmatch(d); len(m) > 1 {
			add(m[1])
		}
		if target, _, ok := parsePatchReturnValue(d); ok {
			add(target)
		}
	}
	// Body / with-statement patches: scan the test source range.
	blob := testSourceSnippet(t, src)
	for _, m := range rePatchTarget.FindAllStringSubmatch(blob, -1) {
		if len(m) > 1 {
			add(m[1])
		}
	}
	for _, m := range rePatchReturnValue.FindAllStringSubmatch(blob, -1) {
		if len(m) > 1 {
			add(m[1])
		}
	}
	return out
}

func testSourceSnippet(t parse.TestFunc, src string) string {
	if src == "" {
		return ""
	}
	lines := strings.Split(src, "\n")
	start := t.Lineno - 1
	if start < 0 {
		start = 0
	}
	end := t.EndLineno
	if end <= 0 || end > len(lines) {
		end = len(lines)
	}
	if start >= len(lines) {
		return ""
	}
	return strings.Join(lines[start:end], "\n")
}

func testCallsPatchedTarget(t parse.TestFunc, target string) bool {
	leaf := leafName(target)
	for _, c := range t.Calls {
		if c.Name == target || leafName(c.Name) == leaf {
			// Exclude the patch() call itself.
			if strings.Contains(strings.ToLower(c.Name), "patch") {
				continue
			}
			return true
		}
	}
	for _, a := range t.Asserts {
		if a.LeftIsCall && callMatchesTarget(a.Left, target) {
			return true
		}
		if a.RightIsCall && callMatchesTarget(a.Right, target) {
			return true
		}
	}
	return false
}

func callMatchesTarget(side, target string) bool {
	side = strings.TrimSpace(side)
	side = strings.TrimSuffix(side, "()")
	if side == "" {
		return false
	}
	return side == target || leafName(side) == leafName(target)
}

func assertOnSelfPatch(a parse.Assert, target string) bool {
	if directMockReturnValueAssert(a) {
		return true
	}
	if a.Kind != "compare" {
		return false
	}
	// assert mod.func() == X where func is the patched target
	if a.LeftIsCall && callMatchesTarget(a.Left, target) {
		return true
	}
	if a.RightIsCall && callMatchesTarget(a.Right, target) {
		return true
	}
	return false
}

func NewSelfPatchedSUT() scan.Rule {
	return selfPatchedSUT{}
}
