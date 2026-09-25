package scan

import (
	"path/filepath"
	"regexp"
	"strings"
)

const ignoreFileHeaderLines = 32

var (
	reIgnore     = regexp.MustCompile(`(?i)#\s*testscan:\s*ignore\[([^\]]*)\]`)
	reIgnoreFile = regexp.MustCompile(`(?i)#\s*testscan:\s*ignore-file\[([^\]]*)\]`)
)

// FilterInlineIgnores drops findings suppressed by # testscan: ignore[...] /
// ignore-file[...] comments. Uses file content from byPath (original Walk paths).
func FilterInlineIgnores(findings []Finding, byPath map[string][]byte) []Finding {
	if len(findings) == 0 {
		return findings
	}
	out := make([]Finding, 0, len(findings))
	cache := make(map[string]*ignoreIndex, len(byPath))
	for _, f := range findings {
		content, ok := byPath[f.File]
		if !ok {
			// try slash-normalized lookup
			for p, c := range byPath {
				if pathKey(p) == pathKey(f.File) {
					content = c
					ok = true
					break
				}
			}
		}
		if !ok {
			out = append(out, f)
			continue
		}
		idx := cache[f.File]
		if idx == nil {
			idx = buildIgnoreIndex(content)
			cache[f.File] = idx
		}
		if idx.suppresses(f) {
			continue
		}
		out = append(out, f)
	}
	return out
}

type ignoreIndex struct {
	fileRules map[string]struct{}         // ignore-file or header ignore
	lineRules map[int]map[string]struct{} // 1-based line → rules
	defLines  map[int]int                 // test body/finding line → def line
}

func buildIgnoreIndex(content []byte) *ignoreIndex {
	lines := splitLines(string(content))
	idx := &ignoreIndex{
		fileRules: map[string]struct{}{},
		lineRules: map[int]map[string]struct{}{},
		defLines:  map[int]int{},
	}

	// file-level: ignore-file anywhere in header, or ignore on early lines
	limit := ignoreFileHeaderLines
	if limit > len(lines) {
		limit = len(lines)
	}
	for i := 0; i < limit; i++ {
		line := lines[i]
		for _, id := range parseIgnoreFileIDs(line) {
			idx.fileRules[id] = struct{}{}
		}
		// also treat ignore[...] in header as file-level when not on a def
		if _, isDef := testDefLine(line); !isDef {
			for _, id := range parseIgnoreIDs(line) {
				idx.fileRules[id] = struct{}{}
			}
		}
	}
	// ignore-file anywhere in file
	for _, line := range lines {
		for _, id := range parseIgnoreFileIDs(line) {
			idx.fileRules[id] = struct{}{}
		}
	}

	for i, line := range lines {
		lineNo := i + 1
		ids := parseIgnoreIDs(line)
		if len(ids) > 0 {
			m := idx.lineRules[lineNo]
			if m == nil {
				m = map[string]struct{}{}
				idx.lineRules[lineNo] = m
			}
			for _, id := range ids {
				m[id] = struct{}{}
			}
		}
		if name, ok := testDefLine(line); ok {
			_ = name
			// map this def line and following body lines until next def/class at same indent — approx: until next top-level def
			end := len(lines)
			for j := i + 1; j < len(lines); j++ {
				if _, ok := testDefLine(lines[j]); ok {
					end = j
					break
				}
				if isClassDef(lines[j]) {
					end = j
					break
				}
			}
			for j := i; j < end; j++ {
				idx.defLines[j+1] = lineNo
			}
		}
	}
	return idx
}

func (idx *ignoreIndex) suppresses(f Finding) bool {
	if _, ok := idx.fileRules[f.Rule]; ok {
		return true
	}
	if _, ok := idx.fileRules["*"]; ok {
		return true
	}
	if rules, ok := idx.lineRules[f.Line]; ok {
		if _, hit := rules[f.Rule]; hit {
			return true
		}
		if _, hit := rules["*"]; hit {
			return true
		}
	}
	// previous line of finding
	if f.Line > 1 {
		if rules, ok := idx.lineRules[f.Line-1]; ok {
			if _, hit := rules[f.Rule]; hit {
				return true
			}
			if _, hit := rules["*"]; hit {
				return true
			}
		}
	}
	// ignore on def line (or line before def)
	if defLine, ok := idx.defLines[f.Line]; ok {
		if rules, ok := idx.lineRules[defLine]; ok {
			if _, hit := rules[f.Rule]; hit {
				return true
			}
			if _, hit := rules["*"]; hit {
				return true
			}
		}
		if defLine > 1 {
			if rules, ok := idx.lineRules[defLine-1]; ok {
				if _, hit := rules[f.Rule]; hit {
					return true
				}
				if _, hit := rules["*"]; hit {
					return true
				}
			}
		}
	}
	return false
}

func parseIgnoreIDs(line string) []string {
	m := reIgnore.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	return splitRuleList(m[1])
}

func parseIgnoreFileIDs(line string) []string {
	m := reIgnoreFile.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	return splitRuleList(m[1])
}

func splitRuleList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func splitLines(src string) []string {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	return strings.Split(src, "\n")
}

var reTestDef = regexp.MustCompile(`^(\s*)((?:async\s+)?def)\s+(test_\w+)\s*\(`)
var reClassDef = regexp.MustCompile(`^(\s*)class\s+\w+`)

func testDefLine(line string) (string, bool) {
	m := reTestDef.FindStringSubmatch(line)
	if m == nil {
		return "", false
	}
	return m[3], true
}

func isClassDef(line string) bool {
	return reClassDef.MatchString(line)
}

// pathKey normalizes for map lookup.
func pathKey(p string) string {
	return filepath.ToSlash(filepath.Clean(p))
}
