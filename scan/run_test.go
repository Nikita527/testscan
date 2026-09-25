package scan_test

import (
	"context"
	"errors"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

type stubRule struct{}

func (stubRule) ID() string { return "stub" }

func (stubRule) Check(file scan.File) []scan.Finding {
	return []scan.Finding{{
		File:    file.Path,
		Line:    1,
		Rule:    "stub",
		Message: "ok",
	}}
}

type astStubRule struct{}

func (astStubRule) ID() string     { return "ast-stub" }
func (astStubRule) NeedsAST() bool { return true }
func (astStubRule) Check(file scan.File) []scan.Finding {
	return []scan.Finding{{
		File:    file.Path,
		Line:    1,
		Rule:    "ast-stub",
		Message: "ok",
	}}
}

type countingParser struct {
	calls atomic.Int64
}

func (c *countingParser) Parse(context.Context, string, []byte) (parse.Model, error) {
	c.calls.Add(1)
	return parse.Model{}, nil
}

func TestRun(t *testing.T) {
	res, err := scan.Run(context.Background(), []string{"testdata"}, scan.Options{
		Rules: []scan.Rule{stubRule{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	// 2 files in testdata × 1 rule
	if len(res.Findings) != 2 {
		t.Fatalf("got %d findings, want 2", len(res.Findings))
	}
	if res.Files != 2 {
		t.Fatalf("got %d files, want 2", res.Files)
	}
}

func TestRun_NoRules(t *testing.T) {
	_, err := scan.Run(context.Background(), []string{"testdata"}, scan.Options{})
	if !errors.Is(err, scan.ErrRulesRequired) {
		t.Fatalf("got %v, want ErrRulesRequired", err)
	}
}

func TestRun_ConcurrentSameAsSequential(t *testing.T) {
	roots := []string{"testdata"}
	seq, err := scan.Run(context.Background(), roots, scan.Options{
		Rules:   []scan.Rule{stubRule{}},
		Workers: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	par, err := scan.Run(context.Background(), roots, scan.Options{
		Rules:   []scan.Rule{stubRule{}},
		Workers: 8,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(seq, par) {
		t.Fatalf("concurrent != sequential\nseq=%v\npar=%v", seq, par)
	}
	assertFindingsSorted(t, seq.Findings)
}

func assertFindingsSorted(t *testing.T, findings []scan.Finding) {
	t.Helper()
	for i := 1; i < len(findings); i++ {
		a, b := findings[i-1], findings[i]
		ordered := a.File < b.File ||
			(a.File == b.File && a.Line < b.Line) ||
			(a.File == b.File && a.Line == b.Line && a.Rule <= b.Rule)
		if !ordered {
			t.Fatalf("findings not sorted at %d: %+v before %+v", i, a, b)
		}
	}
}

func TestWalk_Canceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := scan.Walk(ctx, []string{"testdata"}, scan.WalkOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}

func TestRun_Canceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := scan.Run(ctx, []string{"testdata"}, scan.Options{
		Rules: []scan.Rule{stubRule{}},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}

func TestRun_SkipsASTWhenNoRuleNeedsIt(t *testing.T) {
	cp := &countingParser{}
	_, err := scan.Run(context.Background(), []string{"testdata"}, scan.Options{
		Rules:  []scan.Rule{stubRule{}},
		Parser: cp,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := cp.calls.Load(); got != 0 {
		t.Fatalf("parse calls=%d, want 0 when no rule NeedsAST", got)
	}
}

func TestRun_ParsesWhenRuleNeedsAST(t *testing.T) {
	cp := &countingParser{}
	_, err := scan.Run(context.Background(), []string{"testdata"}, scan.Options{
		Rules:  []scan.Rule{astStubRule{}},
		Parser: cp,
	})
	if err != nil {
		t.Fatal(err)
	}
	// 2 files in testdata
	if got := cp.calls.Load(); got != 2 {
		t.Fatalf("parse calls=%d, want 2", got)
	}
}

func TestRun_OptionsParserOverridesGlobal(t *testing.T) {
	global := &countingParser{}
	opts := &countingParser{}
	// Intentionally exercises deprecated SetParser to assert Options.Parser wins.
	restore := parse.SetParser(global) //nolint:staticcheck // SA1019
	defer restore()

	_, err := scan.Run(context.Background(), []string{"testdata"}, scan.Options{
		Rules:  []scan.Rule{astStubRule{}},
		Parser: opts,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := opts.calls.Load(); got != 2 {
		t.Fatalf("Options.Parser calls=%d, want 2", got)
	}
	if got := global.calls.Load(); got != 0 {
		t.Fatalf("global parser calls=%d, want 0 when Options.Parser set", got)
	}
}

func TestRun_ResetsDiscoveryPatterns(t *testing.T) {
	parse.SetDiscoveryPatterns([]string{"check_*"}, []string{"Suite*"})
	t.Cleanup(func() { parse.SetDiscoveryPatterns(nil, nil) })

	_, err := scan.Run(context.Background(), []string{"testdata"}, scan.Options{
		Rules:  []scan.Rule{astStubRule{}},
		Parser: &countingParser{},
	})
	if err != nil {
		t.Fatal(err)
	}
	f, c := parse.DiscoveryPatterns()
	if len(f) != 0 || len(c) != 0 {
		t.Fatalf("Run must clear discovery when Options empty, got funcs=%v classes=%v", f, c)
	}
}
