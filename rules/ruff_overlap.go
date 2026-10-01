package rules

import (
	"github.com/Nikita527/testscan/internal/config"
	"github.com/Nikita527/testscan/scan"
)

// ruffOverlap maps a testscan rule to the ruff code that covers it.
var ruffOverlap = map[string]string{
	"broad-raises": "PT011",
}

// DeferToRuff drops rules already covered by the project's ruff configuration
// (see docs/rules.md "Overlap with ruff"). It is a no-op when cfg.DeferToRuff is
// false. Rules named in explicit are kept (the user asked for them).
func DeferToRuff(selected []scan.Rule, cfg config.Config, root string, explicit []string) []scan.Rule {
	if !cfg.DeferToRuff {
		return selected
	}
	keep := map[string]bool{}
	for _, id := range explicit {
		keep[id] = true
	}
	out := make([]scan.Rule, 0, len(selected))
	for _, r := range selected {
		if code, ok := ruffOverlap[r.ID()]; ok && !keep[r.ID()] && config.RuffEnables(root, code) {
			continue
		}
		out = append(out, r)
	}
	return out
}
