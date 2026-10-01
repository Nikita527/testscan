package scan

import (
	"math"
	"sort"
)

// WilsonZ95 is the z-score for a 95% confidence interval.
const WilsonZ95 = 1.959964

// WilsonLower returns the lower bound of the Wilson score interval for tp
// successes out of n trials at the confidence given by z. It returns 0 for n <= 0.
func WilsonLower(tp, n int, z float64) float64 {
	if n <= 0 {
		return 0
	}
	nf := float64(n)
	p := float64(tp) / nf
	z2 := z * z
	denom := 1 + z2/nf
	centre := p + z2/(2*nf)
	margin := z * math.Sqrt(p*(1-p)/nf+z2/(4*nf*nf))
	lo := (centre - margin) / denom
	if lo < 0 {
		return 0
	}
	return lo
}

// RuleStats is measured precision for one rule. N = TP + FP (skips excluded).
type RuleStats struct {
	Rule      string  `json:"rule"`
	N         int     `json:"n"`
	TP        int     `json:"tp"`
	FP        int     `json:"fp"`
	Skip      int     `json:"skip"`
	Precision float64 `json:"precision"`
	WilsonLow float64 `json:"wilson_lower"`
}

// ComputePrecision aggregates labels per rule, sorted by rule. When report is
// non-nil only labels whose fingerprint appears in report are counted, so the
// result measures the precision of what that run shows. A nil report uses all
// labels. Labels are deduplicated by fingerprint (last wins).
func ComputePrecision(labels []Label, report []Finding) []RuleStats {
	var inReport map[string]struct{}
	if report != nil {
		inReport = make(map[string]struct{}, len(report))
		for _, f := range report {
			if f.Fingerprint != "" {
				inReport[f.Fingerprint] = struct{}{}
			}
		}
	}
	seen := make(map[string]Label, len(labels))
	for _, l := range labels {
		seen[l.Fingerprint] = l
	}
	byRule := map[string]*RuleStats{}
	for _, l := range seen {
		if inReport != nil {
			if _, ok := inReport[l.Fingerprint]; !ok {
				continue
			}
		}
		rs := byRule[l.Rule]
		if rs == nil {
			rs = &RuleStats{Rule: l.Rule}
			byRule[l.Rule] = rs
		}
		switch l.Label {
		case LabelTP:
			rs.TP++
		case LabelFP:
			rs.FP++
		case LabelSkip:
			rs.Skip++
		}
	}
	out := make([]RuleStats, 0, len(byRule))
	for _, rs := range byRule {
		rs.N = rs.TP + rs.FP
		if rs.N > 0 {
			rs.Precision = float64(rs.TP) / float64(rs.N)
		}
		rs.WilsonLow = WilsonLower(rs.TP, rs.N, WilsonZ95)
		out = append(out, *rs)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rule < out[j].Rule })
	return out
}

// TotalStats sums per-rule stats into one row (Rule "TOTAL").
func TotalStats(stats []RuleStats) RuleStats {
	t := RuleStats{Rule: "TOTAL"}
	for _, s := range stats {
		t.TP += s.TP
		t.FP += s.FP
		t.Skip += s.Skip
	}
	t.N = t.TP + t.FP
	if t.N > 0 {
		t.Precision = float64(t.TP) / float64(t.N)
	}
	t.WilsonLow = WilsonLower(t.TP, t.N, WilsonZ95)
	return t
}
