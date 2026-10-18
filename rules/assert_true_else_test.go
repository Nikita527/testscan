package rules_test

import (
	"path/filepath"
	"testing"

	"github.com/Nikita527/testscan/rules"
)

func TestAssertTrue_ElseExhaustivenessClean(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test_else.py"), `
def test_exhaustiveness(x):
    if isinstance(x, int):
        assert x > 0
    elif isinstance(x, str):
        assert x
    else:
        assert False
`)
	res, err := testRun(t, []string{dir}, rules.NewAssertTrue())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 0 {
		t.Fatalf("else: assert False must be clean, got %+v", res.Findings)
	}
}

func TestAssertTrue_SuppressesInsideBroadRaises(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test_auth.py"), `
import pytest

def test_auth():
    with pytest.raises(Exception):
        connect()
        assert False
`)
	trueHits, err := testRun(t, []string{dir}, rules.NewAssertTrue())
	if err != nil {
		t.Fatal(err)
	}
	insideHits, err := testRun(t, []string{dir}, rules.NewAssertInsideBroadRaises())
	if err != nil {
		t.Fatal(err)
	}
	if len(trueHits.Findings) != 0 {
		t.Fatalf("assert-true must defer to assert-inside-broad-raises, got %+v", trueHits.Findings)
	}
	if len(insideHits.Findings) != 1 {
		t.Fatalf("want 1 assert-inside-broad-raises, got %+v", insideHits.Findings)
	}
}

func TestAssertTrue_ForElseAndTryElseStillFlagged(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test_else_kinds.py"), `
def test_for_else():
    for x in []:
        break
    else:
        assert False

def test_try_else():
    try:
        x = 1
    except Exception:
        raise
    else:
        assert False
`)
	res, err := testRun(t, []string{dir}, rules.NewAssertTrue())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 2 {
		t.Fatalf("for/else and try/else assert False must be flagged, got %+v", res.Findings)
	}
}

func TestAssertTrue_MatchDefaultCaseClean(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test_match.py"), `
def test_match(x):
    match x:
        case 1:
            assert x == 1
        case _:
            assert False
`)
	res, err := testRun(t, []string{dir}, rules.NewAssertTrue())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 0 {
		t.Fatalf("case _: assert False must be clean, got %+v", res.Findings)
	}
}
