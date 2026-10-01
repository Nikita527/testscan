package scan_test

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nikita527/testscan/scan"
)

func TestWilsonLower(t *testing.T) {
	cases := []struct {
		tp, n int
		want  float64
	}{
		{8, 10, 0.4902},
		{20, 20, 0.8389},
		{0, 0, 0},
		{0, 10, 0},
	}
	for _, tc := range cases {
		got := scan.WilsonLower(tc.tp, tc.n, scan.WilsonZ95)
		if math.Abs(got-tc.want) > 1e-3 {
			t.Errorf("WilsonLower(%d,%d)=%v, want ~%v", tc.tp, tc.n, got, tc.want)
		}
	}
}

func writeLabels(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "labels.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadLabels(t *testing.T) {
	p := writeLabels(t, `[
 {"fingerprint":"a","rule":"r1","file":"f.py","line":1,"label":"fp"},
 {"fingerprint":"b","rule":"r1","file":"f.py","line":2,"label":"tp","note":"x"},
 {"fingerprint":"a","rule":"r1","file":"f.py","line":1,"label":"tp"}]`)
	ls, err := scan.LoadLabels(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(ls) != 2 || ls[0].Fingerprint != "a" || ls[0].Label != "tp" || ls[1].Note != "x" {
		t.Fatalf("last-wins dedupe failed: %+v", ls)
	}

	for name, body := range map[string]string{
		"bad label":      `[{"fingerprint":"a","rule":"r","label":"maybe"}]`,
		"empty label":    `[{"fingerprint":"a","rule":"r"}]`,
		"no fingerprint": `[{"rule":"r","label":"tp"}]`,
		"not array":      `{"a":1}`,
	} {
		if _, err := scan.LoadLabels(writeLabels(t, body)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if _, err := scan.LoadLabels(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Error("missing file: expected error")
	}
}

func TestComputePrecision(t *testing.T) {
	labels := []scan.Label{
		{Fingerprint: "1", Rule: "b-rule", Label: "tp"},
		{Fingerprint: "2", Rule: "b-rule", Label: "fp"},
		{Fingerprint: "3", Rule: "b-rule", Label: "skip"},
		{Fingerprint: "4", Rule: "a-rule", Label: "tp"},
	}
	all := scan.ComputePrecision(labels, nil)
	if len(all) != 2 || all[0].Rule != "a-rule" || all[1].Rule != "b-rule" {
		t.Fatalf("not sorted by rule: %+v", all)
	}
	b := all[1]
	if b.N != 2 || b.TP != 1 || b.FP != 1 || b.Skip != 1 || b.Precision != 0.5 || b.WilsonLow <= 0 {
		t.Fatalf("b-rule stats: %+v", b)
	}

	filtered := scan.ComputePrecision(labels, []scan.Finding{
		{Fingerprint: "1", Rule: "b-rule"}, {Fingerprint: "4", Rule: "a-rule"}, {Fingerprint: "zz", Rule: "c"},
	})
	if len(filtered) != 2 || filtered[1].N != 1 || filtered[1].Precision != 1 || filtered[1].Skip != 0 {
		t.Fatalf("report filtering: %+v", filtered)
	}
	if got := scan.ComputePrecision(labels, []scan.Finding{}); len(got) != 0 {
		t.Fatalf("empty non-nil report should match nothing: %+v", got)
	}
}

func TestUnknownRuleNotConfirmed(t *testing.T) {
	info, ok := scan.LookupPrecision("definitely-not-a-rule")
	if ok || info.Precision != 0 || info.Source != "estimated" {
		t.Fatalf("unknown lookup = %+v, %v", info, ok)
	}
	if w := scan.PrecisionWeight("definitely-not-a-rule"); w != 0 {
		t.Fatalf("weight=%v, want 0", w)
	}
	f := []scan.Finding{{Rule: "definitely-not-a-rule", Severity: "error", File: "a.py", Line: 1}}
	if n := scan.CountConfirmed(f); n != 0 {
		t.Fatalf("confirmed=%d, want 0", n)
	}
	if got := scan.FilterLowPrecision(f); len(got) != 0 {
		t.Fatalf("unknown rule must be filtered by default: %+v", got)
	}
	if info, ok := scan.LookupPrecision("empty-test"); !ok || info.Precision != 1 || info.Source != "estimated" {
		t.Fatalf("empty-test = %+v, %v", info, ok)
	}
}

func TestWriteHTML_Labels(t *testing.T) {
	findings := []scan.Finding{
		{File: "t.py", Line: 3, Rule: "empty-test", Severity: "error", Message: "m", Fingerprint: "abc123"},
		{File: "t.py", Line: 9, Rule: "empty-test", Severity: "error", Message: "other"},
	}
	var buf bytes.Buffer
	if err := scan.WriteHTML(&buf, findings, scan.CalculateScore(findings, 1)); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`data-fp="abc123"`, `data-label="tp"`, `data-label="fp"`,
		"Export labels", `id="export-labels"`, "labeled: 0", "testscan.labels.v1",
		"labels.json", "new Blob", "a.download", "try {",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("html missing %q", want)
		}
	}
	if strings.Count(out, `data-fp="`) != 2 { // once per view (file, rule)
		t.Errorf("only the fingerprinted finding gets buttons")
	}
	if strings.Contains(out, "<script src") {
		t.Error("report must be self-contained")
	}
	// Every localStorage access must sit inside a try block.
	if c := strings.Count(out, "localStorage"); c != 2 {
		t.Errorf("expected exactly 2 localStorage accesses, got %d", c)
	}
	for _, call := range []string{"localStorage.getItem", "localStorage.setItem"} {
		i := strings.Index(out, call)
		if i < 0 {
			t.Fatalf("missing %s", call)
		}
		pre := out[:i]
		if strings.LastIndex(pre, "try {") < strings.LastIndex(pre, "function ") {
			t.Errorf("%s not inside try", call)
		}
	}
}
