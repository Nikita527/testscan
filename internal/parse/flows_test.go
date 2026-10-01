package parse_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/Nikita527/testscan/internal/parse"
)

func parseSrc(t *testing.T, src string) parse.TestFunc {
	t.Helper()
	b, err := parse.StartBatch(context.Background())
	if err != nil {
		t.Skipf("python helper unavailable: %v", err)
	}
	defer func() { _ = b.Close() }()
	m, err := b.Parse(context.Background(), "t.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Tests) != 1 {
		t.Fatalf("tests=%d", len(m.Tests))
	}
	return m.Tests[0]
}

func TestHelperExportsDefUseFacts(t *testing.T) {
	src := `def test_x(client):
    built = []
    r = f(1)
    resp = client.post("/x", data=r)
    self.res = g(r)
    with cap(conn) as ctx:
        client.get("/y")
    try:
        h()
    except ValueError as exc:
        assert "x" in str(exc)
    k = adf.t.call_args.kwargs
    run(lambda: built.append(1))
    assert resp.status_code == 200
    assert self.res
`
	tf := parseSrc(t, src)

	byTarget := map[string]parse.Flow{}
	for _, f := range tf.Flows {
		for _, tg := range f.Targets {
			byTarget[tg] = f
		}
	}
	if f := byTarget["resp"]; !reflect.DeepEqual(f.Reads, []string{"client", "r"}) || len(f.Calls) != 1 {
		t.Fatalf("resp flow=%+v", f)
	}
	if f := byTarget["self.res"]; f.Kind != "assign" || !reflect.DeepEqual(f.Reads, []string{"g", "r"}) {
		t.Fatalf("self.res flow=%+v (self attributes are tracked as paths)", f)
	}
	if f := byTarget["ctx"]; f.Kind != "with" || f.EndLine != 7 {
		t.Fatalf("with flow=%+v", f)
	}
	if f := byTarget["exc"]; f.Kind != "except" || f.EndLine == 0 {
		t.Fatalf("except flow=%+v", f)
	}
	if f := byTarget["k"]; !reflect.DeepEqual(f.MState, []string{"adf"}) {
		t.Fatalf("mstate flow=%+v", f)
	}
	if !reflect.DeepEqual(tf.NestedWrites, []string{"built"}) {
		t.Fatalf("nested writes=%v", tf.NestedWrites)
	}
	if len(tf.Asserts) != 3 {
		t.Fatalf("asserts=%d", len(tf.Asserts))
	}
	var last parse.Assert
	for _, a := range tf.Asserts {
		if a.Text == "self.res" {
			last = a
		}
	}
	if !reflect.DeepEqual(last.Reads, []string{"self.res"}) {
		t.Fatalf("self.res assert reads=%v", last.Reads)
	}
	for _, c := range tf.Calls {
		if c.Name == "client.post" && !reflect.DeepEqual(c.Reads, []string{"r"}) {
			t.Fatalf("client.post reads=%v", c.Reads)
		}
	}
}
