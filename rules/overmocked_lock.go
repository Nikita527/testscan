package rules

import (
	"strings"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

type overmockedLock struct{}

func (overmockedLock) ID() string {
	return "overmocked-lock"
}

func (overmockedLock) NeedsAST() bool { return true }

func (overmockedLock) Check(file scan.File) []scan.Finding {
	model, err := astModel(file)
	if err != nil {
		if useHeuristic(file, err) {
			return overmockedLockHeuristic(file)
		}
		return nil
	}
	return overmockedLockFromAST(file, model)
}

func overmockedLockFromAST(file scan.File, model parse.Model) []scan.Finding {
	src := string(file.Content)
	var findings []scan.Finding
	for _, t := range model.Tests {
		if !testHasLockPatch(t, src) {
			continue
		}
		if !hasMockAssert(t) {
			continue
		}
		if hasNonMockAssert(t) {
			continue
		}
		q := t.QualName
		if q == "" {
			q = t.Name
		}
		findings = append(findings, scan.Finding{
			File:     file.Path,
			Line:     t.Lineno,
			Rule:     "overmocked-lock",
			Severity: "warning",
			Message:  "lock API patched and only mock asserts found in test: " + q,
			QualName: q,
		})
	}
	return findings
}

// unittestAssertNeedles are unittest.TestCase helpers (not mock assert_called*).
var unittestAssertNeedles = []string{
	"assertEqual(", "assertEquals(", "assertTrue(", "assertFalse(",
	"assertIn(", "assertNotIn(", "assertIs(", "assertIsNot(",
	"assertIsNone(", "assertIsNotNone(", "assertAlmostEqual(",
	"assertGreater(", "assertLess(", "assertRegex(", "assertCountEqual(",
	"assertDictEqual(", "assertListEqual(", "assertTupleEqual(",
	"assertSetEqual(", "assertRaisesRegex(", "assertLogs(",
}

// heuristicHasBehavioralAssert is file-scoped (heuristic fallback has no per-test
// split): any behavioral assert / raises on a code line clears the finding.
// Comments are ignored via lockScanLine.
func heuristicHasBehavioralAssert(src string) bool {
	for _, line := range strings.Split(src, "\n") {
		code := lockScanLine(line)
		if code == "" {
			continue
		}
		if strings.Contains(code, "assert ") ||
			strings.Contains(code, "pytest.raises") ||
			strings.Contains(code, "assertRaises") {
			return true
		}
		for _, n := range unittestAssertNeedles {
			// Prefer anchored forms: self.assertEqual( / assertEqual( at statement start.
			if strings.Contains(code, "."+n) || strings.HasPrefix(code, n) {
				return true
			}
		}
	}
	return false
}

func heuristicHasMockAssert(src string) bool {
	for _, line := range strings.Split(src, "\n") {
		code := lockScanLine(line)
		if code == "" {
			continue
		}
		if strings.Contains(code, "assert_called") || strings.Contains(code, "assert_has_calls") {
			return true
		}
	}
	return false
}

func overmockedLockHeuristic(file scan.File) []scan.Finding {
	src := string(file.Content)
	if !lockPatchInContentWithAssertHints(src, src) && !lockAttrReplaceInContent(src) {
		return nil
	}
	if !heuristicHasMockAssert(src) {
		return nil
	}
	if heuristicHasBehavioralAssert(src) {
		return nil
	}
	return []scan.Finding{{
		File:     file.Path,
		Line:     lineOfAny(src, lockPatchNeedles...),
		Rule:     "overmocked-lock",
		Severity: "warning",
		Message:  "lock API patched and only mock asserts found",
	}}
}

func NewOvermockedLock() scan.Rule {
	return overmockedLock{}
}
