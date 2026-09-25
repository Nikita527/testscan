package scan

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	reStringLit = regexp.MustCompile(`(?s)'([^'\\]|\\.)*'|\"([^\"\\]|\\.)*\"`)
	reNumberLit = regexp.MustCompile(`\b\d+\.?\d*\b`)
	reWS        = regexp.MustCompile(`[ \t]+`)
)

// ComputeFingerprint returns rule+file+qualname+snippet hash (testscan/v1).
func ComputeFingerprint(f Finding, content []byte, pathRoot string) string {
	rel := RelPath(f.File, pathRoot)
	snippet := extractSnippet(content, f)
	norm := NormalizeSnippet(snippet)
	payload := f.Rule + "\n" + rel + "\n" + f.QualName + "\n" + norm
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:16])
}

// PrimaryLocationLineHash is a stable SARIF partial fingerprint without file content.
func PrimaryLocationLineHash(f Finding) string {
	payload := filepath.ToSlash(f.File) + ":" + strconv.Itoa(f.Line) + ":" + f.Rule + ":" + f.QualName
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:8])
}

// NormalizeSnippet collapses whitespace and replaces string/number literals.
func NormalizeSnippet(s string) string {
	s = reStringLit.ReplaceAllString(s, "S")
	s = reNumberLit.ReplaceAllString(s, "N")
	s = reWS.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func extractSnippet(content []byte, f Finding) string {
	if len(content) == 0 {
		return f.Message
	}
	lines := splitLines(string(content))
	if f.Line < 1 || f.Line > len(lines) {
		return f.Message
	}
	// Prefer test body from def line covering this finding.
	start := f.Line - 1
	for i := start; i >= 0; i-- {
		if _, ok := testDefLine(lines[i]); ok {
			start = i
			break
		}
	}
	end := start + 1
	for end < len(lines) {
		if _, ok := testDefLine(lines[end]); ok {
			break
		}
		if isClassDef(lines[end]) && end > start {
			break
		}
		end++
		if end-start > 80 {
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

// RelPath returns path relative to root with forward slashes; falls back to ToSlash(clean).
func RelPath(path, root string) string {
	path = filepath.Clean(path)
	if root != "" {
		if rel, err := filepath.Rel(root, path); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(path)
}

// RelativizeFindings rewrites File to slash-relative paths under root.
func RelativizeFindings(findings []Finding, root string) {
	for i := range findings {
		findings[i].File = RelPath(findings[i].File, root)
	}
}

// AssignFingerprints fills Fingerprint using file contents keyed by original path.
func AssignFingerprints(findings []Finding, byPath map[string][]byte, pathRoot string) {
	for i := range findings {
		content := contentFor(findings[i].File, byPath)
		findings[i].Fingerprint = ComputeFingerprint(findings[i], content, pathRoot)
	}
}

func contentFor(file string, byPath map[string][]byte) []byte {
	if c, ok := byPath[file]; ok {
		return c
	}
	key := pathKey(file)
	for p, c := range byPath {
		if pathKey(p) == key || RelPath(p, "") == file || filepath.ToSlash(p) == file {
			return c
		}
	}
	return nil
}
