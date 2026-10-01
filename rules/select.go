package rules

import (
	"fmt"

	"github.com/Nikita527/testscan/scan"
)

// Select builds the active rule set.
//
//   - defaults — always-on rules (typically Default())
//   - all — Default ∪ Optional (lookup for --rule / --enable / --disable)
//   - only (--rule) — whitelist from all; when set, enable is ignored for base membership
//   - enable — add Optional (or any all) rules onto the default base
//   - disable — subtract from the result
//
// Unknown IDs and only∩disable / enable∩disable overlaps return an error.
func Select(defaults, all []scan.Rule, only, enable, disable []string) ([]scan.Rule, error) {
	byID := make(map[string]scan.Rule, len(all))
	for _, r := range all {
		byID[r.ID()] = r
	}
	// Defaults must also be resolvable (in case all is incomplete).
	for _, r := range defaults {
		if _, ok := byID[r.ID()]; !ok {
			byID[r.ID()] = r
		}
	}

	onlySet := make(map[string]struct{}, len(only))
	for _, id := range only {
		if _, ok := byID[id]; !ok {
			return nil, fmt.Errorf("unknown rule %q", id)
		}
		onlySet[id] = struct{}{}
	}

	enableSet := make(map[string]struct{}, len(enable))
	for _, id := range enable {
		if _, ok := byID[id]; !ok {
			return nil, fmt.Errorf("unknown rule %q", id)
		}
		if _, conflict := onlySet[id]; conflict {
			return nil, fmt.Errorf("rule %q is both enabled (--rule) and listed in --enable", id)
		}
		enableSet[id] = struct{}{}
	}

	disableSet := make(map[string]struct{}, len(disable))
	for _, id := range disable {
		if _, ok := byID[id]; !ok {
			return nil, fmt.Errorf("unknown rule %q", id)
		}
		if _, conflict := onlySet[id]; conflict {
			return nil, fmt.Errorf("rule %q is both enabled (--rule) and disabled (--disable)", id)
		}
		if _, conflict := enableSet[id]; conflict {
			return nil, fmt.Errorf("rule %q is both enabled (--enable) and disabled (--disable)", id)
		}
		disableSet[id] = struct{}{}
	}

	if len(only) > 0 {
		seen := make(map[string]struct{}, len(only))
		var out []scan.Rule
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

	seen := make(map[string]struct{}, len(defaults)+len(enable))
	var out []scan.Rule
	for _, r := range defaults {
		id := r.ID()
		if _, off := disableSet[id]; off {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, r)
	}
	for _, id := range enable {
		if _, ok := seen[id]; ok {
			continue
		}
		if _, off := disableSet[id]; off {
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
