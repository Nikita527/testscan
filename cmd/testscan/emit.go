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

// emitFindings writes the report. On Windows, --format html writes testscan.html
// and opens it in the default browser instead of dumping HTML to the console.
func emitFindings(findings []scan.Finding, format string) error {
	if format == "html" && runtime.GOOS == "windows" {
		return writeHTMLFileAndOpen(defaultHTMLReport, findings)
	}
	return writeFindings(os.Stdout, findings, format)
}

func writeHTMLFileAndOpen(path string, findings []scan.Finding) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	err = scan.WriteHTML(f, findings)
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

	if err := openHTMLReport(abs); err != nil {
		fmt.Fprintf(os.Stderr, "testscan: open browser: %v\n", err)
		// report is on disk; opening is best-effort
	} else {
		fmt.Fprintln(os.Stderr, "testscan: opened in default browser")
	}
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
