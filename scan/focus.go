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
	"near-duplicate-test":                 {},
	"only-happy-path":                     {},
	"commented-assert":                    {},
	"test-imports-implementation-private": {},
}

// FilterFocus keeps high-signal findings for daily/CI use: error and warning
// with PrecisionWeight ≥ MinPrecisionForGrade. Drops notes, zero-precision rules,
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
			continue
		}
		if PrecisionWeight(f.Rule) == 0 {
			continue
		}
		out = append(out, f)
	}
	return out
}
