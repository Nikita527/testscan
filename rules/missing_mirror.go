package rules

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/Nikita527/testscan/scan"
)

const (
	defaultSourceGlob     = "app/**/domain/*.py"
	defaultMirrorTemplate = "tests/{x}/domain/test_{m}.py"
)

// MissingMirrorTestOpts configures missing-mirror-test.
type MissingMirrorTestOpts struct {
	SourceGlob     string
	MirrorTemplate string
}

type missingMirrorTest struct {
	sourceGlob     string
	mirrorTemplate string
}

func (missingMirrorTest) ID() string { return "missing-mirror-test" }

func (missingMirrorTest) NeedsAST() bool { return false }

func (missingMirrorTest) Check(scan.File) []scan.Finding { return nil }

func (m missingMirrorTest) CheckProject(_ context.Context, info scan.ProjectInfo) []scan.Finding {
	root := info.PathRoot
	if root == "" {
		return nil
	}
	glob := m.sourceGlob
	if glob == "" {
		glob = defaultSourceGlob
	}
	tmpl := m.mirrorTemplate
	if tmpl == "" {
		tmpl = defaultMirrorTemplate
	}
	ignore := scan.MakeIgnoreFunc(root, info.Exclude, info.RespectGitignore)
	var findings []scan.Finding
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if d.IsDir() {
			if ignore(d.Name(), relSlash, true) {
				return filepath.SkipDir
			}
			return nil
		}
		if ignore(d.Name(), relSlash, false) {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(path), ".py") {
			return nil
		}
		if !scan.MatchGlob(glob, relSlash) {
			return nil
		}
		mirrorRel := expandMirrorTemplate(tmpl, glob, relSlash)
		if mirrorRel == "" {
			return nil
		}
		mirrorAbs := filepath.Join(root, filepath.FromSlash(mirrorRel))
		if st, err := os.Stat(mirrorAbs); err == nil && !st.IsDir() {
			return nil
		}
		findings = append(findings, scan.Finding{
			File:     path,
			Line:     1,
			Rule:     "missing-mirror-test",
			Severity: "warning",
			Message:  "no mirror test at " + mirrorRel,
		})
		return nil
	})
	return findings
}

// expandMirrorTemplate fills {m} (stem) and {x} (segments between glob root and a
// fixed template segment such as "domain").
func expandMirrorTemplate(tmpl, sourceGlob, relSlash string) string {
	base := filepath.Base(relSlash)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	x := mirrorXSegments(sourceGlob, relSlash)
	out := tmpl
	out = strings.ReplaceAll(out, "{m}", stem)
	out = strings.ReplaceAll(out, "{x}", x)
	return filepath.ToSlash(out)
}

func mirrorXSegments(sourceGlob, relSlash string) string {
	relParts := strings.Split(relSlash, "/")
	globParts := strings.Split(filepath.ToSlash(sourceGlob), "/")

	// Find the index of a concrete anchor shared with the template (e.g. "domain").
	anchor := ""
	for _, g := range globParts {
		if g == "**" || g == "*" || strings.ContainsAny(g, "*?[") {
			continue
		}
		if g == "" {
			continue
		}
		// Prefer non-root package segment that appears before the file glob.
		anchor = g
	}
	// Prefer "domain" if present in glob as concrete segment.
	for _, g := range globParts {
		if g == "domain" {
			anchor = "domain"
			break
		}
	}

	if anchor == "" {
		if len(relParts) >= 2 {
			return strings.Join(relParts[1:len(relParts)-1], "/")
		}
		return ""
	}

	// Drop leading concrete prefix shared with glob (e.g. "app").
	start := 0
	for _, g := range globParts {
		if g == "**" || strings.ContainsAny(g, "*?[") {
			break
		}
		if start < len(relParts) && relParts[start] == g {
			start++
			continue
		}
		break
	}

	anchorIdx := -1
	for i := start; i < len(relParts); i++ {
		if relParts[i] == anchor {
			anchorIdx = i
			break
		}
	}
	if anchorIdx < 0 {
		if len(relParts) >= 2 {
			return strings.Join(relParts[start:len(relParts)-1], "/")
		}
		return ""
	}
	if anchorIdx <= start {
		return ""
	}
	return strings.Join(relParts[start:anchorIdx], "/")
}

func NewMissingMirrorTest(opts MissingMirrorTestOpts) scan.Rule {
	return missingMirrorTest{
		sourceGlob:     opts.SourceGlob,
		mirrorTemplate: opts.MirrorTemplate,
	}
}

var _ scan.ProjectRule = missingMirrorTest{}
