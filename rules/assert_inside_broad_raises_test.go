package rules_test

import (
	"testing"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/rules"
	"github.com/Nikita527/testscan/scan"
)

func TestAssertInsideBroadRaises_TupleForm(t *testing.T) {
	got := rules.NewAssertInsideBroadRaises().Check(scan.File{
		Path:    "t.py",
		Content: []byte("import pytest\ndef test_x():\n    with pytest.raises((Exception, OSError)):\n        connect()\n        assert False\n"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_x",
				Raises: []parse.Raise{{
					Exc:           "(Exception, OSError)",
					Lineno:        3,
					EndLineno:     5,
					BodyStmtCount: 2,
				}},
				Asserts: []parse.Assert{{
					Kind:   "truthy",
					Lineno: 5,
					Text:   "False",
				}},
			}},
		},
	})
	if len(got) != 1 || got[0].Rule != "assert-inside-broad-raises" {
		t.Fatalf("want 1 assert-inside-broad-raises, got %v", got)
	}
}

func TestAssertInsideBroadRaises_SpecificExcClean(t *testing.T) {
	got := rules.NewAssertInsideBroadRaises().Check(scan.File{
		Path:    "t.py",
		Content: []byte("x"),
		ModelOK: true,
		Model: parse.Model{
			Tests: []parse.TestFunc{{
				Name: "test_x",
				Raises: []parse.Raise{{
					Exc:       "(ValueError, TypeError)",
					Lineno:    3,
					EndLineno: 5,
				}},
				Asserts: []parse.Assert{{
					Kind:   "truthy",
					Lineno: 5,
					Text:   "False",
				}},
			}},
		},
	})
	if len(got) != 0 {
		t.Fatalf("specific tuple must be clean, got %v", got)
	}
}

func TestAssertInsideBroadRaises_PytestFailExact(t *testing.T) {
	mk := func(callName string) []scan.Finding {
		return rules.NewAssertInsideBroadRaises().Check(scan.File{
			Path:    "t.py",
			Content: []byte("x"),
			ModelOK: true,
			Model: parse.Model{
				Tests: []parse.TestFunc{{
					Name: "test_x",
					Raises: []parse.Raise{{
						Exc:       "Exception",
						Lineno:    3,
						EndLineno: 5,
					}},
					Calls: []parse.Call{{
						Name:   callName,
						Lineno: 5,
					}},
				}},
			},
		})
	}
	for _, name := range []string{"pytest.fail", "fail"} {
		if got := mk(name); len(got) != 1 {
			t.Fatalf("%s must flag, got %v", name, got)
		}
	}
	for _, name := range []string{"pytest.xfail", "other.fail", "pytest.helper.fail", "self.fail"} {
		if got := mk(name); len(got) != 0 {
			t.Fatalf("%s must not flag, got %v", name, got)
		}
	}
}
