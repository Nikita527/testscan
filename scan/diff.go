package scan

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// DiffChangedPaths returns slash-relative paths changed or added since baseRef
// (git diff --name-only --diff-filter=ACMR). baseRef may include a triple-dot
// range (e.g. "origin/main...HEAD"); otherwise it is compared to the working tree.
func DiffChangedPaths(baseRef, repoRoot string) ([]string, error) {
	baseRef = strings.TrimSpace(baseRef)
	if baseRef == "" {
		return nil, fmt.Errorf("empty base ref")
	}
	if strings.HasPrefix(baseRef, "-") {
		return nil, fmt.Errorf("invalid base ref %q (must not start with -)", baseRef)
	}
	if repoRoot == "" {
		return nil, fmt.Errorf("empty repo root")
	}
	args := []string{"-C", repoRoot, "diff", "--name-only", "--diff-filter=ACMR", baseRef, "--"}
	cmd := exec.Command("git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("git diff %s: %s", baseRef, msg)
	}
	var out []string
	for _, line := range strings.Split(stdout.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, filepath.ToSlash(line))
	}
	return out, nil
}

// FilterTestPaths keeps paths that match Python test file discovery patterns.
func FilterTestPaths(paths []string, pythonFiles []string) []string {
	var out []string
	for _, p := range paths {
		base := filepath.Base(p)
		if isTestPy(base, pythonFiles) {
			out = append(out, filepath.ToSlash(p))
		}
	}
	return out
}
