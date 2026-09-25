package rules

import (
	"github.com/Nikita527/testscan/internal/config"
	"github.com/Nikita527/testscan/scan"
)

// ApplyConfig adjusts Default-selected rules from project config
// (e.g. only-happy-path min-tests / mode / coverage). Severity is applied later in scan.Run.
func ApplyConfig(selected []scan.Rule, cfg config.Config) []scan.Rule {
	if len(selected) == 0 {
		return selected
	}
	rc, ok := cfg.Rules["only-happy-path"]
	if !ok {
		return selected
	}
	if rc.MinTests <= 0 && rc.Mode == "" && rc.Coverage == "" && len(rc.NegativeNames) == 0 {
		return selected
	}
	out := make([]scan.Rule, len(selected))
	copy(out, selected)
	for i, r := range out {
		if r.ID() == "only-happy-path" {
			out[i] = NewOnlyHappyPathOpts(OnlyHappyPathOpts{
				MinTests:      rc.MinTests,
				Mode:          rc.Mode,
				CoveragePath:  rc.Coverage,
				NegativeNames: rc.NegativeNames,
			})
		}
	}
	return out
}

// EnableOnlyHappyPathCoverage forces coverage mode and sets the report path
// (CLI --coverage). No-op if only-happy-path is not selected.
func EnableOnlyHappyPathCoverage(selected []scan.Rule, coveragePath string) []scan.Rule {
	if coveragePath == "" || len(selected) == 0 {
		return selected
	}
	out := make([]scan.Rule, len(selected))
	copy(out, selected)
	for i, r := range out {
		if r.ID() != "only-happy-path" {
			continue
		}
		opts := OnlyHappyPathOpts{Mode: "coverage", CoveragePath: coveragePath}
		if oh, ok := r.(onlyHappyPath); ok {
			opts.MinTests = oh.minTests
			opts.NegativeNames = append([]string{}, oh.negativeNames...)
			if opts.CoveragePath == "" {
				opts.CoveragePath = oh.coveragePath
			}
		}
		out[i] = NewOnlyHappyPathOpts(opts)
	}
	return out
}

// CoveragePathFromConfig returns the resolved only-happy-path coverage path, if any.
func CoveragePathFromConfig(cfg config.Config) string {
	if rc, ok := cfg.Rules["only-happy-path"]; ok {
		return rc.Coverage
	}
	return ""
}

// SeverityMap extracts rule→severity overrides from config.
func SeverityMap(cfg config.Config) map[string]string {
	if len(cfg.Rules) == 0 {
		return nil
	}
	out := make(map[string]string, len(cfg.Rules))
	for id, r := range cfg.Rules {
		if r.Severity != "" {
			out[id] = r.Severity
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// PathOverrides converts config overrides to scan.PathOverride.
func PathOverrides(cfg config.Config) []scan.PathOverride {
	if len(cfg.Overrides) == 0 {
		return nil
	}
	out := make([]scan.PathOverride, len(cfg.Overrides))
	for i, o := range cfg.Overrides {
		out[i] = scan.PathOverride{Path: o.Path, Disable: o.Disable}
	}
	return out
}
