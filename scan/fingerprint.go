package scan

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	reStringLit = regexp.MustCompile(`(?s)'([^'\\]|\\.)*'|\"([^\"\\]|\\.)*\"`)
	reNumberLit = regexp.MustCompile(`\b\d+\.?\d*\b`)
	reWS        = regexp.MustCompile(`[ \t]+`)
)

// ComputeFingerprint returns rule+file+qualname+snippet hash (testscan/v1)
// for a single occurrence (index 1). Prefer AssignFingerprints for batches so
// identical-shape findings in one test get distinct occurrence indices.
func ComputeFingerprint(f Finding, content []byte, pathRoot string) string {
	return computeFingerprintOcc(f, content, pathRoot, 1)
}

func computeFingerprintOcc(f Finding, content []byte, pathRoot string, occurrence int) string {
	if occurrence < 1 {
		occurrence = 1
	}
	rel := RelPath(f.File, pathRoot)
	snippet := extractSnippet(content, f)
	norm := NormalizeSnippet(snippet)
	payload := f.Rule + "\n" + rel + "\n" + f.QualName + "\n" + norm + "\n" + strconv.Itoa(occurrence)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:16])
}

// fingerprintBaseKey is the line-shift-stable identity without occurrence.
func fingerprintBaseKey(f Finding, content []byte, pathRoot string) string {
	rel := RelPath(f.File, pathRoot)
	snippet := extractSnippet(content, f)
	norm := NormalizeSnippet(snippet)
	return f.Rule + "\n" + rel + "\n" + f.QualName + "\n" + norm
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

// AssignFingerprints fills Fingerprint and Snippet using file contents keyed by
// original path. Identical rule+file+qualname+normalized-snippet findings get
// a 1-based occurrence index (sorted by line) so duplicates in one test are
// distinct for --baseline and precision, while staying stable under line shifts.
func AssignFingerprints(findings []Finding, byPath map[string][]byte, pathRoot string) {
	type meta struct {
		key  string
		line int
		idx  int
	}
	metas := make([]meta, len(findings))
	contents := make([][]byte, len(findings))
	for i := range findings {
		content := contentFor(findings[i].File, byPath)
		contents[i] = content
		metas[i] = meta{
			key:  fingerprintBaseKey(findings[i], content, pathRoot),
			line: findings[i].Line,
			idx:  i,
		}
	}
	order := make([]int, len(findings))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		ia, ib := order[a], order[b]
		if metas[ia].key != metas[ib].key {
			return metas[ia].key < metas[ib].key
		}
		if metas[ia].line != metas[ib].line {
			return metas[ia].line < metas[ib].line
		}
		return ia < ib
	})
	occ := make(map[string]int, len(findings))
	occOf := make([]int, len(findings))
	for _, i := range order {
		occ[metas[i].key]++
		occOf[i] = occ[metas[i].key]
	}
	for i := range findings {
		findings[i].Fingerprint = computeFingerprintOcc(findings[i], contents[i], pathRoot, occOf[i])
		if findings[i].Snippet == "" {
			findings[i].Snippet = ContextSnippet(contents[i], findings[i].Line, 5)
		}
	}
}

// ContextSnippet returns up to radius lines before and after 1-based line.
// Line numbers are prefixed (e.g. "  10| code") for HTML/report readability.
func ContextSnippet(content []byte, line, radius int) string {
	if len(content) == 0 || line < 1 || radius < 0 {
		return ""
	}
	lines := splitLines(string(content))
	if line > len(lines) {
		return ""
	}
	start := line - 1 - radius
	if start < 0 {
		start = 0
	}
	end := line + radius
	if end > len(lines) {
		end = len(lines)
	}
	var b strings.Builder
	width := len(strconv.Itoa(end))
	for i := start; i < end; i++ {
		if i > start {
			b.WriteByte('\n')
		}
		num := i + 1
		marker := " "
		if num == line {
			marker = ">"
		}
		fmt.Fprintf(&b, "%s%*d| %s", marker, width, num, lines[i])
	}
	return b.String()
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
