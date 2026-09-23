package rules

import (
	"fmt"

	"github.com/Nikita527/testscan/scan"
)

// Select фильтрует правила: only пустой → все из all; затем вычитает disable.
// Неизвестный ID или пересечение only∩disable → error.
func Select(all []scan.Rule, only, disable []string) ([]scan.Rule, error) {
	byID := make(map[string]scan.Rule, len(all))
	for _, r := range all {
		byID[r.ID()] = r
	}

	onlySet := make(map[string]struct{}, len(only))
	for _, id := range only {
		if _, ok := byID[id]; !ok {
			return nil, fmt.Errorf("unknown rule %q", id)
		}
		onlySet[id] = struct{}{}
	}

	disableSet := make(map[string]struct{}, len(disable))
	for _, id := range disable {
		if _, ok := byID[id]; !ok {
			return nil, fmt.Errorf("unknown rule %q", id)
		}
		if _, conflict := onlySet[id]; conflict {
			return nil, fmt.Errorf("rule %q is both enabled (--rule) and disabled (--disable)", id)
		}
		disableSet[id] = struct{}{}
	}

	var out []scan.Rule
	if len(only) == 0 {
		for _, r := range all {
			if _, off := disableSet[r.ID()]; off {
				continue
			}
			out = append(out, r)
		}
		if out == nil {
			out = []scan.Rule{}
		}
		return out, nil
	}

	// сохраняем порядок only
	seen := make(map[string]struct{}, len(only))
	for _, id := range only {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, byID[id])
	}
	if out == nil {
		out = []scan.Rule{}
	}
	return out, nil
}
