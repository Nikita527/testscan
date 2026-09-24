package scan_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

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

func TestRun(t *testing.T) {
	findings, err := scan.Run(context.Background(), []string{"testdata"}, scan.Options{
		Rules: []scan.Rule{stubRule{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	// 2 файла в testdata × 1 правило
	if len(findings) != 2 {
		t.Fatalf("got %d findings, want 2", len(findings))
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
	assertFindingsSorted(t, seq)
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
	_, err := scan.Walk(ctx, []string{"testdata"})
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
