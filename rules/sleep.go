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
			if c.Name == "time.sleep" || c.Name == "asyncio.sleep" {
				findings = append(findings, scan.Finding{
					File:     file.Path,
					Line:     c.Lineno,
					Rule:     "sleep-in-test",
					Severity: "warning",
					Message:  "time.sleep / asyncio.sleep in test; prefer freezegun or mocks",
					QualName: q,
				})
			}
		}
	}
	return findings
}

func NewSleepInTest() scan.Rule {
	return sleepInTest{}
}
