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
**When:** file contains `assert True`.

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
**When:** `pytest.skip` / `unittest.skip` / `pytest.fail("TODO")` / `assert False, "TODO"`.

Hit:

```python
def test_todo():
    pytest.skip()
```

Clean:

```python
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

**Severity:** warning  
**When:** more than 3 test functions and no `pytest.raises` / `pytest.warns` / `assertRaises`.

Hit:

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

**Severity:** warning  
**When:** test imports a private name (`from pkg import _foo`) or a private submodule (`import pkg._internal`). Top-level stdlib modules like `import _thread` are ignored.

Hit:

```python
from mymodule import _helper

def test_uses_private():
    assert _helper() == 1
```

Clean:

```python
from mymodule import helper

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
