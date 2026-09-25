package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config holds project settings from .testscan.toml or [tool.testscan].
type Config struct {
	FailOn  string // error|warning|never; empty = unset
	Disable []string
	Paths   []string
	Workers int    // 0 = unset (NumCPU in scan)
	Source  string // config file path loaded from; empty if none

	Exclude          []string
	AssertHelpers    []string
	PythonFiles      []string
	PythonFunctions  []string
	PythonClasses    []string
	RespectGitignore bool // default true
	Rules            map[string]RuleConfig
	Overrides        []Override
}

// RuleConfig holds per-rule overrides from [rules.<id>].
type RuleConfig struct {
	Severity string
	MinTests int // only-happy-path; 0 = default
	// Mode — only-happy-path: ""|"heuristic"|"coverage". Empty = heuristic.
	Mode string
	// Coverage — path to coverage.py JSON report (only-happy-path coverage mode).
	// Relative paths resolve against the config file directory.
	Coverage string
	// NegativeNames — substrings in test names / parametrize ids that count as
	// negative-path signals (only-happy-path). Empty = built-in defaults.
	NegativeNames []string
}

// Override is [[overrides]] path glob → disable rules.
type Override struct {
	Path    string
	Disable []string
}

type fileTOML struct {
	FailOnKebab      string              `toml:"fail-on"`
	FailOnSnake      string              `toml:"fail_on"`
	Disable          []string            `toml:"disable"`
	Paths            []string            `toml:"paths"`
	Workers          int                 `toml:"workers"`
	Exclude          []string            `toml:"exclude"`
	AssertHelpers    []string            `toml:"assert-helpers"`
	PythonFiles      []string            `toml:"python-files"`
	PythonFunctions  []string            `toml:"python-functions"`
	PythonClasses    []string            `toml:"python-classes"`
	RespectGitignore *bool               `toml:"respect-gitignore"`
	Rules            map[string]ruleTOML `toml:"rules"`
	Overrides        []overrideTOML      `toml:"overrides"`
}

type ruleTOML struct {
	Severity      string   `toml:"severity"`
	MinTests      int      `toml:"min-tests"`
	Mode          string   `toml:"mode"`
	Coverage      string   `toml:"coverage"`
	NegativeNames []string `toml:"negative-names"`
}

type overrideTOML struct {
	Path    string   `toml:"path"`
	Disable []string `toml:"disable"`
}

type pyprojectTOML struct {
	Tool struct {
		Testscan fileTOML             `toml:"testscan"`
		Pytest   pytestIniOptionsTOML `toml:"pytest"`
	} `toml:"tool"`
}

type pytestIniOptionsTOML struct {
	IniOptions pytestIniTOML `toml:"ini_options"`
}

type pytestIniTOML struct {
	PythonFiles     any `toml:"python_files"`
	PythonFunctions any `toml:"python_functions"`
	PythonClasses   any `toml:"python_classes"`
}

// Load walks parents from startDir: prefer .testscan.toml, else pyproject.toml
// with [tool.testscan]. Missing config returns an empty Config (RespectGitignore=true).
func Load(startDir string) (Config, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return Config{}, err
	}
	for {
		dot := filepath.Join(dir, ".testscan.toml")
		if st, err := os.Stat(dot); err == nil && !st.IsDir() {
			cfg, err := parseDotFile(dot)
			if err != nil {
				return Config{}, err
			}
			return cfg, nil
		}

		py := filepath.Join(dir, "pyproject.toml")
		if st, err := os.Stat(py); err == nil && !st.IsDir() {
			cfg, found, err := parsePyproject(py)
			if err != nil {
				return Config{}, err
			}
			if found {
				return cfg, nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return Config{RespectGitignore: true, Rules: map[string]RuleConfig{}}, nil
		}
		dir = parent
	}
}

func parseDotFile(path string) (Config, error) {
	var raw fileTOML
	if _, err := toml.DecodeFile(path, &raw); err != nil {
		return Config{}, fmt.Errorf("config %s: %w", path, err)
	}
	return fromTOML(raw, path, nil)
}

func parsePyproject(path string) (Config, bool, error) {
	var raw pyprojectTOML
	if _, err := toml.DecodeFile(path, &raw); err != nil {
		return Config{}, false, fmt.Errorf("config %s: %w", path, err)
	}
	t := raw.Tool.Testscan
	pytest := raw.Tool.Pytest.IniOptions
	if !testscanSectionPresent(t) {
		return Config{}, false, nil
	}
	cfg, err := fromTOML(t, path, &pytest)
	if err != nil {
		return Config{}, false, err
	}
	return cfg, true, nil
}

func testscanSectionPresent(t fileTOML) bool {
	if t.FailOnKebab != "" || t.FailOnSnake != "" || t.Workers != 0 {
		return true
	}
	if len(t.Disable) > 0 || len(t.Paths) > 0 || len(t.Exclude) > 0 {
		return true
	}
	if len(t.AssertHelpers) > 0 || len(t.PythonFiles) > 0 ||
		len(t.PythonFunctions) > 0 || len(t.PythonClasses) > 0 {
		return true
	}
	if t.RespectGitignore != nil || len(t.Rules) > 0 || len(t.Overrides) > 0 {
		return true
	}
	return false
}

func fromTOML(raw fileTOML, source string, pytest *pytestIniTOML) (Config, error) {
	failOn := raw.FailOnKebab
	if failOn == "" {
		failOn = raw.FailOnSnake
	}
	if failOn != "" && failOn != "error" && failOn != "warning" && failOn != "never" {
		return Config{}, fmt.Errorf("config %s: invalid fail-on %q (want error|warning|never)", source, failOn)
	}
	if raw.Workers < 0 {
		return Config{}, fmt.Errorf("config %s: workers must be >= 0", source)
	}

	rules := map[string]RuleConfig{}
	base := filepath.Dir(source)
	for id, r := range raw.Rules {
		if r.Severity != "" && r.Severity != "error" && r.Severity != "warning" && r.Severity != "note" {
			return Config{}, fmt.Errorf("config %s: rules.%s: invalid severity %q (want error|warning|note)", source, id, r.Severity)
		}
		if r.MinTests < 0 {
			return Config{}, fmt.Errorf("config %s: rules.%s: min-tests must be >= 0", source, id)
		}
		mode := r.Mode
		if mode != "" && mode != "heuristic" && mode != "coverage" {
			return Config{}, fmt.Errorf("config %s: rules.%s: invalid mode %q (want heuristic|coverage)", source, id, mode)
		}
		neg := r.NegativeNames
		if neg == nil {
			neg = []string{}
		}
		cov := r.Coverage
		if cov != "" && !filepath.IsAbs(cov) {
			cov = filepath.Clean(filepath.Join(base, cov))
		}
		rules[id] = RuleConfig{
			Severity:      r.Severity,
			MinTests:      r.MinTests,
			Mode:          mode,
			Coverage:      cov,
			NegativeNames: neg,
		}
	}

	overrides := make([]Override, 0, len(raw.Overrides))
	for _, o := range raw.Overrides {
		dis := o.Disable
		if dis == nil {
			dis = []string{}
		}
		overrides = append(overrides, Override{Path: o.Path, Disable: dis})
	}

	respect := true
	if raw.RespectGitignore != nil {
		respect = *raw.RespectGitignore
	}

	pythonFiles := raw.PythonFiles
	pythonFunctions := raw.PythonFunctions
	pythonClasses := raw.PythonClasses
	if pytest != nil {
		if len(pythonFiles) == 0 {
			pythonFiles = anyToStrings(pytest.PythonFiles)
		}
		if len(pythonFunctions) == 0 {
			pythonFunctions = anyToStrings(pytest.PythonFunctions)
		}
		if len(pythonClasses) == 0 {
			pythonClasses = anyToStrings(pytest.PythonClasses)
		}
	}

	disable := raw.Disable
	if disable == nil {
		disable = []string{}
	}
	paths := raw.Paths
	if paths == nil {
		paths = []string{}
	}
	exclude := raw.Exclude
	if exclude == nil {
		exclude = []string{}
	}
	helpers := raw.AssertHelpers
	if helpers == nil {
		helpers = []string{}
	}
	if pythonFiles == nil {
		pythonFiles = []string{}
	}
	if pythonFunctions == nil {
		pythonFunctions = []string{}
	}
	if pythonClasses == nil {
		pythonClasses = []string{}
	}

	// config paths are relative to the config file directory, not cwd
	resolved := make([]string, len(paths))
	for i, p := range paths {
		if p == "" || filepath.IsAbs(p) {
			resolved[i] = p
			continue
		}
		resolved[i] = filepath.Clean(filepath.Join(base, p))
	}
	return Config{
		FailOn:           failOn,
		Disable:          disable,
		Paths:            resolved,
		Workers:          raw.Workers,
		Source:           source,
		Exclude:          exclude,
		AssertHelpers:    helpers,
		PythonFiles:      pythonFiles,
		PythonFunctions:  pythonFunctions,
		PythonClasses:    pythonClasses,
		RespectGitignore: respect,
		Rules:            rules,
		Overrides:        overrides,
	}, nil
}

func anyToStrings(v any) []string {
	switch x := v.(type) {
	case string:
		if x == "" {
			return nil
		}
		return []string{x}
	case []any:
		out := make([]string, 0, len(x))
		for _, e := range x {
			if s, ok := e.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return x
	default:
		return nil
	}
}
