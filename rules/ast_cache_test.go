package rules_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/rules"
	"github.com/Nikita527/testscan/scan"
)

type countingParser struct {
	calls atomic.Int64
	model parse.Model
}

func (c *countingParser) Parse(context.Context, string, []byte) (parse.Model, error) {
	c.calls.Add(1)
	return c.model, nil
}

func TestRun_ASTOncePerFile(t *testing.T) {
	cp := &countingParser{
		model: parse.Model{Tests: []parse.TestFunc{{
			Name: "test_x", Lineno: 1, HasAssert: true,
		}}},
	}
	restore := parse.SetParser(cp)
	defer restore()

	// 2 файла в scan/testdata × 2 AST-правила → раньше 4 spawn, с кэшем 2
	_, err := scan.Run(context.Background(), []string{"../scan/testdata"}, scan.Options{
		Rules: []scan.Rule{
			rules.NewEmptyTest(),
			rules.NewDuplicateTestName(),
		},
		Workers: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := cp.calls.Load(); got != 2 {
		t.Fatalf("parse calls=%d, want 2 (one per file)", got)
	}
}
