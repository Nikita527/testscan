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
