package scan

import "strings"

// neverFocusRules are demoted / mid-noise heuristics that must stay out of
// --focus even if config bumps their severity to warning.
// near-duplicate-test is provisional and intentionally NOT listed here — see
// FilterFocus (note-severity provisional findings still show by default).
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
}

// FilterFocus keeps high-signal findings for daily/CI use: error and warning
// with PrecisionWeight ≥ MinPrecisionForGrade. Drops notes (except provisional
// tier — README promises provisional is shown by default), zero-precision rules,
// and neverFocusRules (severity overrides). parse-error (tool error) is always
// kept: a file that could not be analyzed must never silently disappear.
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
			// Provisional rules may emit note severity (near-duplicate-test) and
			// must still pass the default focus filter.
			if RuleTier(f.Rule) != TierProvisional {
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
