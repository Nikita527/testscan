package scan

import (
	"encoding/json"
	"fmt"
	"os"
)

// ComparePoint is a previous scan's confirmed metrics for trend comparison.
type ComparePoint struct {
	ConfirmedCount   int
	ConfirmedDensity float64
}

// LoadComparePoint reads a previous JSON report (wrapper or bare findings array)
// and returns confirmed count/density for trend comparison.
func LoadComparePoint(path string) (ComparePoint, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ComparePoint{}, fmt.Errorf("compare: %w", err)
	}

	var bare []Finding
	if err := json.Unmarshal(data, &bare); err == nil {
		if bare == nil {
			bare = []Finding{}
		}
		count := CountConfirmed(bare)
		files := uniqueFileCount(bare)
		return ComparePoint{
			ConfirmedCount:   count,
			ConfirmedDensity: ConfirmedDensity(count, files),
		}, nil
	}

	var report JSONReport
	if err2 := json.Unmarshal(data, &report); err2 != nil {
		return ComparePoint{}, fmt.Errorf("compare: %w", err2)
	}

	if len(report.Findings) > 0 {
		count := CountConfirmed(report.Findings)
		files := report.Summary.Files
		if files <= 0 {
			files = uniqueFileCount(report.Findings)
		}
		return ComparePoint{
			ConfirmedCount:   count,
			ConfirmedDensity: ConfirmedDensity(count, files),
		}, nil
	}

	return ComparePoint{
		ConfirmedCount:   report.Summary.ConfirmedCount,
		ConfirmedDensity: report.Summary.ConfirmedDensity,
	}, nil
}

func uniqueFileCount(findings []Finding) int {
	seen := make(map[string]struct{}, len(findings))
	for _, f := range findings {
		seen[f.File] = struct{}{}
	}
	return len(seen)
}
