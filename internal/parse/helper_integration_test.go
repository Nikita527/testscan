package parse

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func hasPython() bool {
	for _, name := range []string{"uv", "python3", "python"} {
		if _, err := exec.LookPath(name); err == nil {
			return true
		}
	}
	return false
}

func TestHelperParser_Integration(t *testing.T) {
	if !hasPython() {
		t.Skip("no python/uv")
	}

	src := "def test_a():\n    pass\n\ndef test_b():\n    assert 1 == 1\n"
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	model, err := (helperParser{}).Parse(ctx, "t.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Tests) != 2 {
		t.Fatalf("got %d tests, want 2: %+v", len(model.Tests), model.Tests)
	}
	byName := map[string]TestFunc{}
	for _, tf := range model.Tests {
		byName[tf.Name] = tf
	}
	if !byName["test_a"].IsEmpty {
		t.Fatalf("test_a should be empty: %+v", byName["test_a"])
	}
	if byName["test_b"].IsEmpty || !byName["test_b"].HasAssert {
		t.Fatalf("test_b should have assert: %+v", byName["test_b"])
	}
	if byName["test_b"].QualName != "test_b" {
		t.Fatalf("qualname=%q", byName["test_b"].QualName)
	}
}

func TestHelperParser_QualNameInClass(t *testing.T) {
	if !hasPython() {
		t.Skip("no python/uv")
	}
	src := "" +
		"class TestA:\n" +
		"    def test_x(self):\n" +
		"        assert 1 == 1\n\n" +
		"class TestB:\n" +
		"    def test_x(self):\n" +
		"        assert 2 == 2\n"
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	model, err := (helperParser{}).Parse(ctx, "t.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Tests) != 2 {
		t.Fatalf("got %d tests: %+v", len(model.Tests), model.Tests)
	}
	quals := map[string]bool{}
	for _, tf := range model.Tests {
		quals[tf.QualName] = true
	}
	if !quals["TestA.test_x"] || !quals["TestB.test_x"] {
		t.Fatalf("want class-qualified names, got %v", quals)
	}
}

func TestHelperParser_TestNameFilter(t *testing.T) {
	if !hasPython() {
		t.Skip("no python/uv")
	}
	src := "" +
		"def Testimony():\n    pass\n\n" +
		"def TestFoo():\n    assert 1 == 1\n\n" +
		"def test_bar():\n    assert 1 == 1\n"
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	model, err := (helperParser{}).Parse(ctx, "t.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tf := range model.Tests {
		names[tf.Name] = true
	}
	if names["Testimony"] {
		t.Fatal("Testimony must not be a test name")
	}
	if !names["TestFoo"] || !names["test_bar"] {
		t.Fatalf("want TestFoo and test_bar, got %v", names)
	}
}

func TestHelperParser_AssertCompareFlags(t *testing.T) {
	if !hasPython() {
		t.Skip("no python/uv")
	}
	src := "def test_same():\n    assert x == x\n\ndef test_calls():\n    assert f(1) == f(1)\n"
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	model, err := (helperParser{}).Parse(ctx, "t.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	byQ := map[string]TestFunc{}
	for _, tf := range model.Tests {
		byQ[tf.QualName] = tf
	}
	a0 := byQ["test_same"].Asserts
	if len(a0) != 1 || a0[0].Kind != "compare" || a0[0].LeftIsCall || a0[0].RightIsCall {
		t.Fatalf("test_same asserts=%+v", a0)
	}
	a1 := byQ["test_calls"].Asserts
	if len(a1) != 1 || !a1[0].LeftIsCall || !a1[0].RightIsCall {
		t.Fatalf("test_calls asserts=%+v", a1)
	}
}

func TestHelperParser_Imports(t *testing.T) {
	if !hasPython() {
		t.Skip("no python/uv")
	}
	src := "" +
		"from mymodule import _helper, public\n" +
		"import pkg._internal\n" +
		"import _thread\n" +
		"def test_a():\n    assert 1 == 1\n"
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	model, err := (helperParser{}).Parse(ctx, "t.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Imports) != 3 {
		t.Fatalf("imports=%+v, want 3", model.Imports)
	}
	if model.Imports[0].Kind != "from" || len(model.Imports[0].Names) != 2 {
		t.Fatalf("from import: %+v", model.Imports[0])
	}
	if model.Imports[1].Kind != "import" || model.Imports[1].Names[0] != "pkg._internal" {
		t.Fatalf("submodule import: %+v", model.Imports[1])
	}
	if model.Imports[2].Names[0] != "_thread" {
		t.Fatalf("stdlib import: %+v", model.Imports[2])
	}
}

func TestBatch_Integration(t *testing.T) {
	if !hasPython() {
		t.Skip("no python/uv")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	b, err := StartBatch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.Close() }()

	m1, err := b.Parse(ctx, "a.py", []byte("def test_a():\n    pass\n"))
	if err != nil {
		t.Fatal(err)
	}
	m2, err := b.Parse(ctx, "b.py", []byte("def test_b():\n    assert 1 == 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(m1.Tests) != 1 || !m1.Tests[0].IsEmpty {
		t.Fatalf("m1=%+v", m1)
	}
	if len(m2.Tests) != 1 || !m2.Tests[0].HasAssert {
		t.Fatalf("m2=%+v", m2)
	}
}

func TestBatch_ParseError(t *testing.T) {
	if !hasPython() {
		t.Skip("no python/uv")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	b, err := StartBatch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.Close() }()

	_, err = b.Parse(ctx, "bad.py", []byte("def test_x(\n"))
	if err == nil {
		t.Fatal("want parse error")
	}
	// session must still work
	m, err := b.Parse(ctx, "ok.py", []byte("def test_ok():\n    assert 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Tests) != 1 {
		t.Fatalf("got %+v", m)
	}
}
