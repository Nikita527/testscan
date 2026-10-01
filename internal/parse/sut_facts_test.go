package parse

import (
	"context"
	"testing"
	"time"
)

func TestHelperParser_SUTAndClockFacts(t *testing.T) {
	if !hasPython() {
		t.Skip("no python/uv")
	}
	src := `import json
import datetime as dt
from app.service import Meta as M, run

def _rows(): pass

def test_a():
    payload = json.dumps({})
    payload.encode("utf-8")
    "x".join([])
    json.dumps({"k": M(1)})
    today = dt.date.today()
    run(valid_from=today)
    assert dt.datetime.now(dt.timezone.utc)
`
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	model, err := (helperParser{}).Parse(ctx, "t.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Tests) != 1 {
		t.Fatalf("tests: %+v", model.Tests)
	}
	tf := model.Tests[0]
	if tf.VarOrigins["payload"] != "json.dumps" {
		t.Errorf("var_origins: %v", tf.VarOrigins)
	}
	byName := map[string]Call{}
	for _, c := range tf.Calls {
		byName[c.Name] = c
	}
	if !byName["join"].RecvLiteral {
		t.Errorf("join should have literal receiver: %+v", byName["join"])
	}
	if !byName["M"].ArgOfCall {
		t.Errorf("M(1) is an argument of json.dumps: %+v", byName["M"])
	}
	today := byName["dt.date.today"]
	if today.Argc != 0 || len(today.Flow) != 1 || today.Flow[0] != "call_arg" {
		t.Errorf("today flow: %+v", today)
	}
	if now := byName["dt.datetime.now"]; now.Argc != 1 {
		t.Errorf("now(tz) argc: %+v", now)
	}
	var sawM bool
	for _, imp := range model.Imports {
		for _, b := range imp.Bindings {
			if b.Local == "M" && b.Module == "app.service" && b.Name == "Meta" && !b.Stdlib {
				sawM = true
			}
			if b.Local == "json" && !b.Stdlib {
				t.Errorf("json must be stdlib: %+v", b)
			}
		}
	}
	if !sawM {
		t.Errorf("alias binding missing: %+v", model.Imports)
	}
	if len(model.ModuleDefs) == 0 || model.ModuleDefs[0] != "_rows" {
		t.Errorf("module_defs: %v", model.ModuleDefs)
	}
}
