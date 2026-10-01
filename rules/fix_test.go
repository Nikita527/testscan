package rules

import (
	"testing"

	"github.com/Nikita527/testscan/scan"
)

func TestEveryRuleHasDefaultFix(t *testing.T) {
	for _, r := range All() {
		if scan.DefaultFix(r.ID()) == "" {
			t.Errorf("rule %q has no default fix in scan.RuleFix", r.ID())
		}
	}
}
