package scan_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/Nikita527/testscan/scan"
)

func TestFilterBaseline(t *testing.T) {
	findings := []scan.Finding{
		{File: "a.py", Line: 1, Rule: "empty-test", Message: "m1"},
		{File: "a.py", Line: 2, Rule: "todo-test", Message: "m2"},
		{File: "b.py", Line: 1, Rule: "empty-test", Message: "m3"},
	}

	cases := []struct {
		name     string
		baseline []scan.Finding
		want     int
		wantKeep []string // "file:line:rule"
	}{
		{
			name:     "empty_noop",
			baseline: nil,
			want:     3,
			wantKeep: []string{"a.py:1:empty-test", "a.py:2:todo-test", "b.py:1:empty-test"},
		},
		{
			name: "hit_suppressed_message_ignored",
			baseline: []scan.Finding{
				{File: "a.py", Line: 1, Rule: "empty-test", Message: "other"},
			},
			want:     2,
			wantKeep: []string{"a.py:2:todo-test", "b.py:1:empty-test"},
		},
		{
			name: "different_line_kept",
			baseline: []scan.Finding{
				{File: "a.py", Line: 99, Rule: "empty-test"},
			},
			want:     3,
			wantKeep: []string{"a.py:1:empty-test", "a.py:2:todo-test", "b.py:1:empty-test"},
		},
		{
			name: "different_rule_kept",
			baseline: []scan.Finding{
				{File: "a.py", Line: 1, Rule: "no-assert"},
			},
			want:     3,
			wantKeep: []string{"a.py:1:empty-test", "a.py:2:todo-test", "b.py:1:empty-test"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := scan.FilterBaseline(findings, tc.baseline)
			if len(got) != tc.want {
				t.Fatalf("got %d findings, want %d: %v", len(got), tc.want, got)
			}
			keep := map[string]bool{}
			for _, f := range got {
				keep[f.File+":"+strconv.Itoa(f.Line)+":"+f.Rule] = true
			}
			for _, k := range tc.wantKeep {
				if !keep[k] {
					t.Errorf("missing kept finding %s", k)
				}
			}
		})
	}
}

func TestFilterBaseline_SlashNormalization(t *testing.T) {
	// finding как с Windows Walk (\); baseline как в JSON (/)
	findings := []scan.Finding{{
		File: filepath.FromSlash("dir/sub/a.py"),
		Line: 1,
		Rule: "empty-test",
	}}
	baseline := []scan.Finding{{
		File: "dir/sub/a.py",
		Line: 1,
		Rule: "empty-test",
	}}
	got := scan.FilterBaseline(findings, baseline)
	if len(got) != 0 {
		t.Fatalf("want suppressed after path normalize, got %v", got)
	}
}

func TestLoadBaseline_BadJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := scan.LoadBaseline(path)
	if err == nil {
		t.Fatal("want error for bad JSON")
	}
}
