package scan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type baselineKey struct {
	file string
	line int
	rule string
}

func legacyKeyOf(f Finding) baselineKey {
	return baselineKey{
		file: filepath.ToSlash(filepath.Clean(f.File)),
		line: f.Line,
		rule: f.Rule,
	}
}

// BaselineResult is the outcome of FilterBaseline.
type BaselineResult struct {
	Findings        []Finding
	UsedLegacyMatch bool // true if any finding matched old file+line+rule entry
}

// FilterBaseline removes findings that match the baseline.
// Prefer Fingerprint when present on both sides; fall back to legacy file+line+rule
// when a baseline entry has an empty Fingerprint.
func FilterBaseline(findings, baseline []Finding) BaselineResult {
	if len(baseline) == 0 {
		return BaselineResult{Findings: findings}
	}

	fpSuppressed := make(map[string]struct{})
	legacySuppressed := make(map[baselineKey]struct{})
	for _, b := range baseline {
		if b.Fingerprint != "" {
			fpSuppressed[b.Fingerprint] = struct{}{}
		} else {
			legacySuppressed[legacyKeyOf(b)] = struct{}{}
		}
	}

	usedLegacy := false
	out := make([]Finding, 0, len(findings))
	for _, f := range findings {
		if f.Fingerprint != "" {
			if _, skip := fpSuppressed[f.Fingerprint]; skip {
				continue
			}
		}
		if _, skip := legacySuppressed[legacyKeyOf(f)]; skip {
			usedLegacy = true
			continue
		}
		out = append(out, f)
	}
	return BaselineResult{Findings: out, UsedLegacyMatch: usedLegacy}
}

// LoadBaseline reads a baseline JSON file.
// Accepts a bare findings array (legacy) or the --format json wrapper
// {"summary":…,"findings":[…]}.
func LoadBaseline(path string) ([]Finding, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("baseline: %w", err)
	}
	var baseline []Finding
	if err := json.Unmarshal(data, &baseline); err == nil {
		if baseline == nil {
			baseline = []Finding{}
		}
		return baseline, nil
	}
	var report JSONReport
	if err2 := json.Unmarshal(data, &report); err2 != nil {
		return nil, fmt.Errorf("baseline: %w", err)
	}
	if report.Findings == nil {
		report.Findings = []Finding{}
	}
	return report.Findings, nil
}
