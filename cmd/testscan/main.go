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

const usage = "usage: testscan [path...] [--format text|json|sarif|html|codequality|agent] [--fail-on error|warning|never] [--rule ID] [--enable ID] [--disable ID] [--baseline path.json] [--compare path.json] [--coverage path.json] [--diff base-ref] [--all] [--show-low-precision] [--workers N] [-o|--output PATH] [--open]\n       testscan precision --labels .testscan/labels.json [--report out.json] [--format text|json]"

var errHelp = errors.New("help")

type cliArgs struct {
	roots               []string
	format              string
	failOn              string
	failOnSet           bool
	only                []string
	enable              []string
	disable             []string
	baseline            string
	compare             string
	showGrade           bool // deprecated no-op
	coverage            string
	diff                string
	focus               bool // deprecated no-op (focus is the default)
	all                 bool
	showLowPrecision    bool
	showLowPrecisionSet bool
	workers             int
	workersSet          bool
	output              string
	open                bool
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "precision" {
		os.Exit(runPrecision(os.Args[2:], os.Stdout, os.Stderr))
	}
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
	if args.showGrade {
		fmt.Fprintln(os.Stderr, "testscan: warning: --show-grade is deprecated and has no effect (Health Score/grade was removed from text and HTML output; JSON still carries it)")
	}
	if args.focus {
		fmt.Fprintln(os.Stderr, "testscan: warning: --focus is deprecated and has no effect (focus is now the default; use --all for the full output)")
	}

	selected, err := rules.Select(rules.Default(), rules.All(), args.only, args.enable, args.disable)
	if err != nil {
		fmt.Fprintf(os.Stderr, "testscan: %v\n", err)
		os.Exit(2)
	}
	selected = rules.ApplyConfig(selected, cfg)
	selected = rules.DeferToRuff(selected, cfg, cwd, append(append([]string{}, args.only...), args.enable...))
	coveragePath := args.coverage
	if coveragePath == "" {
		coveragePath = rules.CoveragePathFromConfig(cfg)
	}
	if args.coverage != "" {
		selected = rules.EnableOnlyHappyPathCoverage(selected, args.coverage)
	}

	var onlyPaths []string
	if args.diff != "" {
		changed, err := scan.DiffChangedPaths(args.diff, cwd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "testscan: --diff: %v\n", err)
			os.Exit(2)
		}
		onlyPaths = scan.FilterTestPaths(changed, cfg.PythonFiles)
		if len(onlyPaths) == 0 {
			fmt.Fprintf(os.Stderr, "testscan: --diff %s: no changed test files\n", args.diff)
			os.Exit(0)
		}
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
		OnlyPaths:        onlyPaths,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "testscan: %v\n", err)
		os.Exit(2)
	}
	findings := result.Findings

	// Trend compare point: --compare, else --baseline.
	// When --baseline is set, trend "current" uses pre-baseline metrics (after
	// focus / low-precision) so hiding known noise cannot fake an "improved" trend
	// against a full previous report — including when --compare and --baseline differ.
	// summary.confirmed_* still reflect post-baseline (new issues only).
	comparePath := args.compare
	if comparePath == "" {
		comparePath = args.baseline
	}
	var preBaseline []scan.Finding
	if args.baseline != "" && comparePath != "" {
		preBaseline = append([]scan.Finding(nil), findings...)
	}

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

	// Display filters: default = focus + actionable/provisional tiers;
	// --show-low-precision adds low tiers; --all disables every filter.
	// Exit code, SARIF, codequality and the score all use this shown set.
	display := displayOptions(args)
	hidden := scan.HiddenByDisplay(findings, display)
	findings = scan.FilterForDisplay(findings, display)
	if msg := hiddenGateWarning(hidden, args.failOn); msg != "" {
		fmt.Fprintln(os.Stderr, msg)
	}

	score := scan.CalculateScoreWithTests(findings, result.Files, result.Tests)
	score.HiddenCount = len(hidden)
	if comparePath != "" {
		prev, err := scan.LoadComparePoint(comparePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "testscan: %v\n", err)
			os.Exit(2)
		}
		currCount, currDens := scan.TrendCurrentMetrics(
			score, preBaseline, display, result.Tests,
		)
		trend := scan.ComputeTrend(currCount, currDens, prev.ActionableCount, prev.ActionableDensity)
		trend.CurrentPreBaseline = preBaseline != nil
		score.Trend = &trend
	}
	score.ShowAll = args.all
	score.ShowLow = args.showLowPrecision
	score.PathRoot = cwd

	if err := emitFindings(findings, score, args.format, args.output, args.open); err != nil {
		fmt.Fprintf(os.Stderr, "testscan: %v\n", err)
		os.Exit(2)
	}

	os.Exit(exitCode(findings, args.failOn))
}

// displayOptions maps CLI/config flags to the display filter.
// Rules requested explicitly (--rule / --enable, CLI or config) bypass the focus
// and tier filters.
func displayOptions(args cliArgs) scan.DisplayOptions {
	return scan.DisplayOptions{
		All:              args.all,
		ShowLowPrecision: args.showLowPrecision,
		ExplicitRules:    scan.ExplicitSet(args.only, args.enable),
	}
}

// hiddenGateWarning returns the stderr line for hidden findings that would have
// triggered --fail-on, or "" when there are none.
func hiddenGateWarning(hidden []scan.Finding, failOn string) string {
	if failOn == "never" {
		return ""
	}
	n := 0
	for _, f := range hidden {
		if triggersFailOn(f, failOn) {
			n++
		}
	}
	if n == 0 {
		return ""
	}
	noun := "findings"
	verb := "are"
	if n == 1 {
		noun, verb = "finding", "is"
	}
	return fmt.Sprintf("testscan: %d %s at or above --fail-on %s %s hidden by the default view (unmeasured precision); use --all to gate on them", n, noun, failOn, verb)
}

// applyConfig: CLI overrides config; disable/enable = config ∪ CLI.
func applyConfig(args cliArgs, cfg config.Config) cliArgs {
	if !args.failOnSet && cfg.FailOn != "" {
		args.failOn = cfg.FailOn
	}
	if !args.workersSet && cfg.Workers > 0 {
		args.workers = cfg.Workers
	}
	if !args.showLowPrecisionSet && cfg.ShowLowPrecision {
		args.showLowPrecision = true
	}
	if cfg.All {
		args.all = true
	}
	if len(cfg.Disable) > 0 {
		args.disable = append(append([]string{}, cfg.Disable...), args.disable...)
	}
	if len(cfg.Enable) > 0 {
		args.enable = append(append([]string{}, cfg.Enable...), args.enable...)
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
		case a == "--enable":
			i++
			if i >= len(argv) {
				return cliArgs{}, fmt.Errorf("missing value for --enable")
			}
			out.enable = append(out.enable, argv[i])
		case strings.HasPrefix(a, "--enable="):
			out.enable = append(out.enable, strings.TrimPrefix(a, "--enable="))
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
		case a == "--compare":
			i++
			if i >= len(argv) {
				return cliArgs{}, fmt.Errorf("missing value for --compare")
			}
			out.compare = argv[i]
		case strings.HasPrefix(a, "--compare="):
			out.compare = strings.TrimPrefix(a, "--compare=")
		case a == "--show-grade":
			out.showGrade = true
		case a == "--coverage":
			i++
			if i >= len(argv) {
				return cliArgs{}, fmt.Errorf("missing value for --coverage")
			}
			out.coverage = argv[i]
		case strings.HasPrefix(a, "--coverage="):
			out.coverage = strings.TrimPrefix(a, "--coverage=")
		case a == "--diff":
			i++
			if i >= len(argv) {
				return cliArgs{}, fmt.Errorf("missing value for --diff")
			}
			out.diff = argv[i]
		case strings.HasPrefix(a, "--diff="):
			out.diff = strings.TrimPrefix(a, "--diff=")
		case a == "--focus":
			out.focus = true
		case a == "--all":
			out.all = true
		case a == "--show-low-precision":
			out.showLowPrecision = true
			out.showLowPrecisionSet = true
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

	if out.format != "text" && out.format != "json" && out.format != "sarif" && out.format != "html" && out.format != "codequality" && out.format != "agent" {
		return cliArgs{}, fmt.Errorf("invalid --format %q (want text|json|sarif|html|codequality|agent)", out.format)
	}
	if out.failOn != "error" && out.failOn != "warning" && out.failOn != "never" {
		return cliArgs{}, fmt.Errorf("invalid --fail-on %q (want error|warning|never)", out.failOn)
	}

	return out, nil
}

func writeFindings(w io.Writer, findings []scan.Finding, score scan.Score, format string) error {
	switch format {
	case "json":
		return scan.WriteJSON(w, findings, score)
	case "sarif":
		return scan.WriteSARIF(w, findings)
	case "html":
		return scan.WriteHTML(w, findings, score)
	case "codequality":
		return scan.WriteCodeQuality(w, findings)
	case "agent":
		return scan.WriteAgent(w, findings, score)
	default:
		if _, err := fmt.Fprintln(w, scan.FormatScoreLine(score)); err != nil {
			return err
		}
		for _, f := range findings {
			tag := ""
			if f.Rule != "parse-error" && scan.RuleTier(f.Rule) == scan.TierProvisional {
				tag = " [provisional]"
			}
			if _, err := fmt.Fprintf(w, "%s:%d: %s %s: %s%s\n",
				f.File, f.Line, f.Severity, f.Rule, f.Message, tag); err != nil {
				return err
			}
			if f.Fix != "" {
				if _, err := fmt.Fprintf(w, "  → fix: %s\n", f.Fix); err != nil {
					return err
				}
			}
		}
		return nil
	}
}

func exitCode(findings []scan.Finding, failOn string) int {
	if failOn == "never" {
		return 0
	}
	for _, f := range findings {
		if triggersFailOn(f, failOn) {
			return 1
		}
	}
	return 0
}

func triggersFailOn(f scan.Finding, failOn string) bool {
	switch failOn {
	case "warning":
		return f.Severity == "warning" || f.Severity == "error"
	case "error":
		return f.Severity == "error"
	}
	return false
}
