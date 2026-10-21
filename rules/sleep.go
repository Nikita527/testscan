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

// rePollDeadlineCall matches deadline/timeout *helpers* that bound a polling loop.
// Bare identifiers like `timeout = 5` must not suppress real sleeps.
var rePollDeadlineCall = regexp.MustCompile(`(?i)\b(?:anyio\.|asyncio\.|trio\.)?(?:fail_after|move_on_after|wait_for|wait_until|timeout)\s*\(`)

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
			if sleepInDeadlinePollingLoop(t, c, lines) {
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

// sleepInDeadlinePollingLoop is true only when the sleep sits inside a while/for
// *and* the same test uses an explicit deadline helper (fail_after / wait_for / …).
// A bare `break`/`return` or a `timeout = N` variable alone does not suppress.
func sleepInDeadlinePollingLoop(t parse.TestFunc, c parse.Call, lines []string) bool {
	start := t.Lineno
	if start < 1 {
		start = 1
	}
	end := t.EndLineno
	if end <= 0 || end > len(lines) {
		end = len(lines)
	}
	snippet := srcSnippet(lines, start, end)
	if !rePollDeadlineCall.MatchString(snippet) {
		return false
	}
	if !sleepInsideLoop(t, c, lines, start) {
		return false
	}
	return true
}

func sleepInsideLoop(t parse.TestFunc, c parse.Call, lines []string, testStart int) bool {
	for _, fl := range t.ForLoops {
		if c.Lineno > fl.Lineno && (fl.EndLineno <= 0 || c.Lineno <= fl.EndLineno) {
			return true
		}
	}
	sleepIndent := leadingSpaces(lineAt(lines, c.Lineno))
	for i := c.Lineno - 2; i >= testStart-1 && i >= 0; i-- {
		trim := strings.TrimSpace(lines[i])
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		if strings.HasPrefix(trim, "while ") || trim == "while:" || strings.HasPrefix(trim, "while(") {
			return leadingSpaces(lines[i]) < sleepIndent
		}
		if leadingSpaces(lines[i]) < sleepIndent && !strings.HasPrefix(trim, "else") {
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

func NewSleepInTest() scan.Rule {
	return sleepInTest{}
}
