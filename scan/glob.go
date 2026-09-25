package scan

import (
	"path/filepath"
	"strings"
)

// MatchGlob reports whether name (slash-separated) matches pattern.
// Supports * (within a segment), ** (across segments), and ? .
func MatchGlob(pattern, name string) bool {
	pattern = filepath.ToSlash(pattern)
	name = filepath.ToSlash(name)
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		if name == prefix || strings.HasPrefix(name, prefix+"/") {
			return true
		}
	}
	return matchGlob(pattern, name)
}

func matchGlob(pattern, name string) bool {
	for len(pattern) > 0 {
		if pattern == "**" {
			return true
		}
		if strings.HasPrefix(pattern, "**/") {
			rest := pattern[3:]
			if matchGlob(rest, name) {
				return true
			}
			for i := 0; i < len(name); i++ {
				if name[i] == '/' && matchGlob(rest, name[i+1:]) {
					return true
				}
			}
			// ** also matches when rest matches a suffix after any prefix without slash constraint
			for i := 0; i <= len(name); i++ {
				if (i == 0 || name[i-1] == '/') && matchGlob(rest, name[i:]) {
					return true
				}
			}
			return false
		}

		star := strings.IndexByte(pattern, '*')
		qmark := strings.IndexByte(pattern, '?')
		special := -1
		if star >= 0 {
			special = star
		}
		if qmark >= 0 && (special < 0 || qmark < special) {
			special = qmark
		}
		if special < 0 {
			return pattern == name
		}
		if special > 0 {
			prefix := pattern[:special]
			if !strings.HasPrefix(name, prefix) {
				return false
			}
			pattern = pattern[special:]
			name = name[len(prefix):]
			continue
		}

		// pattern[0] is * or ?
		if pattern[0] == '?' {
			if name == "" || name[0] == '/' {
				return false
			}
			pattern = pattern[1:]
			name = name[1:]
			continue
		}

		// *
		if len(pattern) >= 2 && pattern[1] == '*' {
			// shouldn't normally reach bare ** here (handled above), but be safe
			pattern = pattern[1:]
			continue
		}
		rest := pattern[1:]
		seg := name
		if i := strings.IndexByte(name, '/'); i >= 0 {
			seg = name[:i]
		}
		if rest == "" {
			return !strings.Contains(name, "/")
		}
		for i := 0; i <= len(seg); i++ {
			if matchGlob(rest, name[i:]) {
				return true
			}
		}
		return false
	}
	return name == ""
}

// MatchAny reports whether name matches any pattern (full path or basename).
func MatchAny(patterns []string, name string) bool {
	name = filepath.ToSlash(name)
	base := filepath.Base(name)
	for _, p := range patterns {
		if p == "" {
			continue
		}
		if MatchGlob(p, name) || MatchGlob(p, base) {
			return true
		}
	}
	return false
}
