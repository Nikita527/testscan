package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/Nikita527/testscan/scan"
)

const defaultHTMLReport = "testscan.html"

// openReport opens an HTML path in the default browser. Tests replace this
// to avoid "File Not Found" dialogs after t.TempDir cleanup.
var openReport = openHTMLReport

// emitFindings writes the report to stdout, or to -o/--output when set.
// --open opens the HTML file in a browser (requires a file path: -o or default).
func emitFindings(findings []scan.Finding, fileCount int, format, output string, open bool) error {
	if open && format != "html" {
		return fmt.Errorf("--open requires --format html")
	}

	if output != "" {
		if err := writeFindingsToFile(output, findings, fileCount, format); err != nil {
			return err
		}
		if open {
			return openWrittenReport(output)
		}
		return nil
	}

	if open {
		// no -o: write default HTML path then open
		if err := writeFindingsToFile(defaultHTMLReport, findings, fileCount, "html"); err != nil {
			return err
		}
		return openWrittenReport(defaultHTMLReport)
	}

	return writeFindings(os.Stdout, findings, fileCount, format)
}

func writeFindingsToFile(path string, findings []scan.Finding, fileCount int, format string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	err = writeFindings(f, findings, fileCount, format)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	fmt.Fprintf(os.Stderr, "testscan: wrote %s\n", abs)
	return nil
}

func openWrittenReport(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	if err := openReport(abs); err != nil {
		fmt.Fprintf(os.Stderr, "testscan: open browser: %v\n", err)
		return nil // best-effort
	}
	fmt.Fprintln(os.Stderr, "testscan: opened in default browser")
	return nil
}

func openHTMLReport(absPath string) error {
	switch runtime.GOOS {
	case "windows":
		// empty title argument so paths with spaces are not misparsed
		cmd := exec.Command("cmd", "/C", "start", "", absPath)
		return cmd.Start()
	case "darwin":
		return exec.Command("open", absPath).Start()
	default:
		return exec.Command("xdg-open", absPath).Start()
	}
}
