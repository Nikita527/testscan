package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Nikita527/testscan/rules"
	"github.com/Nikita527/testscan/scan"
)

const usage = "usage: testscan [path...] [--format text|json] [--fail-on error|warning|never] [--rule ID] [--disable ID]"

var errHelp = errors.New("help")

type cliArgs struct {
	roots   []string
	format  string
	failOn  string
	only    []string
	disable []string
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
	if len(args.roots) == 0 {
		args.roots = []string{"."}
	}

	selected, err := rules.Select(rules.Default(), args.only, args.disable)
	if err != nil {
		fmt.Fprintf(os.Stderr, "testscan: %v\n", err)
		os.Exit(2)
	}

	findings, err := scan.Run(context.Background(), args.roots, scan.Options{
		Rules: selected,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "testscan: %v\n", err)
		os.Exit(2)
	}

	if err := writeFindings(os.Stdout, findings, args.format); err != nil {
		fmt.Fprintf(os.Stderr, "testscan: %v\n", err)
		os.Exit(2)
	}

	os.Exit(exitCode(findings, args.failOn))
}

// parseArgs допускает флаги до и после путей (как в SPEC: testscan path --format json).
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
		case strings.HasPrefix(a, "--fail-on="):
			out.failOn = strings.TrimPrefix(a, "--fail-on=")
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
		case strings.HasPrefix(a, "-"):
			return cliArgs{}, fmt.Errorf("unknown flag %s", a)
		default:
			out.roots = append(out.roots, a)
		}
	}

	if out.format != "text" && out.format != "json" {
		return cliArgs{}, fmt.Errorf("invalid --format %q (want text|json)", out.format)
	}
	if out.failOn != "error" && out.failOn != "warning" && out.failOn != "never" {
		return cliArgs{}, fmt.Errorf("invalid --fail-on %q (want error|warning|never)", out.failOn)
	}

	return out, nil
}

func writeFindings(w io.Writer, findings []scan.Finding, format string) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(findings)
	default:
		for _, f := range findings {
			if _, err := fmt.Fprintf(w, "%s:%d: %s %s: %s\n",
				f.File, f.Line, f.Severity, f.Rule, f.Message); err != nil {
				return err
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
