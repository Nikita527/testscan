package rules

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Nikita527/testscan/scan"
)

// coverage.py JSON report (subset).
type coverageReport struct {
	Files map[string]coverageFileEntry `json:"files"`
}

type coverageFileEntry struct {
	MissingLines    []int   `json:"missing_lines"`
	MissingBranches [][]int `json:"missing_branches"`
	ExecutedLines   []int   `json:"executed_lines"`
}

// findUncoveredErrorPaths loads a coverage.py JSON report and emits findings for
// raise/except lines (and related missing branches) that tests never hit.
func findUncoveredErrorPaths(coveragePath, pathRoot string) []scan.Finding {
	data, err := os.ReadFile(coveragePath)
	if err != nil {
		return []scan.Finding{{
			File:     coveragePath,
			Line:     1,
			Rule:     "only-happy-path",
			Severity: "note",
			Message:  fmt.Sprintf("coverage mode: cannot read %s: %v", coveragePath, err),
		}}
	}

	var report coverageReport
	if err := json.Unmarshal(data, &report); err != nil {
		return []scan.Finding{{
			File:     coveragePath,
			Line:     1,
			Rule:     "only-happy-path",
			Severity: "note",
			Message:  fmt.Sprintf("coverage mode: invalid coverage JSON %s: %v", coveragePath, err),
		}}
	}
	if len(report.Files) == 0 {
		return []scan.Finding{{
			File:     coveragePath,
			Line:     1,
			Rule:     "only-happy-path",
			Severity: "note",
			Message:  fmt.Sprintf("coverage mode: no files in %s", coveragePath),
		}}
	}

	var findings []scan.Finding
	for filePath, entry := range report.Files {
		abs := resolveCoverageFile(filePath, pathRoot, coveragePath)
		if isLikelyTestPath(abs) || isLikelyTestPath(filePath) {
			continue
		}
		src, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		errorLines := errorPathLineSet(string(src))
		if len(errorLines) == 0 {
			continue
		}
		missing := intSet(entry.MissingLines)
		// Branch arcs that never ran: source or destination on an error-path line.
		for _, br := range entry.MissingBranches {
			if len(br) < 1 {
				continue
			}
			srcLine := br[0]
			if _, ok := errorLines[srcLine]; ok {
				missing[srcLine] = true
			}
			if len(br) >= 2 && br[1] > 0 {
				if _, ok := errorLines[br[1]]; ok {
					missing[br[1]] = true
				}
			}
		}
		for line := range errorLines {
			if !missing[line] {
				continue
			}
			kind := errorLines[line]
			findings = append(findings, scan.Finding{
				File:     abs,
				Line:     line,
				Rule:     "only-happy-path",
				Severity: "note",
				Message:  fmt.Sprintf("uncovered %s not hit by tests (coverage mode)", kind),
			})
		}
	}
	return findings
}

func resolveCoverageFile(filePath, pathRoot, coveragePath string) string {
	if filepath.IsAbs(filePath) {
		return filepath.Clean(filePath)
	}
	// Prefer project root; fall back to coverage.json directory.
	candidates := []string{}
	if pathRoot != "" {
		candidates = append(candidates, filepath.Join(pathRoot, filePath))
	}
	candidates = append(candidates, filepath.Join(filepath.Dir(coveragePath), filePath))
	candidates = append(candidates, filePath)
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return filepath.Clean(c)
		}
	}
	if pathRoot != "" {
		return filepath.Clean(filepath.Join(pathRoot, filePath))
	}
	return filepath.Clean(filePath)
}

func isLikelyTestPath(path string) bool {
	base := filepath.Base(path)
	if strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py") {
		return true
	}
	if base == "conftest.py" {
		return true
	}
	slash := filepath.ToSlash(path)
	return strings.Contains(slash, "/tests/") ||
		strings.Contains(slash, "/test/") ||
		strings.HasSuffix(slash, "/tests") ||
		strings.HasSuffix(slash, "/test")
}

// errorPathLineSet maps lineno → "raise"|"except".
func errorPathLineSet(src string) map[int]string {
	out := map[int]string{}
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		if idx := strings.Index(trim, " #"); idx >= 0 {
			trim = strings.TrimSpace(trim[:idx])
		}
		lineno := i + 1
		switch {
		case trim == "raise" || strings.HasPrefix(trim, "raise ") || strings.HasPrefix(trim, "raise\t"):
			out[lineno] = "raise"
		case trim == "except" || strings.HasPrefix(trim, "except ") || strings.HasPrefix(trim, "except:"):
			out[lineno] = "except"
		}
	}
	return out
}

func intSet(xs []int) map[int]bool {
	out := make(map[int]bool, len(xs))
	for _, x := range xs {
		out[x] = true
	}
	return out
}
