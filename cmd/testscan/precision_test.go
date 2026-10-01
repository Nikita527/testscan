package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func precisionFixture(t *testing.T) (labels, report string) {
	t.Helper()
	dir := t.TempDir()
	labels = filepath.Join(dir, "labels.json")
	report = filepath.Join(dir, "out.json")
	l := `[
 {"fingerprint":"1","rule":"weak-assert","file":"a.py","line":1,"label":"tp"},
 {"fingerprint":"2","rule":"weak-assert","file":"a.py","line":2,"label":"fp"},
 {"fingerprint":"3","rule":"no-assert","file":"b.py","line":1,"label":"tp"}]`
	r := `{"summary":{},"findings":[
 {"file":"a.py","line":1,"rule":"weak-assert","severity":"warning","message":"m","fingerprint":"1"},
 {"file":"a.py","line":2,"rule":"weak-assert","severity":"warning","message":"m","fingerprint":"2"}]}`
	if err := os.WriteFile(labels, []byte(l), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(report, []byte(r), 0o600); err != nil {
		t.Fatal(err)
	}
	return labels, report
}

func TestRunPrecision_Text(t *testing.T) {
	labels, _ := precisionFixture(t)
	var out, errb bytes.Buffer
	if code := runPrecision([]string{"--labels", labels}, &out, &errb); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, errb.String())
	}
	s := out.String()
	for _, want := range []string{"RULE", "PRECISION", "WILSON95", "no-assert", "weak-assert", "TOTAL"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) != 4 || !strings.HasPrefix(lines[3], "TOTAL") {
		t.Fatalf("unexpected table:\n%s", s)
	}
}

func TestRunPrecision_JSONWithReport(t *testing.T) {
	labels, report := precisionFixture(t)
	var out, errb bytes.Buffer
	code := runPrecision([]string{"--labels=" + labels, "--report", report, "--format", "json"}, &out, &errb)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, errb.String())
	}
	var got struct {
		Rules []map[string]any `json:"rules"`
		Total map[string]any   `json:"total"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Rules) != 1 || got.Rules[0]["rule"] != "weak-assert" || got.Rules[0]["n"].(float64) != 2 {
		t.Fatalf("rules=%v", got.Rules)
	}
	if got.Total["tp"].(float64) != 1 || got.Total["fp"].(float64) != 1 {
		t.Fatalf("total=%v", got.Total)
	}
	if _, ok := got.Total["wilson_lower"]; !ok {
		t.Fatal("missing wilson_lower")
	}
}

func TestRunPrecision_Errors(t *testing.T) {
	labels, _ := precisionFixture(t)
	bad := filepath.Join(t.TempDir(), "bad.json")
	_ = os.WriteFile(bad, []byte(`[{"fingerprint":"x","label":"zzz"}]`), 0o600)
	for name, argv := range map[string][]string{
		"no labels":      {},
		"missing file":   {"--labels", filepath.Join(t.TempDir(), "nope.json")},
		"invalid label":  {"--labels", bad},
		"bad format":     {"--labels", labels, "--format", "xml"},
		"missing report": {"--labels", labels, "--report", filepath.Join(t.TempDir(), "nope.json")},
		"unknown flag":   {"--labels", labels, "--wat"},
	} {
		var out, errb bytes.Buffer
		if code := runPrecision(argv, &out, &errb); code != 2 {
			t.Errorf("%s: code=%d, want 2", name, code)
		}
	}
}
