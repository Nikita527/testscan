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

func keyOf(f Finding) baselineKey {
	return baselineKey{
		file: filepath.ToSlash(filepath.Clean(f.File)),
		line: f.Line,
		rule: f.Rule,
	}
}

// FilterBaseline убирает findings, совпадающие с baseline по file+line+rule
// (message игнорируется). Пустой/nil baseline — no-op.
func FilterBaseline(findings, baseline []Finding) []Finding {
	if len(baseline) == 0 {
		return findings
	}
	suppressed := make(map[baselineKey]struct{}, len(baseline))
	for _, b := range baseline {
		suppressed[keyOf(b)] = struct{}{}
	}
	out := make([]Finding, 0, len(findings))
	for _, f := range findings {
		if _, skip := suppressed[keyOf(f)]; skip {
			continue
		}
		out = append(out, f)
	}
	return out
}

// LoadBaseline читает JSON-массив Finding (как --format json).
func LoadBaseline(path string) ([]Finding, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("baseline: %w", err)
	}
	var baseline []Finding
	if err := json.Unmarshal(data, &baseline); err != nil {
		return nil, fmt.Errorf("baseline: %w", err)
	}
	if baseline == nil {
		baseline = []Finding{}
	}
	return baseline, nil
}
