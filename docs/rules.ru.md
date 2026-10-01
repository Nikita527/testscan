# Каталог правил

[English](rules.md) | Русский

У каждого правила из `Default()` есть минимальный пример **hit** (должен сработать) и **clean** (не должен). Эвристики текстовые или AST — см. [README.ru.md](../README.ru.md) про ложные срабатывания, **`--diff`** / **`--focus`** / **`--compare`**, confirmed density (`MinPrecisionForDisplay` = 0.3), deprecated Health Score (`MinPrecisionForGrade` = 0.15) и **tool errors** (`parse-error`).

Отключить правило: `--disable ID` или `disable = ["ID"]` в `.testscan.toml` / `[tool.testscan]`.  
Включить opt-in: `--enable ID` или `enable = ["ID"]` (см. **Optional** в конце).

**Severity vs score:** severity ниже влияет на `--fail-on` и чипы в UI. **Confirmed** считает findings с precision ≥ 0.3 (после baseline / display-фильтра). Deprecated Health Score штрафует только **error**/**warning** при precision ≥ 0.15; **note** и tool errors на grade не влияют.

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

**Severity:** note  
**Когда:** в тесте нет `assert` / `pytest.raises` / `pytest.warns` / вызова assert-хелпера.  
Пропускает no-raise имена (`accepts`, `does_not_raise`, …) и тела из вызовов `validate_` / `check_` / `ensure_` / `assert_*`. Резолвит на 1 уровень хелперы того же модуля (`assert_*` / `_assert_*` / `check_*` или функции с assert внутри).

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
**Когда:** `assert True` / `assert 1` / `assert "…"`, или unittest `assertTrue`/`assertFalse` со сравнением в аргументе (лучше `assertEqual`).

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
**Когда:** есть mock-assert (`assert_called` / `assert_has_calls`) и нет обычного value-assert, при этом результат SUT используется/игнорируется, а проверяются только моки.  
Не флагает процедурный SUT (bare-вызовы), патч конструктора (`patch("…CamelCase")`) и boundary-пути (`adapters/`, `clients/`, `*_client.py`, `orchestrator`, `admin`).

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
**Когда (эвристика, по умолчанию):** больше `min-tests` (default 3) тестов **на один SUT** (первый не-framework вызов) и нет признаков негатива: `pytest.raises` / `warns` / `assertRaises`, status 4xx/5xx / `status.HTTP_4xx_*`, `is None` / `is False` / `not` / `not in` / `!=` / `== []|{}|""`, `errors`/`detail`, `is_valid() is False`, `side_effect` Exception, `caplog` WARNING/ERROR, негативные имена / id в parametrize (`403`, `rejects`, `blocked`, `gated`, `degrad`, `fallback`, …). Чистые mapper/helper без ветвлений и исключений не флагаются.

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
**Когда:** тест импортирует приватное имя (`from pkg import _foo`) или приватный submodule (`import pkg._internal`). Одна находка на **import-statement** (не на каждое имя). Message: модуль-кандидат на выделение публичного подмодуля. Пропускает константы `_UPPER_CASE`, top-level stdlib вроде `import _thread`, импорты из `tests.*` и относительные импорты sibling `test_*` / `conftest` helpers.

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
Не флагает, если класс исключения конкретный и есть проверка `exc_info.value.<attr>` в блоке / сразу после. Лучше указать конкретный класс, а не голый `Exception`.

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
**Когда:** все assert только внутри `for`/`async for`, чьё тело — одни assert, и итерируемое потенциально пустое (вычисляемое имя/вызов). **Не** флагает непустые литералы, `range(N)` при N≥1, `UPPER_CASE` / атрибуты enum-констант, и циклы с `assert iter` / `assert len(iter) > 0` перед ними. Лучше pre-loop `assert items, "…"`, чтобы пустая коллекция падала явно.

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
**Когда:** в тесте только слабые assert: bare truthy (`assert x` / `assert obj.flag`), `is not None`, или `len(...)` vs `0`; либо поверхностные `rejects_*`/`fails_*`/`invalid*` без кода/сообщения ошибки; либо GET-подобные тесты только с `status_code == 200`. **Не** ловит `len(x) == N` (N≠0), `assert result.is_valid` в `accepts_*`/`passes_*`/`valid*`, `assert f(...)`, `assert obj.method()`, `assert not collection`. `assert x is None` тоже не weak.

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
**Когда:** сторона assert — сам мок (`m()`, `m.return_value`, `m.attr`) и она повторяет `m.return_value = X`, либо `patch("mod.func", return_value=X)` и затем `assert mod.func() == X`.  
Не флагает, когда реальный вызов SUT возвращает значение, совпадающее с `return_value` зависимости.

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
**When:** `time.sleep` / `asyncio.sleep` в тесте.  
**Note:** `--disable sleep-in-test` **не** отключает `wall-clock-in-test`.

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
**When:** `datetime.now` / `date.today` без freezegun / time-machine / `freeze_time`.

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
**Когда:** два или более теста в одном файле имеют одинаковое тело после нормализации строковых/числовых литералов **и** совпадают по полярности, SUT-вызову и значимым литералам assert (enum / HTTP-статусы). Пары accept/reject, разные SUT и разные enum-ожидания не флагуются. Кластеры 3+ — одна находка с наброском `@pytest.mark.parametrize`.

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
**Когда:** имя теста намекает на негативный/ошибочный кейс (`rejects_*`, `*_404`, `fails_*`, …), а в теле нет соответствующего сигнала (`pytest.raises`, assert на 4xx/5xx, `assert not`, …).

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
**Когда:** тест патчит сам SUT и проверяет патч (например `@patch("mod.compute", return_value=42)` и затем `assert mod.compute() == 42`).

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
**Когда:** правая часть compare-assert вызывает тот же SUT/хелпер, которым уже получили левую сторону (ожидаемое пересчитывается). Проверки детерминизма `f(x) == f(x)` не флагируются.

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
**Когда:** строка — закомментированный `assert` / `self.assert*` (`# assert …`).

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
**Когда:** assert сравнивает с огромным литералом dict/list/tuple. Пропускает контракт API, где одна сторона — `response.data` / `resp.data` / `*.json()`-подобный body.

Hit:

```python
def test_huge_payload():
    assert payload == {"a": 1, "b": 2, /* … много ключей … */}
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

## Optional-правила (opt-in)

Выключены по умолчанию. Включение: `--enable ID` или `enable = ["ID"]`. Опции — в `[rules.<id>]`. Пути профиля только в `pyproject.toml` потребителя.

### error-contract-assert

**Когда:** тест проверяет non-2xx `status_code` / `HTTP_4xx|5xx`, но не assert’ит путь кода ошибки (дефолт `errors[].code`).  
**Опции:** `error-code-path`, `error-status-only`.

### raises-without-check

**Когда:** `pytest.raises(...)` без `match=` и без проверки атрибута доменной ошибки (дефолт `.code`).  
**Опции:** `error-attr`, `exception-classes`.

### missing-mirror-test

**Когда:** source по `source-glob` без файла по `mirror-template` (`{m}` = stem, `{x}` = сегменты пути). Finding на **source**. Django-импорты не проверяет.  
**Опции:** `source-glob`, `mirror-template`.

### rbac-mutation-guard

**Когда:** тест похож на RBAC/permission и мутирует (POST/PUT/PATCH/DELETE или create/update/delete), но не проверяет запрет (401/403 / Permission / forbidden).  
**Опции:** `name-cues`, `mutating-methods`, `forbidden-signals`.
