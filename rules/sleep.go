package rules

import (
	"regexp"
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

type sleepInTest struct{}

func (sleepInTest) ID() string { return "sleep-in-test" }

func (sleepInTest) NeedsAST() bool { return true }

// reSleepZero matches time/asyncio.sleep(0) / sleep(0.0) — yield-to-loop, not a wait.
var reSleepZero = regexp.MustCompile(`(?i)\bsleep\s*\(\s*0(\.0+)?\s*\)`)

// rePollDeadline markers that bound a polling loop around a sleep.
var rePollDeadline = regexp.MustCompile(`(?i)\b(fail_after|move_on_after|wait_for|wait_until|timeout|deadline|asyncio\.wait_for|anyio\.fail_after|anyio\.move_on_after)\b`)

func (sleepInTest) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return nil
	}
	src := string(file.Content)
	lines := strings.Split(src, "\n")
	var findings []scan.Finding
	for _, t := range model.Tests {
		q := qualName(t)
		for _, c := range t.Calls {
			if !isSleepCall(c.Name) {
				continue
			}
			line := lineAt(lines, c.Lineno)
			if reSleepZero.MatchString(line) {
				continue
			}
			if sleepInPollingLoop(t, c, lines, src) {
				continue
			}
			findings = append(findings, scan.Finding{
				File:     file.Path,
				Line:     c.Lineno,
				Rule:     "sleep-in-test",
				Severity: "warning",
				Message:  "time.sleep / asyncio.sleep in test; prefer event waits or freezegun / time-machine",
				Fix:      "remove `" + c.Name + "(...)`; wait on the condition (poll with a timeout or an event) or use a fake clock",
				QualName: q,
			})
		}
	}
	return findings
}

func isSleepCall(name string) bool {
	if name == "time.sleep" || name == "asyncio.sleep" {
		return true
	}
	return leafName(name) == "sleep" &&
		(strings.Contains(name, "time") || strings.Contains(name, "asyncio") || strings.Contains(name, "anyio"))
}

// sleepInPollingLoop is true when the sleep sits in a while/for that has a
// break/return or an explicit deadline/timeout helper in the same test.
func sleepInPollingLoop(t parse.TestFunc, c parse.Call, lines []string, src string) bool {
	for _, fl := range t.ForLoops {
		if c.Lineno > fl.Lineno && (fl.EndLineno <= 0 || c.Lineno <= fl.EndLineno) {
			if loopBodyHasBreakOrReturn(lines, fl.Lineno, fl.EndLineno) ||
				rePollDeadline.MatchString(srcSnippet(lines, t.Lineno, t.EndLineno)) {
				return true
			}
		}
	}
	// while-loops are not in the AST model; scan surrounding source for while + deadline.
	start := t.Lineno
	if start < 1 {
		start = 1
	}
	end := t.EndLineno
	if end <= 0 || end > len(lines) {
		end = len(lines)
	}
	snippet := srcSnippet(lines, start, end)
	if !strings.Contains(snippet, "while ") && !strings.Contains(snippet, "while:") {
		return false
	}
	if !rePollDeadline.MatchString(snippet) && !loopBodyHasBreakOrReturn(lines, start, end) {
		return false
	}
	// Sleep line must be indented deeper than a preceding while.
	sleepIndent := leadingSpaces(lineAt(lines, c.Lineno))
	for i := c.Lineno - 2; i >= start-1 && i >= 0; i-- {
		trim := strings.TrimSpace(lines[i])
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		if strings.HasPrefix(trim, "while ") || trim == "while:" || strings.HasPrefix(trim, "while(") {
			if leadingSpaces(lines[i]) < sleepIndent {
				return true
			}
			return false
		}
		if leadingSpaces(lines[i]) < sleepIndent && !strings.HasPrefix(trim, "else") {
			// Left the indented block without finding while.
			break
		}
	}
	return false
}

func srcSnippet(lines []string, start, end int) string {
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start > end {
		return ""
	}
	return strings.Join(lines[start-1:end], "\n")
}

func loopBodyHasBreakOrReturn(lines []string, start, end int) bool {
	if start < 1 {
		start = 1
	}
	if end <= 0 || end > len(lines) {
		end = len(lines)
	}
	for i := start; i <= end && i <= len(lines); i++ {
		trim := strings.TrimSpace(lines[i-1])
		if trim == "break" || strings.HasPrefix(trim, "break ") ||
			trim == "return" || strings.HasPrefix(trim, "return ") {
			return true
		}
	}
	return false
}

func NewSleepInTest() scan.Rule {
	return sleepInTest{}
}
