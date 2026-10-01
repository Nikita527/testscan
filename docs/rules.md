# Rules catalog

English | [Русский](rules.ru.md)

Each default rule below has a minimal **hit** (should report) and **clean** (should not) example. Heuristics are text- or AST-based; see the main [README](../README.md) for false-positive notes, **`--diff`** / **`--all`** / **`--compare`**, precision tiers (actionable / provisional / low) and actionable density per 100 tests, and **tool errors** (`parse-error`).

Disable a rule: `--disable ID` or `disable = ["ID"]` in `.testscan.toml` / `[tool.testscan]`.  
Enable an opt-in rule: `--enable ID` or `enable = ["ID"]` (see **Optional rules** at the end).

**Severity vs score:** default severities below drive `--fail-on` and UI chips. **Actionable** metrics count findings of rules whose measured precision clears the actionable bar (Wilson lower bound ≥ 0.7, N ≥ 20), after baseline / display filters; provisional rules (precision ≥ 0.8, N ≥ 5) are shown but counted separately; every other rule is **low** and hidden unless `--show-low-precision` / `--all`. The catalog precision shown for a rule in reports is `estimated` until labelled data exists. Health Score / grade is deprecated and JSON-only; **note** findings and tool errors never affect it.

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
**Opt-in** (`--enable no-assert`; data-flow rule, precision not yet measured on the corpus).
**When:** the test exercises the SUT but **no check depends on the SUT result** (data-flow, see below), or the test has no `assert` / `pytest.raises` / `pytest.warns` / assert-helper at all.

SUT = a call into the project package (same definition as `only-happy-path`) or a test-client request (`client.post(...)`, a helper handed the client). A check *depends on the SUT* when it reads a name that flows from a SUT result or SUT side effect:

- assignment, augmented assignment, tuple unpacking, attribute/subscript access, `for`/`with ... as`/`except ... as` targets, comprehensions, f-strings, and calls with a tainted argument;
- objects passed into a SUT call (fixtures, fakes, even when nested in other calls such as `Wrap(state)`) are tainted from that call on;
- state observed after the SUT: `obj.refresh_from_db()`, `Model.objects.*`, calls on fixtures or test-local helpers after the SUT, `caplog`/`capsys`/`mailoutbox`/`tmp_path`;
- `with pytest.raises(...)` / `with CaptureQueriesContext(...) as q` around a SUT call; `django_assert_num_queries`; callbacks (nested defs, lambdas) that append to or rebind names asserted later;
- assert helpers (`assert_*`, `check_*`, `expect_*`, `self.assert*`, configured `assert-helpers`) fed with tainted data or callbacks;
- reads of project symbols (constants/classes imported from the project) and the recorded state (`call_args`, `called`) of a mock wired into the SUT.

Not reported: tests with checks but **no SUT call** (fixture-driven; SUT calls cannot be told apart) and tests whose only checks are mock assertions (that is `mock-only-assert`). Zero-assert tests keep the old logic and skip intentional "does not raise" cases: names containing `does_not_raise`, `does_not_propagate`, `must_not_raise`, `no_error`, `doesnt_raise`, `smoke`, `swallows`, `allows`, ..., or a `# must not raise` / `# should not raise` comment, or a body of `validate_` / `check_` / `ensure_` calls.

Hit:

```python
def test_without_check():
    do_something()

def test_assert_unrelated_literal():
    result = process(3)
    expected = 4
    assert expected == 4          # never looks at `result`
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
**Opt-in** (`--enable mock-only-assert`; data-flow rule).
**When:** the only checks are mock assertions (`assert_called*`, `assert_has_calls`, ...) **and the asserted mock is not connected to anything the test runs**, so the assert cannot depend on the SUT.

A mock is *wired* (never reported) when it is passed as an argument to any non-mock call (including nested ones), assigned onto another object (`holder.repo = m`), or - for patched/fixture/unknown mocks (`patch(...) as m`, `mock_repo`, decorator-injected) - when the test performs any real activity (a SUT or other non-framework call). On a corpus of orchestration code the observable effect of the SUT *is* the call on the injected dependency, so asserting on it is the right test.

Reported: a mock the test calls itself and asserts on, or a locally built `MagicMock()` that is never passed to / attached to anything.

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

**SUT definition** (one shared implementation, `rules/sut.go`: `SUTContext.Calls`). A call is the system under test only if its root name is imported from the **project**: not a fixture/parameter, not a literal-valued local (`json_payload = json.dumps(...)`), not a module-level helper of the test file (`_rows`), not a method of a literal (`"x".join`), not stdlib (`sys.stdlib_module_names`), not `pytest` / `django` / `rest_framework` / `unittest` / `mock` / third-party (the top-level package must be a directory or module under the project root, `<root>/src` or above the test file, outside `tests/`), not a relative import, and not a CapWords class constructed only as an argument of another call (DTO like `SourceFieldMeta(...)`). Local `x = Service()` resolves to `Service.method`. Tests with no project SUT call are skipped. A SUT function annotated `-> str|int|float|bool|bytes` that never `raise`s has no error contract and is not flagged.

**Coverage mode:** set `[rules.only-happy-path] mode = "coverage"` and `coverage = "coverage.json"`, or pass `--coverage path.json`. Then the rule skips per-file heuristics and reports uncovered `raise` / `except` lines (and related missing branches) in non-test files from a coverage.py JSON report. Default: off.

Configurable: `min-tests`, `negative-names`, `mode`, `coverage`.

Hit (heuristic):

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
**Opt-in** (`--enable mock-tautology`; data-flow rule).
**When:** an assert compares a value derived **only from mock configuration** (`m.return_value = X`, then `assert m() == X`, `v = m(); assert v == X`, `assert m.return_value == X`) - it involves a configured mock and does not depend on the SUT (`depends_on_sut` false, no foreign call evaluated in the assert). A configured mock is a local `MagicMock()`/`Mock()`, a `patch(...) as` name, or a `m` / `*mock*` name; arbitrary objects whose attributes are stubbed (`throttle.cache.get.side_effect = ...`) are not mocks.
Not flagged when the value comes from the SUT (`process(m)`, then `assert r == m.return_value`) or reads recorded state of a wired mock (`m.call_args`, `m.call_count`). Decorator `patch(..., return_value=X)` + `assert target() == X` is owned by `self-patched-sut`.

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

**Severity:** note; **warning** in timezone-aware projects  
**When:** naive `datetime.now()` / `datetime.today()` / `datetime.utcnow()` / `date.today()` without freezegun / time-machine / `freeze_time`. Calls with a tz (`datetime.now(timezone.utc)`, `datetime.now(tz=...)`, `.astimezone()`, `.replace(tzinfo=...)`) are never reported.

**Project awareness.** The project counts as timezone-aware when Django settings have `USE_TZ = True` (`DJANGO_SETTINGS_MODULE` from `pyproject.toml` / `pytest.ini` / `setup.cfg` / `tox.ini` / `manage.py`, else any `settings*.py` or `settings/` package file), or non-test sources use `django.utils.timezone` `now` / `localdate`. The scan is bounded (skips `.venv`, `venv`, `node_modules`, `.git`, `site-packages`, `tests`, `migrations`; max 5000 files) and runs once per run. In such a project the finding is a `warning` (`project uses timezone-aware dates (timezone.localdate()); naive datetime.now() diverges near midnight in non-UTC local time ...`) and is reported only when the value reaches a model/factory field, an assert, or another call's argument. It is not reported when the value is unused, sits in a `pytest.raises` body, or is passed to a call in a negative-path test (name/asserts such as `missing`, `error`, `is None`). Other projects keep the `note` for every naive read.

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
**When:** a parametrize candidate: two tests with the same body after normalizing literals that differ **only in input literals** (call args, setup). Assertions must be identical, including right-hand literals, status/enum attributes and `pytest.raises(E, match=...)` arguments; decorator lists must match; names must not be antonyms (`passes`/`dropped`, `demotes`/`retries`, `valid`/`invalid`, `with`/`without`, `is_null`/`is_not_null`, …). Tests must be adjacent in the same scope (at most one test in between), small (up to 15 lines), and must not carry two different docstrings (documented distinct scenarios). Clusters of 3+ emit one finding on the first test; pairs report on the later one ("same body and assertions as test_x:N, differing only in input literals — candidate for `@pytest.mark.parametrize`").

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
    assert limit_for("pro") == 100   # different expected value
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
**Opt-in:** off by default (0/52 precision on the labeled mp-be corpus); enable with `--enable name-body-mismatch`.  
**When:** the test name implies a negative/error case (`rejects_invalid_*`, `*_404`, `fails_*`, `returns_error`, …) but the body has no matching negative signal (`pytest.raises`, 4xx/5xx assert, `assert not`, …). Domain-style failure checks also count as a signal: comparisons against `FAILED`/`ERROR`/`REJECTED`/`SKIPPED`/… members or strings, `error_code`/`.code` checks, non-empty `errors`/`issues`, `failed` counters, `assert_not_called()`, `not x.is_valid()`, try/except/else-fail. Negated or resilience names (`does_not_fail`, `never_raises`, `fails_open`, `fallback`, `survives`, `*_when_x_fails`) and domain verbs (`bulk_reject`) are not failure claims.

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
**When:** a comment is a commented-out `assert …` statement or a `self.assert*(…)` / `assert_*(…)` call. The comment text must look like code (balanced quotes/brackets, no adjacent bare words), so prose such as `# Assert on_commit ran before GET.` or `# assert that the cache is warm.` does not fire.

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
**When:** an assert compares against a huge dict/list/tuple literal (long text or many commas). Skips API-contract compares whose root is `response.data` / `resp.data` / `r.data` / `*.json()` (including subscripts on top), `.values()` / `.values_list()` results, `error_details`, and sets/lists of identifier-like string literals (permission or error-code sets). Often a valid snapshot otherwise; review whether focused field checks would be clearer.

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

The data-flow rules `no-assert`, `mock-only-assert` and `mock-tautology` are opt-in as well (documented above with the other built-in rules).

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

---

## Overlap with ruff (flake8-pytest-style, PT)

Some testscan rules duplicate checks that [ruff](https://docs.astral.sh/ruff/rules/) already ships. Only overlaps we are confident about are listed; "partial" means the ruff rule covers a subset or a different trigger.

| testscan rule | ruff code | Overlap |
|---|---|---|
| `broad-raises` (bare `Exception`/`BaseException` without `match=`) | `PT011` (`pytest-raises-too-broad`) | yes |
| `broad-raises` (raises body with several statements) | `PT012` (`pytest-raises-with-multiple-statements`) | yes |
| `assert-tuple` | `F631` (`assert-tuple`) | yes |
| `duplicate-test-name` | `F811` (`redefined-while-unused`) | partial: ruff reports only when the first definition is unused |
| `swallowed-exception` | `S110` (`try-except-pass`), `BLE001` (`blind-except`), `SIM105` (`suppressible-exception`) | partial: different triggers (testscan flags a broad `except` without re-raise inside a test) |
| `commented-assert` | `ERA001` (`commented-out-code`) | partial: ruff flags any commented-out code |

Rules with no ruff equivalent (not listed above): `skip-without-reason`, `assert-true`, `empty-test`, `no-assert`, all mock rules and the rest of the catalog.

### Default behaviour: defer to ruff

When the project's ruff configuration enables `PT011`, testscan does **not** report `broad-raises` by default (both the `PT011` and `PT012` triggers of the rule are skipped together). Ruff configuration is looked up from the project root (cwd), walking up parents; per directory `.ruff.toml`, `ruff.toml`, then `pyproject.toml` with `[tool.ruff]`:

- enabled: `select` / `extend-select` (in `[tool.ruff.lint]`, legacy `[tool.ruff]`, `ruff.toml` `[lint]` or top level) contains `ALL` or a prefix of `PT011` (`PT`, `PT0`, `PT01`, `PT011`);
- not enabled when `ignore` / `extend-ignore` contains `ALL` or a prefix of `PT011` (e.g. `PT`, `PT011`).

Explicitly requested rules (`--rule broad-raises`, `--enable broad-raises`) are always run. To report `broad-raises` regardless of ruff:

```toml
# .testscan.toml or [tool.testscan]
defer_to_ruff = false   # default: true
```
