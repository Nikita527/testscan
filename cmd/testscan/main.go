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

const usage = "usage: testscan [path...] [--format text|json] [--fail-on error|warning|never]"

var errHelp = errors.New("help")

func main() {
	roots, format, failOn, err := parseArgs(os.Args[1:])
	if errors.Is(err, errHelp) {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(0)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "testscan: %v\n", err)
		os.Exit(2)
	}
	if len(roots) == 0 {
		roots = []string{"."}
	}

	findings, err := scan.Run(context.Background(), roots, scan.Options{
		Rules: rules.Default(),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "testscan: %v\n", err)
		os.Exit(2)
	}

	if err := writeFindings(os.Stdout, findings, format); err != nil {
		fmt.Fprintf(os.Stderr, "testscan: %v\n", err)
		os.Exit(2)
	}

	os.Exit(exitCode(findings, failOn))
}

// parseArgs допускает флаги до и после путей (как в SPEC: testscan path --format json).
func parseArgs(args []string) (roots []string, format, failOn string, err error) {
	format = "text"
	failOn = "error"

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			return nil, "", "", errHelp
		case a == "--format":
			i++
			if i >= len(args) {
				return nil, "", "", fmt.Errorf("missing value for --format")
			}
			format = args[i]
		case strings.HasPrefix(a, "--format="):
			format = strings.TrimPrefix(a, "--format=")
		case a == "--fail-on":
			i++
			if i >= len(args) {
				return nil, "", "", fmt.Errorf("missing value for --fail-on")
			}
			failOn = args[i]
		case strings.HasPrefix(a, "--fail-on="):
			failOn = strings.TrimPrefix(a, "--fail-on=")
		case strings.HasPrefix(a, "-"):
			return nil, "", "", fmt.Errorf("unknown flag %s", a)
		default:
			roots = append(roots, a)
		}
	}

	if format != "text" && format != "json" {
		return nil, "", "", fmt.Errorf("invalid --format %q (want text|json)", format)
	}
	if failOn != "error" && failOn != "warning" && failOn != "never" {
		return nil, "", "", fmt.Errorf("invalid --fail-on %q (want error|warning|never)", failOn)
	}
	return roots, format, failOn, nil
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
