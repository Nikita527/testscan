package rules

import (
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
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

type wallClockInTest struct{}

func (wallClockInTest) ID() string { return "wall-clock-in-test" }

func (wallClockInTest) NeedsAST() bool { return true }

func (wallClockInTest) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		return nil
	}
	fileFrozen := modelHasTimeFreeze(model)
	var findings []scan.Finding
	for _, t := range model.Tests {
		q := qualName(t)
		frozen := fileFrozen || testHasTimeFreeze(t)
		if frozen {
			continue
		}
		for _, c := range t.Calls {
			if !isWallClockCall(c.Name) {
				continue
			}
			findings = append(findings, scan.Finding{
				File:     file.Path,
				Line:     c.Lineno,
				Rule:     "wall-clock-in-test",
				Severity: "note",
				Message:  "wall-clock read (datetime.now / date.today) without freeze; prefer freezegun or time-machine",
				QualName: q,
			})
		}
	}
	return findings
}

func isWallClockCall(name string) bool {
	n := strings.ToLower(name)
	switch n {
	case "datetime.now", "datetime.utcnow", "datetime.today",
		"datetime.datetime.now", "datetime.datetime.utcnow", "datetime.datetime.today",
		"date.today", "datetime.date.today":
		return true
	}
	leaf := strings.ToLower(leafName(name))
	if leaf == "now" || leaf == "utcnow" || leaf == "today" {
		base := strings.ToLower(name)
		return strings.Contains(base, "datetime") || strings.Contains(base, "date")
	}
	return false
}

func modelHasTimeFreeze(model parse.Model) bool {
	for _, imp := range model.Imports {
		blob := strings.ToLower(imp.Module + " " + strings.Join(imp.Names, " "))
		if strings.Contains(blob, "freezegun") ||
			strings.Contains(blob, "time_machine") ||
			strings.Contains(blob, "freeze_time") {
			return true
		}
	}
	return false
}

func testHasTimeFreeze(t parse.TestFunc) bool {
	for _, d := range t.Decorators {
		dl := strings.ToLower(d)
		if strings.Contains(dl, "freeze_time") ||
			strings.Contains(dl, "freezegun") ||
			strings.Contains(dl, "time_machine") ||
			strings.Contains(dl, "travel(") {
			return true
		}
	}
	for _, c := range t.Calls {
		cl := strings.ToLower(c.Name)
		leaf := strings.ToLower(leafName(c.Name))
		if strings.Contains(cl, "freeze_time") ||
			strings.Contains(cl, "time_machine") ||
			strings.HasSuffix(cl, ".travel") ||
			leaf == "travel" {
			return true
		}
	}
	for _, f := range t.Fixtures {
		fl := strings.ToLower(f)
		if strings.Contains(fl, "freezer") || fl == "freezer" {
			return true
		}
	}
	return false
}

func NewWallClockInTest() scan.Rule {
	return wallClockInTest{}
}
