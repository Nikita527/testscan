package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/rules"
	"github.com/Nikita527/testscan/scan"
)

// dfRun parses src (a test module of project `app`) with the real AST helper and
// returns the data-flow result per test name.
func dfRun(t *testing.T, src string) map[string]*rules.Dataflow {
	t.Helper()
	root := sutProject(t)
	path := filepath.Join(root, "tests", "test_df.py")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	model, err := parse.BatchParser{Batch: sharedBatch}.Parse(context.Background(), path, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	file := scan.File{Path: path, Content: []byte(src), ProjectRoot: root, ModelOK: true, Model: model}
	ctx := rules.NewSUTContext(file, model)
	out := map[string]*rules.Dataflow{}
	for _, tf := range model.Tests {
		out[tf.Name] = rules.AnalyzeDataflow(file, model, ctx, tf)
	}
	return out
}

const dfHeader = "import pytest\nfrom unittest.mock import MagicMock, patch\nfrom app.service import process\nfrom app.models import Order\n\n"

func TestDataflow_Dependency(t *testing.T) {
	cases := []struct {
		name string
		body string
		dep  bool // some check depends on the SUT
	}{
		{"result", "def test_x():\n    r = process(1)\n    assert r == 2\n", true},
		{"unrelated_literal", "def test_x():\n    process(1)\n    e = 2\n    assert e == 2\n", false},
		{"attribute_chain", "def test_x():\n    r = process(1)\n    v = r.items[0].name\n    assert v\n", true},
		{"tuple_unpack", "def test_x():\n    a, b = process(1), 2\n    assert b == a\n", true},
		{"for_target", "def test_x():\n    for v in process(1):\n        assert v\n", true},
		{"comprehension_fstring", "def test_x():\n    rows = [x for x in process(1)]\n    s = f'{rows}'\n    assert s\n", true},
		{"aug_assign", "def test_x():\n    n = 0\n    n += process(1)\n    assert n == 3\n", true},
		{"call_with_tainted_arg", "def test_x():\n    r = process(1)\n    out = str(r)\n    assert out\n", true},
		{"refresh_from_db", "def test_x(acct):\n    process(1)\n    acct.refresh_from_db()\n    assert acct.active\n", true},
		{"refresh_before_sut", "def test_x(acct):\n    acct.refresh_from_db()\n    process(1)\n    assert acct.active is False\n", false},
		{"objects_after_sut", "def test_x(db):\n    process(1)\n    assert Order.objects.filter(pk=1).count() == 1\n", true},
		{"client_response", "def test_x(client):\n    resp = client.post('/x', {})\n    assert resp.status_code == 201\n", true},
		{"client_helper", "def _post(c, body):\n    return c.post('/x', body)\n\n\ndef test_x(api_client):\n    resp = _post(api_client, {})\n    assert resp.status_code == 201\n", true},
		{"fixture_side_effect", "def test_x(cart):\n    process(cart)\n    assert cart.touched\n", true},
		{"nested_fake_arg", "def test_x():\n    state = {}\n    process(Wrap(state))\n    assert state\n", true},
		{"unrelated_fixture", "def test_x(settings):\n    process(1)\n    assert settings.DEBUG is False\n", false},
		{"raises_block_with_sut", "def test_x():\n    with pytest.raises(ValueError):\n        process(-1)\n", true},
		{"raises_block_without_sut", "def test_x():\n    process(1)\n    with pytest.raises(ZeroDivisionError):\n        1 / 0\n", false},
		{"raises_as_exc", "def test_x():\n    with pytest.raises(ValueError) as exc:\n        process(-1)\n    assert exc.value.args\n", true},
		{"except_as", "def test_x():\n    try:\n        process(-1)\n    except ValueError as exc:\n        assert 'x' in str(exc)\n", true},
		{"helper_tainted", "def test_x():\n    r = process(1)\n    assert_valid(r)\n", true},
		{"helper_unrelated", "def test_x():\n    process(1)\n    assert_valid({'a': 1})\n", false},
		{"check_prefix", "def test_x():\n    check_it(process(1))\n", true},
		{"unittest_assert", "def test_x(self):\n    self.assertEqual(process(1), 2)\n", true},
		{"callback_mutation", "def test_x(monkeypatch):\n    built = []\n    monkeypatch.setattr('a.b', lambda: built.append(1))\n    process(1)\n    assert built == [1]\n", true},
		{"query_count_with_as", "def test_x(client):\n    with CaptureQueriesContext(conn) as ctx:\n        client.get('/x')\n    assert len(ctx) < 5\n", true},
		{"project_constant", "def test_x():\n    process(1)\n    assert Order.KINDS == ('a',)\n", true},
		{"observer_helper_after_sut", "def _rows():\n    return []\n\n\ndef test_x():\n    process(1)\n    rows = _rows()\n    assert rows == []\n", true},
		{"caplog_after_sut", "def test_x(caplog):\n    process(1)\n    assert 'done' in caplog.text\n", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dfRun(t, dfHeader+tc.body)["test_x"]
			if got == nil {
				t.Fatal("test_x not found")
			}
			if !got.HasSUT {
				t.Fatalf("expected a SUT call")
			}
			if (got.DependentChecks > 0) != tc.dep {
				t.Fatalf("dependent checks=%d, want dependent=%v", got.DependentChecks, tc.dep)
			}
			if !got.Test.HasSUTCall {
				t.Fatalf("HasSUTCall not annotated")
			}
		})
	}
}

func TestDataflow_NoSUTCallNoClaim(t *testing.T) {
	got := dfRun(t, dfHeader+"def test_x(svc):\n    r = svc.run()\n    assert r\n")["test_x"]
	if got.HasSUT || got.NoDependentCheck() {
		t.Fatalf("no SUT call: must not claim a missing dependency (HasSUT=%v)", got.HasSUT)
	}
}

func TestDataflow_AssertAnnotation(t *testing.T) {
	src := dfHeader + "def test_x():\n    r = process(1)\n    e = 4\n    assert r == 2\n    assert e == 4\n"
	got := dfRun(t, src)["test_x"].Test
	if len(got.Asserts) != 2 {
		t.Fatalf("asserts=%d", len(got.Asserts))
	}
	if !got.Asserts[0].DependsOnSUT || got.Asserts[1].DependsOnSUT {
		t.Fatalf("depends_on_sut = %v/%v, want true/false", got.Asserts[0].DependsOnSUT, got.Asserts[1].DependsOnSUT)
	}
	if !got.AnySUTDependentAssert || !got.HasSUTCall {
		t.Fatalf("summary not annotated: %+v", got)
	}
}

func TestDataflow_MockWiring(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		wired bool
	}{
		{"injected_arg", "def test_x():\n    m = MagicMock()\n    process(m)\n    m.send.assert_called_once()\n", true},
		{"attribute_assign", "def test_x():\n    m = MagicMock()\n    obj.repo = m\n    m.save.assert_called()\n", true},
		{"self_called_only", "def test_x():\n    m = MagicMock()\n    m.send(1)\n    m.send.assert_called_once_with(1)\n", false},
		{"sut_but_not_passed", "def test_x():\n    m = MagicMock()\n    process(1)\n    m.assert_called()\n", false},
		{"patched_with_activity", "def test_x():\n    with patch('app.service.send') as send:\n        process(1)\n    send.assert_called_once()\n", true},
		{"fixture_mock_with_activity", "def test_x(mock_repo):\n    process(1)\n    mock_repo.save.assert_called()\n", true},
		{"fixture_mock_no_activity", "def test_x(mock_repo):\n    mock_repo.save.assert_called()\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dfRun(t, dfHeader+tc.body)["test_x"]
			if len(got.MockAsserts) != 1 {
				t.Fatalf("mock asserts=%d", len(got.MockAsserts))
			}
			if got.MockAsserts[0].Wired != tc.wired {
				t.Fatalf("wired=%v, want %v", got.MockAsserts[0].Wired, tc.wired)
			}
			if len(got.Test.Asserts) == 1 && got.Test.Asserts[0].DependsOnSUT != tc.wired {
				t.Fatalf("assert.DependsOnSUT=%v, want %v", got.Test.Asserts[0].DependsOnSUT, tc.wired)
			}
		})
	}
}

func TestDataflow_MockTautology(t *testing.T) {
	cases := []struct {
		name string
		body string
		hit  bool
	}{
		{"direct_call", "def test_x():\n    m = MagicMock()\n    m.return_value = 42\n    assert m() == 42\n", true},
		{"derived_value", "def test_x():\n    m = MagicMock()\n    m.return_value = 42\n    v = m()\n    process(1)\n    assert v == 42\n", true},
		{"sut_result", "def test_x():\n    m = MagicMock()\n    m.return_value = 42\n    r = process(m)\n    assert r == m.return_value\n", false},
		{"call_args_of_wired_mock", "def test_x():\n    m = MagicMock()\n    m.return_value = 1\n    process(m)\n    kw = m.call_args.kwargs\n    assert kw['a'] == 1\n", false},
		{"object_under_test_configured", "def test_x():\n    t = build()\n    t.cache.get.side_effect = lambda *a: []\n    assert t.allow() is True\n", false},
		{"unrelated_true", "def test_x():\n    m = MagicMock()\n    m.return_value = True\n    flag = compute()\n    assert flag is True\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dfRun(t, dfHeader+tc.body)["test_x"]
			hit := false
			for _, v := range got.Tautology {
				hit = hit || v
			}
			if hit != tc.hit {
				t.Fatalf("tautology=%v, want %v", hit, tc.hit)
			}
		})
	}
}

func ruleRun(t *testing.T, rule scan.Rule, src string) []scan.Finding {
	t.Helper()
	root := sutProject(t)
	path := filepath.Join(root, "tests", "test_rule.py")
	model, err := parse.BatchParser{Batch: sharedBatch}.Parse(context.Background(), path, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return rule.Check(scan.File{Path: path, Content: []byte(src), ProjectRoot: root, ModelOK: true, Model: model})
}

func TestNoAssert_DataflowSemantics(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want int
	}{
		{"unrelated_assert", dfHeader + "def test_a():\n    process(1)\n    assert 1 == 1\n", 1},
		{"dependent_assert", dfHeader + "def test_a():\n    assert process(1) == 2\n", 0},
		{"no_sut_unrelated_assert", dfHeader + "def test_a():\n    x = 1\n    assert x == 1\n", 0},
		{"zero_asserts_reported", dfHeader + "def test_a():\n    process(1)\n", 1},
		{"zero_asserts_smoke", dfHeader + "def test_process_smoke():\n    process(1)\n", 0},
		{"zero_asserts_no_error_name", dfHeader + "def test_process_no_error():\n    process(1)\n", 0},
		{"zero_asserts_comment", dfHeader + "def test_a():\n    # must not raise\n    process(1)\n", 0},
		{"unrelated_assert_intentional_name", dfHeader + "def test_errors_does_not_propagate():\n    process(1)\n    assert 1 == 1\n", 0},
		{"mock_only_left_to_other_rule", dfHeader + "def test_a():\n    m = MagicMock()\n    process(1)\n    m.assert_called()\n", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ruleRun(t, rules.NewNoAssert(), tc.src); len(got) != tc.want {
				t.Fatalf("findings=%d, want %d: %v", len(got), tc.want, got)
			}
		})
	}
}

func TestMockOnlyAssert_DataflowSemantics(t *testing.T) {
	run := func(body string) []scan.Finding {
		return ruleRun(t, rules.NewMockOnlyAssert(), dfHeader+body)
	}
	if got := run("def test_a():\n    m = MagicMock()\n    m.send(1)\n    m.send.assert_called_once_with(1)\n"); len(got) != 1 {
		t.Fatalf("self-called mock must hit, got %v", got)
	}
	if got := run("def test_a():\n    m = MagicMock()\n    process(m)\n    m.send.assert_called_once()\n"); len(got) != 0 {
		t.Fatalf("injected mock must be clean, got %v", got)
	}
	if got := run("def test_a():\n    m = MagicMock()\n    process(m)\n    m.send.assert_called_once()\n    assert m.x == 1\n"); len(got) != 0 {
		t.Fatalf("mock + plain assert is not mock-only, got %v", got)
	}
}

func TestMockTautology_DataflowSemantics(t *testing.T) {
	src := dfHeader + "def test_a():\n    m = MagicMock()\n    m.return_value = 42\n    assert m() == 42\n"
	if got := ruleRun(t, rules.NewMockTautology(), src); len(got) != 1 {
		t.Fatalf("want 1 finding, got %v", got)
	}
}
