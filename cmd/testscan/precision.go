package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Nikita527/testscan/scan"
)

const precisionUsage = "usage: testscan precision --labels .testscan/labels.json [--report out.json] [--format text|json]"

type precisionArgs struct {
	labels string
	report string
	format string
}

func parsePrecisionArgs(argv []string) (precisionArgs, error) {
	out := precisionArgs{format: "text"}
	val := func(i *int, name string) (string, error) {
		*i++
		if *i >= len(argv) {
			return "", fmt.Errorf("missing value for %s", name)
		}
		return argv[*i], nil
	}
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		var err error
		switch {
		case a == "--labels":
			out.labels, err = val(&i, a)
		case strings.HasPrefix(a, "--labels="):
			out.labels = strings.TrimPrefix(a, "--labels=")
		case a == "--report":
			out.report, err = val(&i, a)
		case strings.HasPrefix(a, "--report="):
			out.report = strings.TrimPrefix(a, "--report=")
		case a == "--format":
			out.format, err = val(&i, a)
		case strings.HasPrefix(a, "--format="):
			out.format = strings.TrimPrefix(a, "--format=")
		case a == "-h" || a == "--help":
			return out, errHelp
		default:
			return out, fmt.Errorf("unknown argument %s", a)
		}
		if err != nil {
			return out, err
		}
	}
	if out.labels == "" {
		return out, fmt.Errorf("missing --labels")
	}
	if out.format != "text" && out.format != "json" {
		return out, fmt.Errorf("invalid --format %q (want text|json)", out.format)
	}
	return out, nil
}

// runPrecision implements `testscan precision`; returns the process exit code.
func runPrecision(argv []string, stdout, stderr io.Writer) int {
	args, err := parsePrecisionArgs(argv)
	if errors.Is(err, errHelp) {
		fmt.Fprintln(stderr, precisionUsage)
		return 0
	}
	if err != nil {
		fmt.Fprintf(stderr, "testscan: precision: %v\n", err)
		return 2
	}
	labels, err := scan.LoadLabels(args.labels)
	if err != nil {
		fmt.Fprintf(stderr, "testscan: %v\n", err)
		return 2
	}
	var report []scan.Finding
	if args.report != "" {
		report, err = loadReportFindings(args.report)
		if err != nil {
			fmt.Fprintf(stderr, "testscan: %v\n", err)
			return 2
		}
	}
	stats := scan.ComputePrecision(labels, report)
	total := scan.TotalStats(stats)
	if err := writePrecision(stdout, stats, total, args.format); err != nil {
		fmt.Fprintf(stderr, "testscan: %v\n", err)
		return 2
	}
	return 0
}

// loadReportFindings reads a testscan JSON report ({"summary","findings"}) or a
// bare findings array. The result is never nil (an empty report filters out all labels).
func loadReportFindings(path string) ([]scan.Finding, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("report: %w", err)
	}
	var rep scan.JSONReport
	if err := json.Unmarshal(data, &rep); err != nil {
		var bare []scan.Finding
		if err2 := json.Unmarshal(data, &bare); err2 != nil {
			return nil, fmt.Errorf("report: %s: %w", path, err)
		}
		rep.Findings = bare
	}
	if rep.Findings == nil {
		rep.Findings = []scan.Finding{}
	}
	return rep.Findings, nil
}

func writePrecision(w io.Writer, stats []scan.RuleStats, total scan.RuleStats, format string) error {
	if format == "json" {
		if stats == nil {
			stats = []scan.RuleStats{}
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			Rules []scan.RuleStats `json:"rules"`
			Total scan.RuleStats   `json:"total"`
		}{stats, total})
	}
	width := len("RULE")
	for _, s := range stats {
		if len(s.Rule) > width {
			width = len(s.Rule)
		}
	}
	row := func(name string, n, tp, fp int, p, wl float64) error {
		_, err := fmt.Fprintf(w, "%-*s  %4d  %4d  %4d  %9.2f  %8.2f\n", width, name, n, tp, fp, p, wl)
		return err
	}
	if _, err := fmt.Fprintf(w, "%-*s  %4s  %4s  %4s  %9s  %8s\n", width, "RULE", "N", "TP", "FP", "PRECISION", "WILSON95"); err != nil {
		return err
	}
	for _, s := range stats {
		if err := row(s.Rule, s.N, s.TP, s.FP, s.Precision, s.WilsonLow); err != nil {
			return err
		}
	}
	return row("TOTAL", total.N, total.TP, total.FP, total.Precision, total.WilsonLow)
}
