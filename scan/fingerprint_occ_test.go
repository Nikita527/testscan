package scan_test

import (
	"testing"

	"github.com/Nikita527/testscan/scan"
)

func TestAssignFingerprints_OccurrenceDistinguishesIdenticalShapes(t *testing.T) {
	content := []byte("" +
		"def test_same():\n" +
		"    assert False\n" +
		"    x = 1\n" +
		"    assert False\n")
	findings := []scan.Finding{
		{File: "t.py", Line: 2, Rule: "assert-true", QualName: "test_same", Message: "c", Severity: "warning"},
		{File: "t.py", Line: 4, Rule: "assert-true", QualName: "test_same", Message: "c", Severity: "warning"},
	}
	scan.AssignFingerprints(findings, map[string][]byte{"t.py": content}, "")
	if findings[0].Fingerprint == "" || findings[1].Fingerprint == "" {
		t.Fatal("expected fingerprints")
	}
	if findings[0].Fingerprint == findings[1].Fingerprint {
		t.Fatalf("identical-shape findings must differ by occurrence: both %s", findings[0].Fingerprint)
	}
}

func TestAssignFingerprints_OccurrenceStableUnderLineShift(t *testing.T) {
	// Padding before the test shifts line numbers but keeps the test-body snippet identical.
	base := []byte("def test_same():\n    assert False\n    assert False\n")
	shifted := []byte("# pad\n# pad\ndef test_same():\n    assert False\n    assert False\n")
	a := []scan.Finding{
		{File: "t.py", Line: 2, Rule: "assert-true", QualName: "test_same", Message: "c"},
		{File: "t.py", Line: 3, Rule: "assert-true", QualName: "test_same", Message: "c"},
	}
	b := []scan.Finding{
		{File: "t.py", Line: 4, Rule: "assert-true", QualName: "test_same", Message: "c"},
		{File: "t.py", Line: 5, Rule: "assert-true", QualName: "test_same", Message: "c"},
	}
	scan.AssignFingerprints(a, map[string][]byte{"t.py": base}, "")
	scan.AssignFingerprints(b, map[string][]byte{"t.py": shifted}, "")
	if a[0].Fingerprint != b[0].Fingerprint || a[1].Fingerprint != b[1].Fingerprint {
		t.Fatalf("line shift must keep occurrence fingerprints: %v vs %v",
			[]string{a[0].Fingerprint, a[1].Fingerprint},
			[]string{b[0].Fingerprint, b[1].Fingerprint})
	}
}
