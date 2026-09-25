package rules_test

import (
	"testing"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/rules"
	"github.com/Nikita527/testscan/scan"
)

func TestAssertTrue_UnittestBoolCompare(t *testing.T) {
	got := rules.NewAssertTrue().Check(scan.File{
		Path:    "t.py",
		Content: []byte("def test_x():\n    self.assertTrue(a == b)\n"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name:     "test_x",
				QualName: "test_x",
				Lineno:   1,
				Asserts: []parse.Assert{{
					Kind:   "unittest_bool",
					Lineno: 2,
					Text:   "self.assertTrue(a == b)",
				}},
			}},
		},
	})
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %v", got)
	}
	if got[0].Rule != "assert-true" {
		t.Errorf("rule = %q", got[0].Rule)
	}
}
