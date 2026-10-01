package scan_test

import (
	"testing"

	"github.com/Nikita527/testscan/scan"
)

// withCatalog replaces scan.RulePrecision for one test (restored on cleanup).
// Tests using it must not run in parallel.
func withCatalog(t *testing.T, entries map[string]scan.PrecisionInfo) {
	t.Helper()
	old := scan.RulePrecision
	scan.RulePrecision = entries
	t.Cleanup(func() { scan.RulePrecision = old })
}

// withOverrides copies scan.RulePrecision, overrides the given rules with
// estimated precisions, and restores the original on cleanup.
func withOverrides(t *testing.T, est map[string]float64) {
	t.Helper()
	merged := make(map[string]scan.PrecisionInfo, len(scan.RulePrecision)+len(est))
	for k, v := range scan.RulePrecision {
		merged[k] = v
	}
	for k, p := range est {
		merged[k] = scan.PrecisionInfo{Precision: p, Source: scan.SourceEstimated}
	}
	withCatalog(t, merged)
}
