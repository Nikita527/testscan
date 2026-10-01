package scan

import (
	"fmt"
	"io"
	"strings"
)

// WriteAgent writes the compact --format agent output: one line per finding
// ("file:line rule — problem → fix", no header) and a single summary line.
func WriteAgent(w io.Writer, findings []Finding, score Score) error {
	counts := map[string]int{}
	for _, f := range findings {
		line := fmt.Sprintf("%s:%d %s — %s", f.File, f.Line, f.Rule, oneLine(f.Message))
		if f.Fix != "" {
			line += " → " + oneLine(f.Fix)
		}
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
		counts[strings.ToLower(f.Severity)]++
	}
	_, err := fmt.Fprintln(w, agentSummary(findings, counts, score))
	return err
}

func agentSummary(findings []Finding, counts map[string]int, score Score) string {
	noun := "findings"
	if len(findings) == 1 {
		noun = "finding"
	}
	var parts []string
	for _, sev := range []string{"error", "warning", "note"} {
		if counts[sev] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[sev], sev))
		}
	}
	s := fmt.Sprintf("testscan: %d %s", len(findings), noun)
	if len(parts) > 0 {
		s += " (" + strings.Join(parts, ", ") + ")"
	}
	if score.Tests > 0 {
		s += fmt.Sprintf(" · %d tests", score.Tests)
	}
	if score.ParseSkipped > 0 {
		s += " · " + ParseErrorNote(score.ParseSkipped)
	}
	if score.HiddenCount > 0 {
		s += fmt.Sprintf(" · %d hidden (use --all)", score.HiddenCount)
	}
	return s
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
