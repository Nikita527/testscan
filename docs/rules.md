# Rules catalog

English | [Русский](rules.ru.md)

Each default rule below has a minimal **hit** (should report) and **clean** (should not) example. Heuristics are text- or AST-based; see the main [README](../README.md) for false-positive notes.

Disable a rule: `--disable ID` or `disable = ["ID"]` in `.testscan.toml` / `[tool.testscan]`.

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

**Severity:** error  
**When:** no `assert` / `pytest.raises` / `pytest.warns` in the file.

Hit:

```python
def test_without_check():
    do_something()
```

Clean:

```python
def test_ok():
    assert result == 1
```

---

## assert-true

**Severity:** warning  
**When:** `assert True`, or unittest `assertTrue`/`assertFalse` with a comparison argument (prefer `assertEqual`).

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

**Severity:** warning  
**When:** mock asserts (`assert_called` / `assert_has_calls`) and no plain `assert `.

Hit:

```python
def test_mock_only():
    mock.assert_called()
```

Clean:

```python
def test_ok():
    mock.assert_called()
    assert result == 1
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
**When (heuristic, default):** more than `min-tests` (default 3) test functions and no negative-path signals: `pytest.raises` / `warns` / `assertRaises`, status 4xx/5xx, `is None` / `is False` / `not`, `errors`/`detail`, `is_valid() is False`, negative names / parametrize ids.

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
**When:** test imports a private name (`from pkg import _foo`) or a private submodule (`import pkg._internal`). Reports **every** matching import in the file (AST import nodes). Message includes the private name. Top-level stdlib modules like `import _thread` are ignored, as are imports from `tests.*` packages and relative imports of sibling `test_*` / `conftest` helpers.

Hit:

```python
from mymodule import _helper

def test_uses_private():
    assert _helper() == 1
```

Clean:

```python
from mymodule import helper
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
**When:** every assert lives inside a `for`/`async for` whose body is only asserts (empty collection → vacuous pass).

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
```

---

## weak-assert

**Severity:** note  
**When:** the only asserts in a test are weak: bare truthy (`assert x` / `assert obj.flag`), `is not None`, or `len(...)` vs `0`. Does **not** flag `assert f(...)`, `assert obj.method()`, or `assert not collection` (explicit boolean contracts). `assert x is None` is also not weak.

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
```

---

## mock-tautology

**Severity:** warning  
**When:** `m.return_value = X` then `assert sut() == X` or an assert on `.return_value`.

Hit:

```python
def test_tautology():
    m.return_value = 42
    assert sut() == 42
```

Clean:

```python
def test_ok():
    m.return_value = 42
    assert sut() == 7
```

---

## sleep-in-test

**Severity:** warning  
**When:** `time.sleep` / `asyncio.sleep` inside a test.

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
**When:** two tests in the same file share the same body after normalizing string/number literals.

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
def test_a():
    assert foo() == 1

def test_b():
    assert bar() == 2
```
