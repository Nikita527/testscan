package scan

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// skipDirs are always skipped (venv / caches / editor worktrees); exclude + .gitignore add more.
var skipDirs = map[string]struct{}{
	".venv": {}, "venv": {}, ".git": {},
	"__pycache__": {}, "node_modules": {}, ".tox": {},
	// Agent/editor worktrees duplicate the suite and inflate findings/score.
	".claude": {}, ".cursor": {},
}

var defaultPythonFiles = []string{"test_*.py", "*_test.py"}

// WalkOptions configures test file discovery.
type WalkOptions struct {
	Exclude          []string
	PythonFiles      []string // empty → test_*.py / *_test.py
	RespectGitignore bool
	Root             string // for .gitignore and relative exclude matching; empty → cwd
	// OnlyPaths, when non-empty, keeps only matching paths (slash-relative or basename).
	OnlyPaths []string
}

func isTestPy(name string, patterns []string) bool {
	if !strings.HasSuffix(name, ".py") {
		return false
	}
	if len(patterns) == 0 {
		patterns = defaultPythonFiles
	}
	for _, p := range patterns {
		if MatchGlob(p, name) {
			return true
		}
	}
	return false
}

func Walk(ctx context.Context, roots []string, opts WalkOptions) ([]File, error) {
	root := opts.Root
	if root == "" {
		if cwd, err := os.Getwd(); err == nil {
			root = cwd
		}
	}
	var gi *gitIgnore
	if opts.RespectGitignore {
		gi = findGitIgnore(root)
	}

	files := []File{}
	for _, walkRoot := range roots {
		err := filepath.Walk(walkRoot, func(path string, info os.FileInfo, err error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err != nil {
				return err
			}
			rel := relToRoot(path, root)

			if info.IsDir() {
				if _, skip := skipDirs[info.Name()]; skip {
					return filepath.SkipDir
				}
				if gi != nil && gi.ignored(rel, true) {
					return filepath.SkipDir
				}
				if MatchAny(opts.Exclude, rel) || MatchAny(opts.Exclude, info.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if gi != nil && gi.ignored(rel, false) {
				return nil
			}
			if MatchAny(opts.Exclude, rel) || MatchAny(opts.Exclude, info.Name()) {
				return nil
			}
			if !isTestPy(info.Name(), opts.PythonFiles) {
				return nil
			}
			if len(opts.OnlyPaths) > 0 && !pathInOnly(path, rel, info.Name(), opts.OnlyPaths) {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files = append(files, File{Path: path, Content: content})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

// pathInOnly reports whether path/rel matches any allowlisted entry.
// Basename-only matches are intentionally omitted to avoid scanning unrelated
// same-named files across packages under --diff.
func pathInOnly(absPath, rel, base string, only []string) bool {
	_ = base
	absSlash := filepath.ToSlash(absPath)
	relSlash := filepath.ToSlash(rel)
	for _, p := range only {
		p = filepath.ToSlash(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		if p == relSlash || p == absSlash {
			return true
		}
		// Suffix match: allow "tests/foo.py" against deep abs paths.
		if strings.HasSuffix(absSlash, "/"+p) || strings.HasSuffix(relSlash, "/"+p) {
			return true
		}
		if MatchGlob(p, relSlash) {
			return true
		}
	}
	return false
}

func relToRoot(path, root string) string {
	if root == "" {
		return filepath.ToSlash(path)
	}
	if rel, err := filepath.Rel(root, path); err == nil {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}

// MakeIgnoreFunc returns a predicate for project-wide walks: skipDirs, exclude globs,
// and optional .gitignore (loaded once). name is the path base; rel is slash-relative to root.
func MakeIgnoreFunc(root string, exclude []string, respectGitignore bool) func(name, rel string, isDir bool) bool {
	var gi *gitIgnore
	if respectGitignore {
		gi = findGitIgnore(root)
	}
	return func(name, rel string, isDir bool) bool {
		if isDir {
			if _, ok := skipDirs[name]; ok {
				return true
			}
		}
		if gi != nil && gi.ignored(rel, isDir) {
			return true
		}
		return MatchAny(exclude, rel) || MatchAny(exclude, name)
	}
}
