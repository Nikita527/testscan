# Каталог правил

[English](rules.md) | Русский

У каждого правила из `Default()` есть минимальный пример **hit** (должен сработать) и **clean** (не должен). Эвристики текстовые или AST — см. [README.ru.md](../README.ru.md) про ложные срабатывания.

Отключить правило: `--disable ID` или `disable = ["ID"]` в `.testscan.toml` / `[tool.testscan]`.

---

## empty-test

**Severity:** error  
**Когда:** пустое тело `test_*` / `Test*` (`pass` / только docstring); предпочтительно AST.

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
**Когда:** в файле нет `assert` / `pytest.raises` / `pytest.warns`.

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
**Когда:** есть `assert True`.

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
**Когда:** есть mock-assert (`assert_called` / `assert_has_calls`) и нет обычного `assert `.

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
**Когда:** `pytest.skip` / `unittest.skip` / `pytest.fail("TODO")` / `assert False, "TODO"`.

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
**Когда:** два `test_*` с одинаковым именем (предпочтительно AST).

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
**Когда:** больше 3 тест-функций и нет `pytest.raises` / `pytest.warns` / `assertRaises`.

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
**Когда:** `assert <expr> == <expr>` с одинаковым текстом слева и справа.

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
**Когда:** snapshot-инструменты (`syrupy`, `assert_match_snapshot`, `== snapshot`, …) без обычного (не-snapshot) `assert `.

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
**Когда:** пропатчен IO (`builtins.open`, `pathlib`, `requests.`, `httpx.`, `urllib`) и остались только mock-assert.

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
**Когда:** тест импортирует приватное имя (`from pkg import _foo`) или приватный submodule (`import pkg._internal`). Top-level stdlib вроде `import _thread` игнорируется.

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
**Когда:** все assert — только `isinstance(...)` или `type(...) is` / `==` (нет raises / проверок значений).

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
