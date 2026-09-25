package scan

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type gitIgnore struct {
	patterns []giPattern
}

type giPattern struct {
	negated bool
	dirOnly bool
	raw     string
}

func loadGitIgnoreFile(path string) (*gitIgnore, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &gitIgnore{}, nil
		}
		return nil, err
	}
	defer f.Close()

	g := &gitIgnore{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		neg := false
		if strings.HasPrefix(line, "!") {
			neg = true
			line = line[1:]
		}
		dirOnly := strings.HasSuffix(line, "/")
		if dirOnly {
			line = strings.TrimSuffix(line, "/")
		}
		line = strings.TrimPrefix(line, "/")
		if line == "" {
			continue
		}
		g.patterns = append(g.patterns, giPattern{negated: neg, dirOnly: dirOnly, raw: filepath.ToSlash(line)})
	}
	return g, sc.Err()
}

// ignored reports whether relPath (slash-separated, relative to ignore root) is ignored.
func (g *gitIgnore) ignored(relPath string, isDir bool) bool {
	if g == nil || len(g.patterns) == 0 {
		return false
	}
	relPath = filepath.ToSlash(relPath)
	matched := false
	for _, p := range g.patterns {
		if p.dirOnly && !isDir {
			// still allow matching if path is under a dir pattern via prefix
			if matchGitPattern(p.raw, relPath) || hasPathPrefix(relPath, p.raw) {
				if p.negated {
					matched = false
				} else {
					matched = true
				}
			}
			continue
		}
		if matchGitPattern(p.raw, relPath) {
			if p.negated {
				matched = false
			} else {
				matched = true
			}
		}
	}
	return matched
}

func hasPathPrefix(path, prefix string) bool {
	path = filepath.ToSlash(path)
	prefix = filepath.ToSlash(prefix)
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func matchGitPattern(pattern, name string) bool {
	pattern = filepath.ToSlash(pattern)
	name = filepath.ToSlash(name)
	// patterns without slash match in any directory (basename or full)
	if !strings.Contains(pattern, "/") {
		if MatchGlob(pattern, filepath.Base(name)) {
			return true
		}
		// also ** / pattern
		return MatchGlob("**/"+pattern, name)
	}
	return MatchGlob(pattern, name) || MatchGlob("**/"+pattern, name)
}

func findGitIgnore(root string) *gitIgnore {
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return &gitIgnore{}
		}
		root = cwd
	}
	gi, err := loadGitIgnoreFile(filepath.Join(root, ".gitignore"))
	if err != nil || gi == nil {
		return &gitIgnore{}
	}
	return gi
}
