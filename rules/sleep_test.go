package rules_test

import (
	"testing"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/rules"
	"github.com/Nikita527/testscan/scan"
)

func TestSleepInTest_DatetimeNow(t *testing.T) {
	rule := rules.NewSleepInTest()

	hit := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("from datetime import datetime\ndef test_x():\n    datetime.now()\n    assert True\n"),
		ModelOK: true,
		Model: parse.Model{
			Imports: []parse.Import{{Kind: "from", Module: "datetime", Names: []string{"datetime"}}},
			Tests: []parse.TestFunc{{
				Name:  "test_x",
				Calls: []parse.Call{{Name: "datetime.now", Lineno: 3}},
			}},
		},
	})
	if len(hit) != 1 {
		t.Fatalf("want hit for datetime.now, got %v", hit)
	}
	if hit[0].Severity != "note" {
		t.Fatalf("datetime.now severity=%q, want note", hit[0].Severity)
	}

	sleep := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("import time\ndef test_x():\n    time.sleep(1)\n    assert True\n"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name:  "test_x",
				Calls: []parse.Call{{Name: "time.sleep", Lineno: 3}},
			}},
		},
	})
	if len(sleep) != 1 || sleep[0].Severity != "warning" {
		t.Fatalf("want warning for time.sleep, got %v", sleep)
	}

	frozen := rule.Check(scan.File{
		Path:    "t.py",
		Content: []byte("from freezegun import freeze_time\nfrom datetime import datetime\n@freeze_time('2024-01-01')\ndef test_x():\n    datetime.now()\n"),
		ModelOK: true,
		Model: parse.Model{
			Imports: []parse.Import{
				{Kind: "from", Module: "freezegun", Names: []string{"freeze_time"}},
				{Kind: "from", Module: "datetime", Names: []string{"datetime"}},
			},
			Tests: []parse.TestFunc{{
				Name:       "test_x",
				Decorators: []string{`freeze_time("2024-01-01")`},
				Calls:      []parse.Call{{Name: "datetime.now", Lineno: 5}},
			}},
		},
	})
	if len(frozen) != 0 {
		t.Fatalf("want clean with freezegun, got %v", frozen)
	}
}
