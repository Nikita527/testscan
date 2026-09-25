package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/Nikita527/testscan/internal/config"
	"github.com/Nikita527/testscan/rules"
	"github.com/Nikita527/testscan/scan"
)

const usage = "usage: testscan [path...] [--format text|json|sarif|html] [--fail-on error|warning|never] [--rule ID] [--disable ID] [--baseline path.json] [--coverage path.json] [--workers N] [-o|--output PATH] [--open]"

var errHelp = errors.New("help")

type cliArgs struct {
	roots      []string
	format     string
	failOn     string
	failOnSet  bool
	only       []string
	disable    []string
	baseline   string
	coverage   string
	workers    int
	workersSet bool
	output     string
	open       bool
}

func main() {
	args, err := parseArgs(os.Args[1:])
	if errors.Is(err, errHelp) {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(0)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "testscan: %v\n", err)
		os.Exit(2)
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "testscan: %v\n", err)
		os.Exit(2)
	}
	cfg, err := config.Load(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "testscan: %v\n", err)
		os.Exit(2)
	}
	args = applyConfig(args, cfg)

	selected, err := rules.Select(rules.Default(), args.only, args.disable)
	if err != nil {
		fmt.Fprintf(os.Stderr, "testscan: %v\n", err)
		os.Exit(2)
	}
	selected = rules.ApplyConfig(selected, cfg)
	coveragePath := args.coverage
	if coveragePath == "" {
		coveragePath = rules.CoveragePathFromConfig(cfg)
	}
	if args.coverage != "" {
		selected = rules.EnableOnlyHappyPathCoverage(selected, args.coverage)
	}

	result, err := scan.Run(context.Background(), args.roots, scan.Options{
		Rules:            selected,
		Workers:          args.workers,
		Exclude:          cfg.Exclude,
		PythonFiles:      cfg.PythonFiles,
		RespectGitignore: cfg.RespectGitignore,
		PathRoot:         cwd,
		AssertHelpers:    cfg.AssertHelpers,
		RuleSeverity:     rules.SeverityMap(cfg),
		Overrides:        rules.PathOverrides(cfg),
		PythonFunctions:  cfg.PythonFunctions,
		PythonClasses:    cfg.PythonClasses,
		CoveragePath:     coveragePath,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "testscan: %v\n", err)
		os.Exit(2)
	}
	findings := result.Findings

	if args.baseline != "" {
		baseline, err := scan.LoadBaseline(args.baseline)
		if err != nil {
			fmt.Fprintf(os.Stderr, "testscan: %v\n", err)
			os.Exit(2)
		}
		res := scan.FilterBaseline(findings, baseline)
		findings = res.Findings
		if res.UsedLegacyMatch {
			fmt.Fprintln(os.Stderr, "testscan: warning: baseline matched using legacy file+line+rule keys; re-save baseline to migrate to fingerprints")
		}
	}

	if err := emitFindings(findings, result.Files, args.format, args.output, args.open); err != nil {
		fmt.Fprintf(os.Stderr, "testscan: %v\n", err)
		os.Exit(2)
	}

	os.Exit(exitCode(findings, args.failOn))
}

// applyConfig: CLI overrides config; disable = config ∪ CLI.
func applyConfig(args cliArgs, cfg config.Config) cliArgs {
	if !args.failOnSet && cfg.FailOn != "" {
		args.failOn = cfg.FailOn
	}
	if !args.workersSet && cfg.Workers > 0 {
		args.workers = cfg.Workers
	}
	if len(cfg.Disable) > 0 {
		args.disable = append(append([]string{}, cfg.Disable...), args.disable...)
	}
	if len(args.roots) == 0 {
		if len(cfg.Paths) > 0 {
			args.roots = append([]string{}, cfg.Paths...)
		} else {
			args.roots = []string{"."}
		}
	}
	return args
}

// parseArgs allows flags before and after paths (per SPEC: testscan path --format json).
func parseArgs(argv []string) (cliArgs, error) {
	out := cliArgs{
		format: "text",
		failOn: "error",
	}

	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			return cliArgs{}, errHelp
		case a == "--format":
			i++
			if i >= len(argv) {
				return cliArgs{}, fmt.Errorf("missing value for --format")
			}
			out.format = argv[i]
		case strings.HasPrefix(a, "--format="):
			out.format = strings.TrimPrefix(a, "--format=")
		case a == "--fail-on":
			i++
			if i >= len(argv) {
				return cliArgs{}, fmt.Errorf("missing value for --fail-on")
			}
			out.failOn = argv[i]
			out.failOnSet = true
		case strings.HasPrefix(a, "--fail-on="):
			out.failOn = strings.TrimPrefix(a, "--fail-on=")
			out.failOnSet = true
		case a == "--rule":
			i++
			if i >= len(argv) {
				return cliArgs{}, fmt.Errorf("missing value for --rule")
			}
			out.only = append(out.only, argv[i])
		case strings.HasPrefix(a, "--rule="):
			out.only = append(out.only, strings.TrimPrefix(a, "--rule="))
		case a == "--disable":
			i++
			if i >= len(argv) {
				return cliArgs{}, fmt.Errorf("missing value for --disable")
			}
			out.disable = append(out.disable, argv[i])
		case strings.HasPrefix(a, "--disable="):
			out.disable = append(out.disable, strings.TrimPrefix(a, "--disable="))
		case a == "--baseline":
			i++
			if i >= len(argv) {
				return cliArgs{}, fmt.Errorf("missing value for --baseline")
			}
			out.baseline = argv[i]
		case strings.HasPrefix(a, "--baseline="):
			out.baseline = strings.TrimPrefix(a, "--baseline=")
		case a == "--coverage":
			i++
			if i >= len(argv) {
				return cliArgs{}, fmt.Errorf("missing value for --coverage")
			}
			out.coverage = argv[i]
		case strings.HasPrefix(a, "--coverage="):
			out.coverage = strings.TrimPrefix(a, "--coverage=")
		case a == "--workers":
			i++
			if i >= len(argv) {
				return cliArgs{}, fmt.Errorf("missing value for --workers")
			}
			n, err := strconv.Atoi(argv[i])
			if err != nil || n < 0 {
				return cliArgs{}, fmt.Errorf("invalid --workers %q (want integer >= 0)", argv[i])
			}
			out.workers = n
			out.workersSet = true
		case strings.HasPrefix(a, "--workers="):
			v := strings.TrimPrefix(a, "--workers=")
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return cliArgs{}, fmt.Errorf("invalid --workers %q (want integer >= 0)", v)
			}
			out.workers = n
			out.workersSet = true
		case a == "-o" || a == "--output":
			i++
			if i >= len(argv) {
				return cliArgs{}, fmt.Errorf("missing value for %s", a)
			}
			out.output = argv[i]
		case strings.HasPrefix(a, "--output="):
			out.output = strings.TrimPrefix(a, "--output=")
		case a == "--open":
			out.open = true
		case strings.HasPrefix(a, "-"):
			return cliArgs{}, fmt.Errorf("unknown flag %s", a)
		default:
			out.roots = append(out.roots, a)
		}
	}

	if out.format != "text" && out.format != "json" && out.format != "sarif" && out.format != "html" {
		return cliArgs{}, fmt.Errorf("invalid --format %q (want text|json|sarif|html)", out.format)
	}
	if out.failOn != "error" && out.failOn != "warning" && out.failOn != "never" {
		return cliArgs{}, fmt.Errorf("invalid --fail-on %q (want error|warning|never)", out.failOn)
	}

	return out, nil
}

func writeFindings(w io.Writer, findings []scan.Finding, fileCount int, format string) error {
	score := scan.CalculateScore(findings, fileCount)
	switch format {
	case "json":
		return scan.WriteJSON(w, findings, score)
	case "sarif":
		return scan.WriteSARIF(w, findings)
	case "html":
		return scan.WriteHTML(w, findings, score)
	default:
		for _, f := range findings {
			if _, err := fmt.Fprintf(w, "%s:%d: %s %s: %s\n",
				f.File, f.Line, f.Severity, f.Rule, f.Message); err != nil {
				return err
			}
		}
		_, err := fmt.Fprintln(w, scan.FormatScoreLine(score))
		return err
	}
}

func exitCode(findings []scan.Finding, failOn string) int {
	if failOn == "never" {
		return 0
	}
	for _, f := range findings {
		switch failOn {
		case "warning":
			if f.Severity == "warning" || f.Severity == "error" {
				return 1
			}
		case "error":
			if f.Severity == "error" {
				return 1
			}
		}
	}
	return 0
}
