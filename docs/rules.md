# Rules catalog

English | [Русский](rules.ru.md)

Each default rule below has a minimal **hit** (should report) and **clean** (should not) example. Heuristics are text- or AST-based; see the main [README](../README.md) for false-positive notes, **`--diff`** / **`--focus`** / **`--compare`**, confirmed density (`MinPrecisionForDisplay` = 0.3), deprecated Health Score (`MinPrecisionForGrade` = 0.15), and **tool errors** (`parse-error`).

Disable a rule: `--disable ID` or `disable = ["ID"]` in `.testscan.toml` / `[tool.testscan]`.  
Enable an opt-in rule: `--enable ID` or `enable = ["ID"]` (see **Optional rules** at the end).

**Severity vs score:** default severities below drive `--fail-on` and UI chips. **Confirmed** metrics count findings with precision ≥ 0.3 (after baseline / display filter). Deprecated Health Score only penalizes **error** and **warning** when precision ≥ 0.15; **note** findings and tool errors never affect the grade.

---

## empty-test

**Severity:** error  
**When:** empty `test_*` / `Test*` body (`pass` / docstring only); AST preferred, text fallback.

Hit:

```python
def test_vacuous():
    pass
```

Clean:

```python
def test_ok():
    assert 1 == 1
```

---

## no-assert

**Severity:** note  
**When:** test has no `assert` / `pytest.raises` / `pytest.warns` / assert-helper call.  
Skips no-raise style names (`accepts`, `does_not_raise`, …) and bodies that only call `validate_` / `check_` / `ensure_` / `assert_*`. Resolves one level into same-module helpers (`assert_*` / `_assert_*` / `check_*` or helpers that themselves assert).

Hit:

```python
def test_without_check():
    do_something()
```

Clean:

```python
def test_ok():
    assert result == 1

def _assert_ok(x):
    assert x

def test_via_helper():
    _assert_ok(run())
```

---

## assert-true

**Severity:** warning  
**When:** `assert True` / `assert 1` / `assert "…"`, or unittest `assertTrue`/`assertFalse` with a comparison argument (prefer `assertEqual`).

Hit:

```python
def test_assert_true():
    assert True
```

Clean:

```python
def test_ok():
    assert value is True
```

---

## mock-only-assert

**Severity:** note  
**When:** mock asserts (`assert_called` / `assert_has_calls`) and no plain value assert, and the SUT return value is used/ignored while only mocks are checked.  
Not flagged for procedural SUT (bare calls), constructor patches (`patch("…CamelCase")`), or boundary paths (`adapters/`, `clients/`, `*_client.py`, `orchestrator`, `admin`).

Hit:

```python
def test_mock_only():
    result = compute()
    mock.assert_called()
```

Clean:

```python
def test_ok():
    mock.assert_called()
    assert result == 1

def test_procedural():
    do_side_effect()
    mock.assert_called_once()
```

---

## todo-test

**Severity:** warning  
**When:** placeholder `pytest.skip()` / `pytest.fail("TODO")` / `assert False, "TODO"`, or skip/xfail decorator whose reason contains TODO/FIXME. Intentional `@pytest.mark.skip(reason=...)` is handled by `skip-without-reason`, not this rule.

Hit:

```python
def test_todo():
    pytest.skip()
```

Clean:

```python
@pytest.mark.skip(reason="flaky")
def test_ok():
    assert ready
```

---

## duplicate-test-name

**Severity:** error  
**When:** two `test_*` functions share a name (AST preferred).

Hit:

```python
def test_foo():
    assert 1 == 1

def test_foo():
    assert 2 == 2
```

Clean:

```python
def test_foo():
    assert 1 == 1

def test_bar():
    assert 2 == 2
```

---

## only-happy-path

**Severity:** note (configurable)  
**When (heuristic, default):** more than `min-tests` (default 3) tests **for the same SUT** (primary non-framework call) and no negative-path signals: `pytest.raises` / `warns` / `assertRaises`, status 4xx/5xx / `status.HTTP_4xx_*`, `is None` / `is False` / `not` / `not in` / `!=` / `== []|{}|""`, `errors`/`detail`, `is_valid() is False`, `side_effect` Exception, `caplog` WARNING/ERROR, negative names / parametrize ids (`403`, `rejects`, `blocked`, `gated`, `degrad`, `fallback`, …). Pure mappers/helpers without branches or exceptions are not flagged.

**Coverage mode:** set `[rules.only-happy-path] mode = "coverage"` and `coverage = "coverage.json"`, or pass `--coverage path.json`. Then the rule skips per-file heuristics and reports uncovered `raise` / `except` lines (and related missing branches) in non-test files from a coverage.py JSON report. Default: off.

Configurable: `min-tests`, `negative-names`, `mode`, `coverage`.

Hit (heuristic):

```python
def test_a():
    assert 1 == 1
def test_b():
    assert 2 == 2
def test_c():
    assert 3 == 3
def test_d():
    assert 4 == 4
```

Clean:

```python
def test_a():
    assert 1 == 1
def test_b():
    assert 2 == 2
def test_c():
    assert 3 == 3
def test_d():
    with pytest.raises(ValueError):
        bad()
```

---

## assert-equals-same

**Severity:** warning  
**When:** `assert <expr> == <expr>` with identical left and right text.

Hit:

```python
def test_same():
    assert x == x
```

Clean:

```python
def test_ok():
    assert x == y
```

---

## snapshot-only

**Severity:** warning  
**When:** snapshot tooling (`syrupy`, `assert_match_snapshot`, `== snapshot`, …) without a non-snapshot `assert `.

Hit:

```python
from syrupy import snapshot

def test_snapshot_only(snapshot):
    assert result == snapshot
```

Clean:

```python
from syrupy import snapshot

def test_ok(snapshot):
    assert result == snapshot
    assert result["status"] == "ok"
```

---

## overmocked-io

**Severity:** warning  
**When:** IO is patched (`builtins.open`, `pathlib`, `requests.`, `httpx.`, `urllib`) and only mock asserts remain.

Hit:

```python
from unittest.mock import patch

@patch("builtins.open")
def test_overmocked(mock_open):
    mock_open.assert_called()
```

Clean:

```python
from unittest.mock import patch

@patch("builtins.open")
def test_ok(mock_open):
    mock_open.assert_called()
    assert body == "data"
```

---

## test-imports-implementation-private

**Severity:** note  
**When:** test imports a private name (`from pkg import _foo`) or a private submodule (`import pkg._internal`). Emits **one finding per import statement** (not per name). Message frames the implementation module as a candidate for a public submodule. Skips `_UPPER_CASE` constants, top-level stdlib modules like `import _thread`, imports from `tests.*`, and relative imports of sibling `test_*` / `conftest` helpers.

Hit:

```python
from mymodule import _helper, _normalize

def test_uses_private():
    assert _helper() == 1
```

Clean:

```python
from mymodule import helper
from mymodule import _FOO_BAR  # UPPER const OK
import _thread
from tests.blueprint.test_api import _build_payload
from .test_helper_mod import _build

def test_ok():
    assert helper() == 1
```

---

## no-behavior-change

**Severity:** warning  
**When:** every assert is only `isinstance(...)` or `type(...) is` / `==` (no raises / value asserts).

Hit:

```python
def test_type_only():
    obj = Foo()
    assert isinstance(obj, Foo)
```

Clean:

```python
def test_ok():
    obj = Foo()
    assert isinstance(obj, Foo)
    assert obj.value == 42
```

---

## fake-mock-assert

**Severity:** warning  
**When:** typo or non-existent mock assert (`assert_called_once_wiht`, `mock.called_once_with(...)`, `assert mock.called_once_with`).

Hit:

```python
def test_fake_mock():
    mock.assert_called_once_wiht()
```

Clean:

```python
def test_ok():
    mock.assert_called_once_with()
```

---

## assert-tuple

**Severity:** warning  
**When:** `assert (x == 1, "msg")` — a non-empty tuple is always truthy.

Hit:

```python
def test_tuple():
    assert (x == 1, "msg")
```

Clean:

```python
def test_ok():
    assert x == 1, "msg"
```

---

## broad-raises

**Severity:** warning  
**When:** `pytest.raises(Exception)` / `BaseException` without `match=`, or raises body has more than one statement.  
Not flagged when the exception class is specific and the test checks `exc_info.value.<attr>` in/after the block. Prefer a concrete exception class over a bare `Exception`.

Hit:

```python
def test_broad():
    with pytest.raises(Exception):
        raise ValueError("boom")
```

Clean:

```python
def test_ok():
    with pytest.raises(ValueError, match="boom"):
        raise ValueError("boom")

def test_attr():
    with pytest.raises(ValueError) as exc_info:
        raise ValueError("boom")
    assert exc_info.value.args[0] == "boom"
```

---

## swallowed-exception

**Severity:** warning  
**When:** `except:` / `except Exception` (or `BaseException`) without a re-raise.

Hit:

```python
def test_swallowed():
    try:
        raise ValueError("x")
    except Exception:
        pass
```

Clean:

```python
def test_ok():
    try:
        raise ValueError("x")
    except Exception:
        raise
```

---

## assert-in-emptyable-loop

**Severity:** warning  
**When:** every assert lives inside a `for`/`async for` whose body is only asserts and the iterable is emptyable (computed name/call). Does **not** flag nonempty literals, `range(N)` with N≥1, `UPPER_CASE` / enum-const attributes, or loops preceded by `assert iter` / `assert len(iter) > 0`. Prefer a pre-loop `assert items, "…"` so an empty collection fails loudly.

Hit:

```python
def test_items():
    for item in items:
        assert item.ok
```

Clean:

```python
def test_ok():
    assert items
    for item in items:
        assert item == 1

def test_range():
    for i in range(10):
        assert i >= 0
```

---

## weak-assert

**Severity:** note  
**When:** the only asserts in a test are weak: bare truthy (`assert x` / `assert obj.flag`), `is not None`, or `len(...)` vs `0`; or shallow `rejects_*`/`fails_*`/`invalid*` checks without error code/message; or GET-like tests with only `status_code == 200`. Does **not** flag `len(x) == N` (N≠0), `assert result.is_valid` in `accepts_*`/`passes_*`/`valid*` tests, `assert f(...)`, `assert obj.method()`, or `assert not collection`. `assert x is None` is also not weak.

Hit:

```python
def test_weak():
    assert result is not None
```

Clean:

```python
def test_ok():
    assert result == 42

def test_bool_call():
    assert obj.is_valid()
    assert not errors

def test_accepts_payload():
    assert result.is_valid

def test_len_exact():
    assert len(ids) == 200
```

---

## mock-tautology

**Severity:** note  
**When:** the assert side is the mock itself (`m()`, `m.return_value`, `m.attr`) echoing `m.return_value = X`, or `patch("mod.func", return_value=X)` then `assert mod.func() == X`.  
Not flagged when a real SUT call returns a value that happens to equal a mock `return_value`.

Hit:

```python
def test_tautology():
    m.return_value = 42
    assert m() == 42
```

Clean:

```python
def test_ok():
    m.return_value = 42
    assert sut() == 7

def test_sut_uses_dependency():
    m.return_value = None
    assert ensure.execute() is None
```

---

## sleep-in-test

**Severity:** warning  
**When:** `time.sleep` / `asyncio.sleep` in a test.  
**Note:** `--disable sleep-in-test` does **not** disable `wall-clock-in-test` (separate rule).

Hit:

```python
def test_sleep():
    time.sleep(1)
    assert True
```

Clean:

```python
def test_ok():
    assert True
```

---

## wall-clock-in-test

**Severity:** note  
**When:** `datetime.now` / `date.today` without freezegun / time-machine / `freeze_time`.

Hit:

```python
def test_wall_clock():
    now = datetime.now()
    assert process(now) is not None
```

Clean:

```python
@freeze_time("2024-01-01")
def test_frozen_now():
    assert datetime.now().year == 2024
```

---

## skip-without-reason

**Severity:** warning  
**When:** `@pytest.mark.skip` / `xfail` (or `unittest.skip`) without `reason` / `strict` / positional reason string.

Hit:

```python
@pytest.mark.skip
def test_skipped():
    assert True
```

Clean:

```python
@pytest.mark.skip(reason="flaky")
def test_ok():
    assert True
```

---

## near-duplicate-test

**Severity:** note  
**When:** two or more tests in the same file share the same body after normalizing string/number literals, **and** they share polarity, SUT call, and significant assert literals (enums / HTTP statuses). Opposite accept/reject pairs, different SUT calls, and different enum expectations are not flagged. Clusters of 3+ emit one finding with a `@pytest.mark.parametrize` sketch.

Hit:

```python
def test_a():
    x = 1
    assert x == 1

def test_b():
    x = 2
    assert x == 2
```

Clean:

```python
def test_accepts_item():
    out = process(item)
    assert out

def test_rejects_item():
    out = process(item)
    assert out

def test_managed_identity():
    assert resolve_auth() == "MANAGED_IDENTITY"

def test_default_chain():
    assert resolve_auth() == "DEFAULT_CHAIN"
```

---

## name-body-mismatch

**Severity:** note  
**When:** the test name implies a negative/error case (`rejects_*`, `*_404`, `fails_*`, …) but the body has no matching negative signal (`pytest.raises`, 4xx/5xx assert, `assert not`, …).

Hit:

```python
def test_rejects_missing_item():
    result = process(item)
    assert result.is_valid
```

Clean:

```python
def test_rejects_missing_item():
    with pytest.raises(ValueError):
        process(item)
```

---

## self-patched-sut

**Severity:** warning  
**When:** the test patches the SUT under test and asserts the patch (e.g. `@patch("mod.compute", return_value=42)` then `assert mod.compute() == 42`).

Hit:

```python
@patch("mod.compute", return_value=42)
def test_self_patch():
    assert mod.compute() == 42
```

Clean:

```python
@patch("mod.dependency", return_value=7)
def test_dep_patch():
    assert mod.compute() == 7
```

---

## expected-recomputed

**Severity:** warning  
**When:** the right-hand side of a compare assert calls the same SUT/helper already used to produce the left-hand value (recomputing the expected value). Determinism checks `f(x) == f(x)` are not flagged.

Hit:

```python
def test_recomputed():
    got = compute(3)
    assert got == compute(3)
```

Clean:

```python
def test_literal_expected():
    got = compute(3)
    assert got == 6
```

```python
def test_determinism():
    assert plan(build()) == plan(build())
```

---

## commented-assert

**Severity:** note  
**When:** a line is a commented-out `assert` / `self.assert*` (`# assert …`).

Hit:

```python
def test_commented():
    value = run()
    # assert value == 1
    assert value is not None
```

Clean:

```python
def test_ok():
    # setup note: assert helpers live below
    assert value == 1
```

---

## overbroad-equality

**Severity:** note  
**When:** an assert compares against a huge dict/list/tuple literal (long text or many commas). Skips API-contract compares where one side is `response.data` / `resp.data` / a `*.json()`-like response body. Often a valid snapshot otherwise; review whether focused field checks would be clearer.

Hit:

```python
def test_huge_payload():
    assert payload == {"a": 1, "b": 2, /* … many keys … */}
```

Clean:

```python
def test_field_checks():
    data = resp.json()
    assert data["id"] == 1

def test_api_contract():
    assert response.data == {"id": 1, "name": "x", /* … */}
```

---

## Optional rules (opt-in)

Off by default. Enable with `--enable ID` or `enable = ["ID"]` in `[tool.testscan]`. Configure under `[rules.<id>]`. Profile paths belong in the consumer’s `pyproject.toml` — testscan does not hardcode `app/`.

### error-contract-assert

**When:** test asserts a non-2xx `status_code` / `HTTP_4xx|5xx` but does not assert the configured error-code path (default `errors[].code`).  
**Options:** `error-code-path`, `error-status-only` (default true).

### raises-without-check

**When:** `pytest.raises(...)` without `match=` and without checking a domain error attribute (default `.code` / `exc_info.value.code`).  
**Options:** `error-attr`, `exception-classes` (optional allowlist).

### missing-mirror-test

**When:** a source file matching `source-glob` has no file at `mirror-template` (`{m}` = stem, `{x}` = path segments). Finding is on the **source** file. Does not check Django imports.  
**Options:** `source-glob`, `mirror-template`.

### rbac-mutation-guard

**When:** a test looks RBAC/permission-related (name/fixtures cues) and performs a mutating action (HTTP POST/PUT/PATCH/DELETE or create/update/delete-style calls) but does not assert forbid (401/403 / Permission raises / forbidden signals).  
**Options:** `name-cues`, `mutating-methods`, `forbidden-signals`.