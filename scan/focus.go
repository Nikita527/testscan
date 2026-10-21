package scan

import "strings"

// neverFocusRules are demoted / mid-noise heuristics that must stay out of
// --focus even if config bumps their severity to warning.
var neverFocusRules = map[string]struct{}{
	"name-body-mismatch":                  {},
	"no-assert":                           {},
	"mock-only-assert":                    {},
	"mock-tautology":                      {},
	"overbroad-equality":                  {},
	"weak-assert":                         {},
	"only-happy-path":                     {},
	"commented-assert":                    {},
	"test-imports-implementation-private": {},
	// wall-clock is measured/provisional on mp-be but note-heavy and 0% on agent
	// corpora when used as input data — keep it out of the default view.
	"wall-clock-in-test": {},
}

// noteFocusAllowRules are note-severity rules that still pass the default focus
// filter (README: near-duplicate-test is provisional and shown by default).
// Other provisional notes (e.g. wall-clock before neverFocus) stay hidden.
var noteFocusAllowRules = map[string]struct{}{
	"near-duplicate-test": {},
}

// FilterFocus keeps high-signal findings for daily/CI use: error and warning
// with PrecisionWeight ≥ MinPrecisionForGrade. Drops notes (except
// noteFocusAllowRules), zero-precision rules, and neverFocusRules.
// parse-error (tool error) is always kept.
func FilterFocus(findings []Finding) []Finding {
	out := make([]Finding, 0, len(findings))
	for _, f := range findings {
		if f.Rule == "parse-error" {
			out = append(out, f)
			continue
		}
		if _, skip := neverFocusRules[f.Rule]; skip {
			continue
		}
		sev := strings.ToLower(f.Severity)
		if sev != "error" && sev != "warning" {
			if _, ok := noteFocusAllowRules[f.Rule]; !ok {
				continue
			}
		}
		if PrecisionWeight(f.Rule) == 0 {
			continue
		}
		out = append(out, f)
	}
	return out
}
