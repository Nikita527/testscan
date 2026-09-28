package scan_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Nikita527/testscan/scan"
)

func TestWriteCodeQuality(t *testing.T) {
	findings := []scan.Finding{
		{
			File:        "tests/test_a.py",
			Line:        3,
			Rule:        "empty-test",
			Severity:    "error",
			Message:     "empty test body",
			Fingerprint: "abc123",
		},
		{
			File:     "tests/test_b.py",
			Line:     0,
			Rule:     "weak-assert",
			Severity: "note",
			Message:  "weak",
		},
	}
	var buf bytes.Buffer
	if err := scan.WriteCodeQuality(&buf, findings); err != nil {
		t.Fatal(err)
	}
	var issues []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &issues); err != nil {
		t.Fatal(err)
	}
	if len(issues) != 2 {
		t.Fatalf("got %d issues", len(issues))
	}
	if issues[0]["check_name"] != "empty-test" {
		t.Fatalf("check_name=%v", issues[0]["check_name"])
	}
	if issues[0]["severity"] != "major" {
		t.Fatalf("severity=%v", issues[0]["severity"])
	}
	if issues[0]["fingerprint"] != "abc123" {
		t.Fatalf("fingerprint=%v", issues[0]["fingerprint"])
	}
	loc := issues[0]["location"].(map[string]any)
	if loc["path"] != "tests/test_a.py" {
		t.Fatalf("path=%v", loc["path"])
	}
	lines := loc["lines"].(map[string]any)
	if int(lines["begin"].(float64)) != 3 {
		t.Fatalf("begin=%v", lines["begin"])
	}
	if issues[1]["severity"] != "info" {
		t.Fatalf("note severity=%v", issues[1]["severity"])
	}
	loc2 := issues[1]["location"].(map[string]any)
	lines2 := loc2["lines"].(map[string]any)
	if int(lines2["begin"].(float64)) != 1 {
		t.Fatalf("line 0 must become begin=1, got %v", lines2["begin"])
	}
}

func TestWriteCodeQuality_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := scan.WriteCodeQuality(&buf, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "[]") {
		t.Fatalf("want empty array, got %s", buf.String())
	}
}
