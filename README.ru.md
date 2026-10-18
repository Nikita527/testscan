# testscan

[English](README.md) | Русский

Линтер запахов Python-тестов на Go. Обходит дерево, ищет `test_*.py` / `*_test.py`, применяет набор правил.

[![PyPI](https://img.shields.io/pypi/v/testscan?style=flat-square&logo=pypi)](https://pypi.org/project/testscan/)
[![Downloads](https://img.shields.io/pypi/dm/testscan?style=flat-square&logo=pypi&label=downloads)](https://pypi.org/project/testscan/)
[![Go](https://img.shields.io/github/go-mod/go-version/Nikita527/testscan?style=flat-square&logo=go)](https://go.dev/)
[![CI](https://img.shields.io/github/actions/workflow/status/Nikita527/testscan/ci.yml?branch=main&style=flat-square&label=CI)](https://github.com/Nikita527/testscan/actions/workflows/ci.yml)
[![License](https://img.shields.io/github/license/Nikita527/testscan?style=flat-square)](LICENSE)

**Не общий инструмент качества кода.** Не заменяет [ruff](https://docs.astral.sh/ruff/), [pyscn](https://github.com/ludo-technologies/pyscn) или pytest-xdist. Фокус — запахи тестов, типичные для AI-сгенерированных suites (пустые assert, mock-only, только happy-path), а не complexity / clones / architecture.

| | pyscn | **testscan** |
|--|--|--|
| Scope | Весь Python-код | Только `test_*.py` / `*_test.py` |
| Pain | «AI написал плохой/сложный код» | «AI написал пустые / mock-only / happy-path тесты» |
| Output | HTML score + gate | HTML score + findings / exit code / baseline / SARIF |

Сообщения CLI на английском (стандарт для CLI).

## Как это выглядит

Вид по умолчанию показывает **deterministic**-запахи (пустые тесты, tuple-assert, фейковые mock-методы, …) с конкретной строкой fix:

```text
testscan: 0 actionable findings (0.00 per 100 tests) · 0 provisional · 3 deterministic · 4 tests in 3 files
rules/testdata/empty/hit/test_empty.py:1: error empty-test: empty test function: test_vacuous [deterministic]
  → fix: add the missing arrange/act/assert body, or delete the test
rules/testdata/assert_tuple/hit/test_tuple.py:3: warning assert-tuple: assert on a tuple is always True; did you mean a comparison? [deterministic]
  → fix: remove the parentheses/comma so the assert compares values: `assert a == b`
rules/testdata/fake_mock_assert/hit/test_fake.py:2: warning fake-mock-assert: suspicious mock assert (typo or non-existent method) [deterministic]
  → fix: fix the mock assertion to a real method, e.g. `mock.assert_called_once_with(...)`
```

HTML-отчёт добавляет бейджи тиров и разметку TP/FP (`--format html --open`). Опциональный скрин: [docs/assets/](docs/assets/).

## Быстрый старт

```bash
uvx testscan@latest tests/                          # PyPI wheel — Go не нужен
testscan tests --diff origin/main --format html --open
testscan tests --format json --fail-on never > .testscan/baseline.json
testscan tests --diff origin/main --baseline .testscan/baseline.json --fail-on warning
testscan tests --format agent                       # по строке на находку для AI-хуков
```

Установка через Go: `go install github.com/Nikita527/testscan/cmd/testscan@latest`. Локальный embed: [python/README.ru.md](python/README.ru.md).

## С ИИ-агентами

`--format agent` печатает по компактной строке на находку (`file:line rule — проблема → fix`) и summary — для хуков Claude Code / Cursor / Windsurf, которые возвращают находки в тот же ход:

```text
tests/test_orders.py:42 sleep-in-test — time.sleep in test → remove `time.sleep(...)`; wait on the condition or use a fake clock
tests/test_orders.py:88 empty-test — empty test function: test_vacuous → add the missing arrange/act/assert body, or delete the test
testscan: 2 findings (1 error, 1 warning) · 31 tests
```

Подключайте как post-edit hook на изменённые `test_*.py`. Рецепты: [docs/agent-hook.ru.md](docs/agent-hook.ru.md).

## Что ловит (пример)

| Правило | Запах | Hit | Clean |
|---------|-------|-----|-------|
| `empty-test` | пустое тело `test_*` | `def test_vacuous(): pass` | `assert 1 == 1` в теле |
| `assert-tuple` | `assert (x == 1, "msg")` всегда true | скобки + запятая | `assert x == 1, "msg"` |
| `fake-mock-assert` | опечатка / несуществующий mock assert | `mock.assert_called_once_wiht()` | `mock.assert_called_once_with()` |
| `assert-equals-same` | assert сравнивает выражение с собой | `assert x == x` | `assert x == y` |

Полные hit/clean для всех правил: [docs/rules.ru.md](docs/rules.ru.md).

### Флаги

- `--format text|json|sarif|html|codequality|agent` (default: `text`) — `agent` для AI-хуков; `html` — интерактивный отчёт; `codequality` = GitLab Code Quality. Пишите через `-o`/`--output` или stdout.
- `--fail-on error|warning|never` (default: `error`) — exit `1`, если есть **показанная** находка ≥ порога; ошибки CLI → exit `2`.
- `--diff <base-ref>` — только тестовые файлы, изменённые с `base-ref` (PR-workflow).
- `--baseline path.json` / `--compare path.json` — подавить известные находки / тренд vs предыдущий JSON.
- `--all` — показать всё (полный вывод до 0.5). `--show-low-precision` — также **low**-тир (focus остаётся).
- `--rule` / `--enable` / `--disable` — выбор правил (можно через запятую); одно ID в enable и disable → exit `2`.
- `--workers N` — параллельные проверки (`0` → `runtime.NumCPU()`).
- `-o` / `--output PATH`, `--open` — записать / открыть HTML (по умолчанию `.testscan/reports/`).

CI: `--diff` + `--baseline` + `codequality|sarif|json`. Exit code, SARIF и Code Quality считаются по **показанному** набору (actionable / provisional / deterministic по умолчанию). Тренд `--compare` — только **actionable**; всплеск `deterministic_count` может валить гейт, не двигая trend.

**Миграция 0.4 → 0.5 / 0.5 → 0.6** (deprecated-флаги, гейт deterministic): [docs/migration-0.5.ru.md](docs/migration-0.5.ru.md).

### Тиры точности, плотность и разметка

Каждое правило попадает в один тир (пороги в `scan/score.go`):

| Тир | Условие | Показывается по умолчанию |
|-----|---------|---------------------------|
| **actionable** | измерено, нижняя граница Вильсона 95% ≥ `ActionableMinWilson` (0.7) и N ≥ `ActionableMinN` (20) | да — считается в главном числе |
| **provisional** | измерено, precision ≥ `ProvisionalMinPrecision` (0.8) и N ≥ `ProvisionalMinN` (5), но не actionable | да — тег `[provisional]` / бейдж |
| **deterministic** | каталог `SourceDeterministic` — следует из семантики Python/pytest (empty-test, assert-true, todo-test, fake-mock-assert, …) | да — тег `[deterministic]` / бейдж; **не** входит в `actionable_count` |
| **low** | всё остальное: estimated, не измерено, неизвестное правило или ниже порогов | нет (`--show-low-precision` или `--all`) |

N — число размеченных TP + FP правила. Estimated-записи в каталоге (включая оставшиеся `est(1.0)`-эвристики) — **low**.

**Плотность** — actionable-находки на **100 тестов**: `0.19 per 100 tests`. При 0 тестов плотность `0`. **Тренд** (`--compare`) — те же actionable-метрики; гейт и `deterministic_count` — отдельно.

**Вид по умолчанию** = focus-фильтр + тиры actionable, provisional и deterministic. `--show-low-precision` добавляет low, `--all` снимает все фильтры.

**Измеренная точность.** Разметьте находки и посчитайте точность по правилам:

1. Сделайте HTML-отчёт (`testscan tests --all --format html -o report.html`) и пометьте находки кнопками **TP** / **FP** (строка со свёрнутым `×N` размечает все находки группы; состояние хранится в localStorage браузера). **Export labels** скачивает `labels.json`.
2. `labels.json` (обычно `.testscan/labels.json`) — JSON-массив; для одного fingerprint побеждает последняя запись:

   ```json
   [
     {"fingerprint": "9d17e1409e5c531b81c3858012540f41", "rule": "empty-test",
      "file": "tests/test_a.py", "line": 12, "label": "tp", "note": "optional"}
   ]
   ```

   `label` — `tp`, `fp` или `skip` (skip в статистике не учитывается).
3. `testscan precision --labels .testscan/labels.json [--report out.json] [--format text|json]` выводит по правилам `N`, `TP`, `FP`, precision и нижнюю границу Вильсона 95%. С `--report` (JSON из `--format json`) учитываются только метки, чьи fingerprint есть в этом отчёте.
4. Перенесите измеренные значения в каталог (`scan.RulePrecision`, например `scan.Measured(tp, n)`) — тир выводится из них.


### pre-commit

См. [docs/pre-commit.ru.md](docs/pre-commit.ru.md) (есть пример для mp-be). Короткий вариант с `uvx`:

```yaml
# .pre-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: testscan
        name: testscan
        entry: uvx testscan@0.5.0   # pin релиза; см. docs/pre-commit.ru.md
        language: system
        types: [python]
        files: '(^|/)(test_[^/]*|[^/]*_test)\.py$'
        pass_filenames: true
```

### Конфиг

Обход родителей от cwd. Предпочтительно `.testscan.toml`; иначе `[tool.testscan]` в `pyproject.toml`. Если не задано — fallback на `[tool.pytest.ini_options]` для `python_files` / `python_functions` / `python_classes`.

```toml
# .testscan.toml
fail-on = "warning"
disable = ["only-happy-path"]
enable = ["error-contract-assert", "raises-without-check"]
paths = ["tests"]
workers = 4
exclude = ["**/conftest.py"]
assert-helpers = ["assert_*", "check_*"]
python-files = ["test_*.py", "*_test.py"]
respect-gitignore = true
# show-low-precision = true  # показать и low-тир (estimated / не измеренные правила); focus остаётся
# all = true                 # показать всё (как --all): без focus, все тиры
# defer_to_ruff = false      # сообщать broad-raises, даже если ruff включает PT011 (по умолчанию true, см. docs/rules.ru.md)

[rules.only-happy-path]
severity = "note"
min-tests = 3
# mode = "coverage"
# coverage = "coverage.json"
# negative-names = ["invalid", "forbidden", "missing"]

[rules.todo-test]
severity = "warning"

[rules.error-contract-assert]
error-code-path = "errors[].code"

[rules.raises-without-check]
error-attr = "code"

[rules.missing-mirror-test]
source-glob = "app/**/domain/*.py"
mirror-template = "tests/{x}/domain/test_{m}.py"

[rules.rbac-mutation-guard]
# name-cues / mutating-methods / forbidden-signals — см. README (EN)

[[overrides]]
path = "tests/integration/**"
disable = ["only-happy-path"]
```

Inline: `# testscan: ignore[rule-id]` / `ignore-file[...]`.

Явные флаги CLI перекрывают конфиг. Без path-аргументов берутся `paths` из конфига (иначе `.`). `paths` резолвятся относительно каталога файла конфига (не cwd процесса). `disable` / `enable` из конфига объединяются с `--disable` / `--enable`.

`--coverage path` включает coverage-режим для `only-happy-path` (непокрытые `raise`/`except` в измеренном коде).

Как библиотека:

```go
selected, err := rules.Select(rules.Default(), only, disable)
result, err := scan.Run(ctx, []string{"tests"}, scan.Options{
    Rules:   selected,
    Workers: 0, // 0 → runtime.NumCPU(); параллель по файлам
})
findings := result.Findings
score := scan.CalculateScore(findings, result.Files)
baseline, err := scan.LoadBaseline("baseline.json")
findings = scan.FilterBaseline(findings, baseline).Findings
```

`scan.Options.Workers` — размер пула для `Check` по файлам (Walk последовательный). Exit code по severity (`--fail-on`) и считается по показанным находкам.

## AST-helper (`internal/parse`)

AST-правила используют внешний Python-helper (batch JSONL). Модель включает поля на тест плюс file-level `imports`.

Запуск helper вручную:

```bash
uv run --no-project python internal/parse/ast_dump.py path/to/test_foo.py
# или: python internal/parse/ast_dump.py path/to/test_foo.py
```

Схема stdout (один JSON на файл):

```json
{
  "tests": [
    {
      "name": "test_foo",
      "qualname": "test_foo",
      "lineno": 3,
      "end_lineno": 10,
      "is_empty": false,
      "has_assert": true,
      "has_raises": false
    }
  ],
  "imports": [
    {"kind": "from", "module": "pkg", "names": ["helper"], "lineno": 1}
  ]
}
```

Go вызывает `parse.File` / batch один раз на файл внутри `scan.Run` (кэш в `scan.File.Model*`); AST-правила читают кэш — без двойного spawn Python.

Зависимость: установленный `uv` или `python3`/`python`. Текстовые эвристики — только с `AllowHeuristicFallback` или в unit-тестах.

## Правила (Default + Optional)

Примеры hit/clean: [docs/rules.ru.md](docs/rules.ru.md). Default включены; Optional — через `--enable` / `enable`.

| ID | Severity | Когда |
|----|----------|--------|
| empty-test | error | AST: пустое тело `test_*`/`Test*` (pass / только docstring); fallback — эвристика по функциям/файлу |
| assert-true | warning | `assert True` / `assert 1` / `assert "…"` / `… or True`, или `assertTrue`/`assertFalse` со сравнением (**deterministic**) |
| todo-test | warning | pytest.skip / fail("TODO") / assert False, "TODO" (**deterministic**) |
| duplicate-test-name | error | AST: дубликаты имён test-функций (lineno второго); fallback — построчный разбор |
| only-happy-path | note | >min-tests на один SUT без негативных сигналов (расширенный словарь); пропускает чистые mapper; опционально `mode = "coverage"` / `--coverage` |
| assert-equals-same | warning | `assert <name\|literal> == <то же>` (не `f(x)==f(x)` / кэш атрибута) |
| snapshot-only | warning | snapshot-инструменты без обычного (не-snapshot) `assert ` |
| overmocked-io | warning | пропатчен IO (`open` / pathlib / requests / httpx / urllib) и только mock-assert |
| overmocked-lock | warning | пропатчен/подменён lock API (Django `select_for_update*` / SQLAlchemy `with_for_update` / `with_lockmode`; multiline patch, setattr, attr assign) и только mock-assert |
| test-imports-implementation-private | note | одна находка на import приватного `_name` / `pkg._sub` (не `_UPPER_CASE`); message — кандидат на публичный подмодуль |
| no-behavior-change | warning | все assert — только `isinstance` / `type(...)` |
| fake-mock-assert | warning | опечатка / несуществующий mock-assert (не `Mock.called` и не локальный `called`) |
| assert-inside-broad-raises | warning | `assert False` / `pytest.fail` внутри `pytest.raises(Exception\|AssertionError)` (**deterministic**) |
| assert-tuple | warning | `assert (comparison, msg)` — tuple всегда truthy |
| broad-raises | warning | `raises(Exception)` без конкретного класса / match, или тело из нескольких stmt |
| swallowed-exception | warning | bare / `except Exception` без re-raise |
| assert-in-emptyable-loop | warning | assert только внутри for по emptyable (`other`); пропускает литералы / `range` / consts / pre-loop assert |
| weak-assert | note | только bare truthy name / is not None / len > 0; поверхностный rejects / GET 200; не `len==0` и не bool-атрибуты |
| sleep-in-test | warning | `time.sleep` / `asyncio.sleep` (не `sleep(0)` и не polling с дедлайном) |
| wall-clock-in-test | note | `datetime.now` / `date.today` без freeze (не чистый call-arg; **скрыт по умолчанию** — `--all` / `--enable wall-clock-in-test`) |
| skip-without-reason | warning | skip/xfail без reason или strict |
| near-duplicate-test | note | почти одинаковые тела после нормализации литералов; **provisional — виден по умолчанию** (в text — одна сводка на файл; `--all` перечисляет каждую); пропускает противоположную полярность / разный SUT / enum; 3+ → набросок parametrize |
| self-patched-sut | warning | патчит сам SUT и проверяет патч |
| expected-recomputed | warning | RHS assert пересчитывает ожидаемое через SUT/хелпер (не CamelCase-конструкторы / не `f(x)==f(x)`) |
| commented-assert | note | закомментированный `# assert` / `# self.assert` |
| overbroad-equality | *opt-in* | assert равен огромному литералу dict/list/tuple (пропускает `response.data` / `*.json()`) |
| no-assert | *opt-in* | data-flow: нет assert, зависящего от результата SUT (трассирует через присваивания/атрибуты/subscripts в вызовы проектного кода) |
| mock-only-assert | *opt-in* | data-flow: assert на мок, не подключённый в SUT |
| mock-tautology | *opt-in* | data-flow: assert сравнивает только значения из конфигурации мока |
| name-body-mismatch | *opt-in* | имя теста намекает на ошибку, но в теле нет сигнала ошибки (status FAILED/REJECTED, error_code, non-empty errors/issues) |
| error-contract-assert | *opt-in* | non-2xx status без проверки пути кода ошибки |
| raises-without-check | *opt-in* | `pytest.raises` без `match=` / attr доменной ошибки |
| missing-mirror-test | *opt-in* | domain-файл без зеркального теста |
| rbac-mutation-guard | *opt-in* | RBAC-тест мутирует без проверки запрета |

## Парсинг

AST-helper (batch) кормит правила с `NeedsAST()`; остальное — текстовые эвристики. Walk по умолчанию пропускает venv/`__pycache__` и учитывает `exclude` + `.gitignore` (`respect-gitignore`).

Helper парсит исходники как UTF-8 (форсирует `PYTHONUTF8=1` / `PYTHONIOENCODING=utf-8` у subprocess) и не собирает `@pytest.fixture` / `@fixture` и классы с `__init__` (как pytest). Сбой парсинга → одна находка `parse-error` (tool error) на файл.

## За пределами статики

testscan — дешёвый префильтр для AI-тестов. **Оркестрация мутаций** (`testscan mutate` → [mutmut](https://mutmut.readthedocs.io/) / cosmic-ray на `--diff`, survivors как findings) — **Planned** отдельной подкомандой; **не** в этом релизе. Пока мутационные тулы запускайте отдельно по изменённым строкам, если нужна объективная проверка «тест ловит баг».

## False positives (кратко)

1. `no-assert` — data-flow трассирует только до SUT (вызовов, импортированных из пакета проекта); вызовы на фикстурах или локальных хелперах после SUT считаются наблюдением состояния, поэтому recall намеренно низкий. No-raise имена по-прежнему пропускаются.
2. `assert-true` — сработает на `assert True` / другие константы в docstring или строке (эвристика).
3. `mock-only-assert` — data-flow трассирует только до SUT (вызовов, импортированных из пакета проекта); recall намеренно низкий, зависит от восстановления control-flow из AST.
4. `todo-test` — `pytest.skip` / `unittest.skip` без разбора причины; `assert False` только с `, "TODO"` / `pytest.fail("TODO")`.
5. `empty-test` — без AST: грубый разбор тела по отступам; с AST точнее, но helpers/`pytest.skip` в теле не делают тест «непустым» сами по себе.
6. `assert-equals-same` — `assert 1 == 1`; `assert "x==y" == z` (первый `==` внутри строки); сравнения в комментариях.
7. `only-happy-path` — не видит негативные кейсы через свои хелперы/фикстуры без `raises`/`warns`/`assertRaises`.
8. `duplicate-test-name` — одинаковое голое имя в разных классах OK (ключ — `qualname`); дубликат = class+name.
9. `snapshot-only` — любой не-snapshot `assert ` снимает правило; snapshot-API вне списка игл не видны.
10. `overmocked-io` — нужна строка с `patch` и IO-целью; другие стили mock могут промахнуться или сработать лишний раз.
11. `overmocked-lock` — call span `patch`/`setattr` (включая multiline) или replace `.lock_leaf = mock`; без lookback в соседние тесты; heuristic fallback на весь файл; не доказывает отсутствие второго concurrent actor; имена хелперов с lock-leaf могут давать false positive.
12. `test-imports-implementation-private` — намеренный white-box импорт `_private` всё равно note; константы `_UPPER_CASE`, `tests.*` и relative `test_*` helpers игнорируются; top-level stdlib `import _thread` / `_ast` игнорируется; одна находка на import-statement.
13. `no-behavior-change` — один value-assert в файле снимает правило; type-check хелперы не через `isinstance`/`type` не учитываются.

## Тесты

```bash
go test ./...
```

## Участие

Сборка, AST helper, embed wheels, rule testdata и precision/labels: [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT
