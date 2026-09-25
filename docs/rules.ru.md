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
**Когда:** `assert True`, или unittest `assertTrue`/`assertFalse` со сравнением в аргументе (лучше `assertEqual`).

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
**Когда:** placeholder `pytest.skip()` / `pytest.fail("TODO")` / `assert False, "TODO"`, или skip/xfail-декоратор с TODO/FIXME в reason. Осознанный `@pytest.mark.skip(reason=...)` — зона `skip-without-reason`, не этого правила.

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

**Severity:** note (настраивается)  
**Когда (эвристика, по умолчанию):** больше `min-tests` (default 3) тест-функций и нет признаков негатива: `pytest.raises` / `warns` / `assertRaises`, status 4xx/5xx, `is None` / `is False` / `not`, `errors`/`detail`, `is_valid() is False`, негативные имена / id в parametrize.

**Coverage mode:** `[rules.only-happy-path] mode = "coverage"` и `coverage = "coverage.json"`, либо `--coverage path.json`. Эвристики по файлам отключаются; правило ищет непокрытые `raise` / `except` (и связанные missing branches) в не-тестовых файлах из JSON-отчёта coverage.py. По умолчанию выключен.

Настройки: `min-tests`, `negative-names`, `mode`, `coverage`.

Hit (эвристика):

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

**Severity:** note  
**Когда:** тест импортирует приватное имя (`from pkg import _foo`) или приватный submodule (`import pkg._internal`). Сообщает о **каждом** таком импорте в файле (AST import nodes). В message есть имя (`_foo`). Top-level stdlib вроде `import _thread` игнорируется, как и импорты из `tests.*` и относительные импорты sibling `test_*` / `conftest` helpers.

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

---

## fake-mock-assert

**Severity:** warning  
**Когда:** опечатка или несуществующий mock-assert (`assert_called_once_wiht`, `mock.called_once_with(...)`, `assert mock.called_once_with`).

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
**Когда:** `assert (x == 1, "msg")` — непустой tuple всегда truthy.

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
**Когда:** `pytest.raises(Exception)` / `BaseException` без `match=`, или тело raises больше одного statement.

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
**Когда:** `except:` / `except Exception` (или `BaseException`) без повторного raise.

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
**Когда:** все assert только внутри `for`/`async for`, чьё тело — одни assert (пустая коллекция → пустой проход).

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
**Когда:** в тесте только слабые assert: bare truthy (`assert x` / `assert obj.flag`), `is not None`, или `len(...)` vs `0`. **Не** ловит `assert f(...)`, `assert obj.method()`, `assert not collection` (явный булев контракт). `assert x is None` тоже не weak.

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
**Когда:** `m.return_value = X`, затем `assert sut() == X` или assert на `.return_value`.

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
**Когда:** `time.sleep` / `asyncio.sleep` внутри теста.

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
**Когда:** `@pytest.mark.skip` / `xfail` (или `unittest.skip`) без `reason` / `strict` / позиционной строки-причины.

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
**Когда:** два теста в одном файле имеют одинаковое тело после нормализации строковых/числовых литералов.

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
