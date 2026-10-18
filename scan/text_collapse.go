package scan

import "fmt"

// CollapseNearDuplicatesForText collapses near-duplicate-test findings to one
// summary line per file for default text output. Other formats keep every finding.
func CollapseNearDuplicatesForText(findings []Finding) []Finding {
	counts := map[string]int{}
	first := map[string]Finding{}
	for _, f := range findings {
		if f.Rule != "near-duplicate-test" {
			continue
		}
		counts[f.File]++
		if _, ok := first[f.File]; !ok {
			first[f.File] = f
		}
	}
	if len(counts) == 0 {
		return findings
	}
	out := make([]Finding, 0, len(findings))
	emitted := map[string]bool{}
	for _, f := range findings {
		if f.Rule != "near-duplicate-test" {
			out = append(out, f)
			continue
		}
		if emitted[f.File] {
			continue
		}
		emitted[f.File] = true
		n := counts[f.File]
		base := first[f.File]
		msg := fmt.Sprintf(
			"%d near-duplicate test pair(s)/cluster(s) in this file — use --all to list each",
			n,
		)
		if n == 1 {
			// Keep the specific message for a single finding.
			out = append(out, f)
			continue
		}
		base.Message = msg
		if base.Fix == "" {
			base.Fix = DefaultFix("near-duplicate-test")
		}
		out = append(out, base)
	}
	return out
}
