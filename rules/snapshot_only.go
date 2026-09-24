package rules

import (
	"strings"

	"github.com/Nikita527/testscan/scan"
)

type snapshotOnly struct{}

func (snapshotOnly) ID() string {
	return "snapshot-only"
}

func (snapshotOnly) Check(file scan.File) []scan.Finding {
	src := string(file.Content)
	if !hasSnapshotMarker(src) {
		return nil
	}
	if hasBehavioralAssert(src) {
		return nil
	}
	return []scan.Finding{{
		File:     file.Path,
		Line:     lineOfAny(src, snapshotNeedles...),
		Rule:     "snapshot-only",
		Severity: "warning",
		Message:  "snapshot assert without behavioral assert",
	}}
}

var snapshotNeedles = []string{
	"assert_match_snapshot",
	"syrupy",
	"snapshot.assert_match",
	"toMatchSnapshot",
	"== snapshot",
}

func hasSnapshotMarker(src string) bool {
	for _, n := range snapshotNeedles {
		if strings.Contains(src, n) {
			return true
		}
	}
	return false
}

// hasBehavioralAssert — есть assert , не связанный со snapshot.
func hasBehavioralAssert(src string) bool {
	for _, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "assert ") {
			continue
		}
		lower := strings.ToLower(t)
		if strings.Contains(lower, "snapshot") || strings.Contains(lower, "assert_match_snapshot") {
			continue
		}
		return true
	}
	return false
}

func NewSnapshotOnly() scan.Rule {
	return snapshotOnly{}
}
