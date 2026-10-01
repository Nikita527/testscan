package scan

import (
	"encoding/json"
	"fmt"
	"os"
)

// Label values.
const (
	LabelTP   = "tp"
	LabelFP   = "fp"
	LabelSkip = "skip"
)

// Label is one human verdict on a finding, keyed by Finding.Fingerprint.
type Label struct {
	Fingerprint string `json:"fingerprint"`
	Rule        string `json:"rule"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Label       string `json:"label"` // "tp" | "fp" | "skip"
	Note        string `json:"note,omitempty"`
}

// LoadLabels reads a labels file (.testscan/labels.json): a JSON array of Label.
// Every entry needs a fingerprint and a label of tp, fp or skip.
// Duplicate fingerprints are allowed: the LAST entry wins (position of the
// first occurrence is kept), so re-labelling by appending works.
func LoadLabels(path string) ([]Label, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("labels: %w", err)
	}
	var raw []Label
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("labels: %s: %w", path, err)
	}
	idx := make(map[string]int, len(raw))
	out := make([]Label, 0, len(raw))
	for i, l := range raw {
		if l.Fingerprint == "" {
			return nil, fmt.Errorf("labels: %s: entry %d: missing fingerprint", path, i)
		}
		switch l.Label {
		case LabelTP, LabelFP, LabelSkip:
		default:
			return nil, fmt.Errorf("labels: %s: entry %d: invalid label %q (want tp|fp|skip)", path, i, l.Label)
		}
		if j, ok := idx[l.Fingerprint]; ok {
			out[j] = l
			continue
		}
		idx[l.Fingerprint] = len(out)
		out = append(out, l)
	}
	return out, nil
}
