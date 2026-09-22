package scan

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// каталоги, которые не обходим
var skipDirs = map[string]struct{}{
	".venv": {}, "venv": {}, ".git": {},
	"__pycache__": {}, "node_modules": {}, ".tox": {},
}

func isTestPy(name string) bool {
	if !strings.HasSuffix(name, ".py") {
		return false
	}
	// test_*.py
	if strings.HasPrefix(name, "test_") {
		return true
	}
	// *_test.py
	base := strings.TrimSuffix(name, ".py")
	return strings.HasSuffix(base, "_test")
}

func Walk(ctx context.Context, roots []string) ([]File, error) {
	files := []File{}
	for _, root := range roots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err != nil {
				return err
			}
			if info.IsDir() {
				if _, skip := skipDirs[info.Name()]; skip {
					return filepath.SkipDir
				}
				return nil
			}
			if !isTestPy(info.Name()) {
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
