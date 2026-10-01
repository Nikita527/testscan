package scan

import (
	"encoding/json"
	"fmt"
	"os"
)

// ComparePoint is a previous scan's actionable metrics for trend comparison.
// ActionableDensity is findings per 100 tests, or -1 when unknown (legacy
// reports that only carry confirmed_* / per-file density, and bare finding arrays).
type ComparePoint struct {
	ActionableCount   int
	ActionableDensity float64
	// Legacy is true when the point came from a pre-0.5 report (confirmed_*)
	// or a bare findings array; only the count is comparable.
	Legacy bool
}

// compareSummary decodes just the summary fields needed for a compare point.
// Pointer fields distinguish "absent" (old report) from zero.
type compareSummary struct {
	ActionableCount  *int     `json:"actionable_count"`
	ActionablePer100 *float64 `json:"actionable_per_100_tests"`
	ConfirmedCount   *int     `json:"confirmed_count"`
}

// LoadComparePoint reads a previous JSON report (wrapper or bare findings array)
// and returns actionable count/density for trend comparison. Reports written
// before 0.5 (confirmed_count only) fall back to that count with unknown density.
func LoadComparePoint(path string) (ComparePoint, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ComparePoint{}, fmt.Errorf("compare: %w", err)
	}

	var bare []Finding
	if err := json.Unmarshal(data, &bare); err == nil {
		return ComparePoint{
			ActionableCount:   CountActionable(bare),
			ActionableDensity: -1,
			Legacy:            true,
		}, nil
	}

	var report struct {
		Summary  compareSummary `json:"summary"`
		Findings []Finding      `json:"findings"`
	}
	if err2 := json.Unmarshal(data, &report); err2 != nil {
		return ComparePoint{}, fmt.Errorf("compare: %w", err2)
	}

	sum := report.Summary
	switch {
	case sum.ActionableCount != nil:
		dens := -1.0
		if sum.ActionablePer100 != nil {
			dens = *sum.ActionablePer100
		}
		return ComparePoint{ActionableCount: *sum.ActionableCount, ActionableDensity: dens}, nil
	case sum.ConfirmedCount != nil:
		return ComparePoint{ActionableCount: *sum.ConfirmedCount, ActionableDensity: -1, Legacy: true}, nil
	}
	return ComparePoint{
		ActionableCount:   CountActionable(report.Findings),
		ActionableDensity: -1,
		Legacy:            true,
	}, nil
}
