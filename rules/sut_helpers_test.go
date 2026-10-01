package rules_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

// sutProject creates a temp project with package `app` and returns its root.
func sutProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app", "service.py"), []byte("def process(x):\n    if x < 0:\n        raise ValueError(x)\n    return x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "tests"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// sutFile wraps hand-built tests into a File whose tests each call the project
// function app.service.process once (on the line of the def, before any assert),
// so SUT detection resolves to a real project import.
func sutFile(t *testing.T, tests []parse.TestFunc, content string) scan.File {
	t.Helper()
	root := sutProject(t)
	for i := range tests {
		tests[i].Calls = append(tests[i].Calls, parse.Call{Name: "process", Lineno: tests[i].Lineno})
	}
	return scan.File{
		Path:        filepath.Join(root, "tests", "test_x.py"),
		Content:     []byte(content),
		ProjectRoot: root,
		ModelOK:     true,
		Model: parse.Model{
			Tests: tests,
			Imports: []parse.Import{{
				Kind: "from", Module: "app.service", Names: []string{"process"},
				Bindings: []parse.ImportBinding{{Local: "process", Module: "app.service", Name: "process"}},
			}},
		},
	}
}
