# Каталог правил

[English](rules.md) | Русский

У каждого правила из `Default()` есть минимальный пример **hit** (должен сработать) и **clean** (не должен). Эвристики текстовые или AST — см. [README.ru.md](../README.ru.md) про ложные срабатывания, **`--diff`** / **`--all`** / **`--compare`**, тиры точности (actionable / provisional / low) и плотность actionable на 100 тестов и **tool errors** (`parse-error`).

Отключить правило: `--disable ID` или `disable = ["ID"]` в `.testscan.toml` / `[tool.testscan]`.  
Включить opt-in: `--enable ID` или `enable = ["ID"]` (см. **Optional** в конце).

**Severity vs score:** severity ниже влияет на `--fail-on` и чипы в UI. **Actionable** считает findings правил, у которых измеренная точность проходит порог (нижняя граница Вильсона ≥ 0.7, N ≥ 20) после baseline / display-фильтров; provisional (precision ≥ 0.8, N ≥ 5) показываются, но считаются отдельно; остальные правила — **low** и скрыты без `--show-low-precision` / `--all`. Пока нет размеченных данных, precision правила в отчётах — `estimated`. Health Score / grade deprecated и есть только в JSON; **note** и tool errors на него не влияют.

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
**Opt-in** (`--enable no-assert`; data-flow правило, precision на корпусе ещё не измерен).
**Когда:** тест вызывает SUT, но **ни одна проверка не зависит от результата SUT** (data-flow, см. ниже), либо в тесте вообще нет `assert` / `pytest.raises` / `pytest.warns` / assert-хелпера.

SUT = вызов кода проектного пакета (то же определение, что в `only-happy-path`) или запрос тестового клиента (`client.post(...)`, хелпер, получивший клиент). Проверка *зависит от SUT*, если читает имя, в которое попали результат или побочный эффект SUT:

- присваивание, `+=`, распаковка кортежей, атрибуты/индексы, цели `for` / `with ... as` / `except ... as`, comprehensions, f-строки и вызовы с «заражённым» аргументом;
- объекты, переданные в вызов SUT (фикстуры, фейки, в т.ч. вложенные в другие вызовы вроде `Wrap(state)`), считаются изменёнными с этого вызова;
- состояние, наблюдаемое после SUT: `obj.refresh_from_db()`, `Model.objects.*`, вызовы фикстур и тестовых хелперов после SUT, `caplog`/`capsys`/`mailoutbox`/`tmp_path`;
- `with pytest.raises(...)` / `with CaptureQueriesContext(...) as q` вокруг вызова SUT; `django_assert_num_queries`; колбэки (вложенные функции, lambda), меняющие имена, которые потом проверяются;
- assert-хелперы (`assert_*`, `check_*`, `expect_*`, `self.assert*`, `assert-helpers` из конфига) с «заражёнными» аргументами или колбэками;
- чтение проектных символов (константы/классы из проекта) и записанное состояние (`call_args`, `called`) мока, подключённого к SUT.

Не флагает: тесты с проверками, но **без вызова SUT** (на фикстурах; SUT-вызов не определить) и тесты, где проверяются только моки (это `mock-only-assert`). Тесты вообще без assert — прежняя логика; намеренные «не должно падать» пропускаются: имена с `does_not_raise`, `does_not_propagate`, `must_not_raise`, `no_error`, `doesnt_raise`, `smoke`, `swallows`, `allows`, ..., комментарий `# must not raise` / `# should not raise`, либо тело из вызовов `validate_` / `check_` / `ensure_`.

Hit:

```python
def test_without_check():
    do_something()

def test_assert_unrelated_literal():
    result = process(3)
    expected = 4
    assert expected == 4          # `result` не проверяется
```

Clean:

```python
def test_result():
    assert process(3) == 4

def test_persisted(account):
    activate(1)
    account.refresh_from_db()
    assert account.active

def test_created(client):
    response = client.post("/orders/", {"sku": "a"})
    assert response.status_code == 201

def test_mutates_fixture(cart):
    mutate(cart)
    assert cart.touched
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
**Opt-in** (`--enable mock-only-assert`; data-flow правило).
**Когда:** проверяются только mock-вызовы (`assert_called*`, `assert_has_calls`, ...) **и мок, на котором стоит assert, не связан ни с чем, что выполняет тест** - assert не может зависеть от SUT.

Мок *подключён* (не репортится), если он передан аргументом в любой не-mock вызов (в т.ч. вложенный), присвоен атрибуту другого объекта (`holder.repo = m`) или - для patch/фикстурных/неизвестных моков (`patch(...) as m`, `mock_repo`, декоратор) - если в тесте есть реальная активность (вызов SUT или другой не-framework вызов). В оркестрирующем коде наблюдаемый эффект SUT *и есть* вызов внедрённой зависимости, поэтому такой assert корректен.

Репортится: мок, который тест сам вызывает и сам проверяет, либо локальный `MagicMock()`, который никуда не передан и ни к чему не прикреплён.

Hit:

```python
def test_mock_called_by_itself():
    notifier = MagicMock()
    notifier.send("hello")
    notifier.send.assert_called_once_with("hello")

def test_mock_never_reaches_sut():
    audit = MagicMock()
    process(1)
    audit.record.assert_called_once()
```

Clean:

```python
def test_injected():
    repo = MagicMock()
    place_order("alice", repo)
    repo.save.assert_called_once_with("alice")

def test_patched_boundary():
    with patch("df_app.service.send_mail") as send:
        process(1)
    send.assert_called_once()
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

**Определение SUT** (одна общая реализация, `rules/sut.go`: `SUTContext.Calls`). SUT - только вызов, корень которого импортирован из **проекта**: не fixture/параметр, не локальная переменная с литералом (`json_payload = json.dumps(...)`), не модульный helper тестового файла (`_rows`), не метод литерала (`"x".join`), не stdlib (`sys.stdlib_module_names`), не `pytest` / `django` / `rest_framework` / `unittest` / `mock` / сторонний пакет (top-level пакет должен быть каталогом или модулем в корне проекта, `<root>/src` или выше тестового файла, вне `tests/`), не относительный импорт и не CapWords-класс, который только передаётся аргументом в другой вызов (DTO вроде `SourceFieldMeta(...)`). Локальное `x = Service()` разворачивается в `Service.method`. Тесты без вызова проектного кода пропускаются. SUT-функция с аннотацией `-> str|int|float|bool|bytes` без `raise` не имеет контракта ошибок и не флагается.

**Coverage mode:** `[rules.only-happy-path] mode = "coverage"` и `coverage = "coverage.json"`, либо `--coverage path.json`. Эвристики по файлам отключаются; правило ищет непокрытые `raise` / `except` (и связанные missing branches) в не-тестовых файлах из JSON-отчёта coverage.py. По умолчанию выключен.

Настройки: `min-tests`, `negative-names`, `mode`, `coverage`.

Hit (эвристика):

```python
from app.service import process  # project code


def test_a():
    assert process(1) == 1
def test_b():
    assert process(2) == 2
def test_c():
    assert process(3) == 3
def test_d():
    assert process(4) == 4
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
**Opt-in** (`--enable mock-tautology`; data-flow правило).
**Когда:** assert сравнивает значение, **выведенное только из конфигурации мока** (`m.return_value = X`, затем `assert m() == X`, `v = m(); assert v == X`, `assert m.return_value == X`) - в нём участвует настроенный мок, и он не зависит от SUT (`depends_on_sut` false, в assert нет чужих вызовов). Настроенный мок - локальный `MagicMock()`/`Mock()`, имя из `patch(...) as` либо `m` / `*mock*`; обычные объекты с подменёнными атрибутами (`throttle.cache.get.side_effect = ...`) моками не считаются.
Не флагает, когда значение пришло из SUT (`process(m)`, затем `assert r == m.return_value`) или читается записанное состояние подключённого мока (`m.call_args`, `m.call_count`). Декораторный `patch(..., return_value=X)` + `assert target() == X` относится к `self-patched-sut`.

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

def test_sut_result():
    m.return_value = 42
    result = process(m)
    assert result == m.return_value
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

**Severity:** note; **warning** в проектах с timezone-aware датами  
**When:** наивные `datetime.now()` / `datetime.today()` / `datetime.utcnow()` / `date.today()` без freezegun / time-machine / `freeze_time`. Вызовы с tz (`datetime.now(timezone.utc)`, `datetime.now(tz=...)`, `.astimezone()`, `.replace(tzinfo=...)`) не репортятся никогда.

**Контекст проекта.** Проект считается timezone-aware, если в Django settings `USE_TZ = True` (`DJANGO_SETTINGS_MODULE` из `pyproject.toml` / `pytest.ini` / `setup.cfg` / `tox.ini` / `manage.py`, иначе любой `settings*.py` или файл пакета `settings/`), либо не-тестовый код использует `django.utils.timezone` `now` / `localdate`. Обход ограничен (пропускает `.venv`, `venv`, `node_modules`, `.git`, `site-packages`, `tests`, `migrations`; максимум 5000 файлов) и выполняется один раз за запуск. В таком проекте находка - `warning` (`project uses timezone-aware dates (timezone.localdate()); naive datetime.now() diverges near midnight ...`), и только если значение попадает в поле модели/фабрики, assert или аргумент вызова. Не репортится, если значение не используется, находится в теле `pytest.raises` или передаётся в вызов внутри негативного теста (имя/assert вроде `missing`, `error`, `is None`). В остальных проектах остаётся `note` для любого наивного чтения.

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
**Когда:** кандидат на parametrize: два теста с одинаковым телом после нормализации литералов, которые отличаются **только входными литералами** (аргументы вызовов, setup). Assert'ы должны совпадать полностью, включая правые части сравнений, status/enum-атрибуты и аргументы `pytest.raises(E, match=...)`; списки декораторов совпадают; имена не антонимы (`passes`/`dropped`, `demotes`/`retries`, `valid`/`invalid`, `with`/`without`, `is_null`/`is_not_null`, …). Тесты соседние в одной области (не более одного теста между ними), небольшие (до 15 строк) и не имеют двух разных docstring (документированные отдельные сценарии). Кластеры 3+ — одна находка на первом тесте; пара — на более позднем («same body and assertions as test_x:N, differing only in input literals — candidate for `@pytest.mark.parametrize`»).

Hit:

```python
def test_unknown_user_returns_404(client):
    resp = client.get("/users/999")
    assert resp.status_code == 404

def test_unknown_order_returns_404(client):
    resp = client.get("/orders/123")
    assert resp.status_code == 404
```

Clean:

```python
def test_plan_free_limit():
    assert limit_for("free") == 10

def test_plan_pro_limit():
    assert limit_for("pro") == 100   # другое ожидаемое значение
```

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
**Opt-in:** выключено по умолчанию (0/52 precision на размеченном корпусе mp-be); включается `--enable name-body-mismatch`.  
**Когда:** имя теста намекает на негативный/ошибочный кейс (`rejects_invalid_*`, `*_404`, `fails_*`, `returns_error`, …), а в теле нет соответствующего сигнала (`pytest.raises`, assert на 4xx/5xx, `assert not`, …). Доменные проверки тоже считаются сигналом: сравнение с `FAILED`/`ERROR`/`REJECTED`/`SKIPPED`/…, `error_code`/`.code`, непустые `errors`/`issues`, счётчики `failed`, `assert_not_called()`, `not x.is_valid()`, try/except/else-fail. Отрицающие и resilience-имена (`does_not_fail`, `never_raises`, `fails_open`, `fallback`, `survives`, `*_when_x_fails`) и доменные глаголы (`bulk_reject`) не считаются заявкой на ошибку.

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
**Когда:** комментарий — закомментированный `assert …` либо вызов `self.assert*(…)` / `assert_*(…)`. Текст должен выглядеть как код (парные кавычки/скобки, нет соседних «голых» слов), поэтому проза вроде `# Assert on_commit ran before GET.` или `# assert that the cache is warm.` не срабатывает.

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
**Когда:** assert сравнивает с огромным литералом dict/list/tuple. Пропускает контракт API с корнем `response.data` / `resp.data` / `r.data` / `*.json()` (включая индексы поверх), результаты `.values()` / `.values_list()`, `error_details` и наборы/списки строк-идентификаторов (permission- и error-code множества).

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

Data-flow правила `no-assert`, `mock-only-assert` и `mock-tautology` тоже opt-in (описаны выше вместе со встроенными).

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


---

## Пересечение с ruff (flake8-pytest-style, PT)

Часть правил testscan дублирует проверки, которые уже есть в [ruff](https://docs.astral.sh/ruff/rules/). Перечислены только пересечения, в которых мы уверены; «partial» — ruff покрывает лишь подмножество или другой триггер.

| правило testscan | код ruff | Пересечение |
|---|---|---|
| `broad-raises` (голый `Exception`/`BaseException` без `match=`) | `PT011` (`pytest-raises-too-broad`) | да |
| `broad-raises` (в теле raises несколько операторов) | `PT012` (`pytest-raises-with-multiple-statements`) | да |
| `assert-tuple` | `F631` (`assert-tuple`) | да |
| `duplicate-test-name` | `F811` (`redefined-while-unused`) | partial: ruff срабатывает, только если первое определение не использовано |
| `swallowed-exception` | `S110` (`try-except-pass`), `BLE001` (`blind-except`), `SIM105` (`suppressible-exception`) | partial: другие триггеры (testscan ловит широкий `except` без re-raise внутри теста) |
| `commented-assert` | `ERA001` (`commented-out-code`) | partial: ruff ловит любой закомментированный код |

Без аналога в ruff (в таблице не перечислены): `skip-without-reason`, `assert-true`, `empty-test`, `no-assert`, все mock-правила и остальной каталог.

### Поведение по умолчанию: уступаем ruff

Если в конфигурации ruff проекта включён `PT011`, testscan по умолчанию **не** сообщает `broad-raises` (оба триггера правила — `PT011` и `PT012` — пропускаются вместе). Конфиг ruff ищется от корня проекта (cwd) вверх по родителям; в каждой директории по порядку `.ruff.toml`, `ruff.toml`, затем `pyproject.toml` с `[tool.ruff]`:

- включено: `select` / `extend-select` (в `[tool.ruff.lint]`, legacy `[tool.ruff]`, `[lint]` или верхнем уровне `ruff.toml`) содержит `ALL` или префикс `PT011` (`PT`, `PT0`, `PT01`, `PT011`);
- не включено, если `ignore` / `extend-ignore` содержит `ALL` или префикс `PT011` (например `PT`, `PT011`).

Явно запрошенные правила (`--rule broad-raises`, `--enable broad-raises`) выполняются всегда. Чтобы получать `broad-raises` независимо от ruff:

```toml
# .testscan.toml или [tool.testscan]
defer_to_ruff = false   # по умолчанию: true
```
