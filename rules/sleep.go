package rules

import (
	"github.com/Nikita527/testscan/scan"
)

type sleepInTest struct{}

func (sleepInTest) ID() string { return "sleep-in-test" }

func (sleepInTest) NeedsAST() bool { return true }

func (sleepInTest) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return nil
	}
	var findings []scan.Finding
	for _, t := range model.Tests {
		q := qualName(t)
		for _, c := range t.Calls {
			if !isSleepCall(c.Name) {
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
	return name == "time.sleep" || name == "asyncio.sleep"
}

func NewSleepInTest() scan.Rule {
	return sleepInTest{}
}
