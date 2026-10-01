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

## Быстрый старт

### Из PyPI (рекомендуется)

Без Go — platform wheels уже содержат бинарник:

```bash
uvx testscan@latest tests/
# или: pipx run testscan tests/
```

```bash
# Ежедневно / PR — изменённые тесты, только высокий сигнал
testscan tests --diff origin/main --format html --open

# Снять/обновить baseline шума на всём репо, затем гейт на PR
testscan tests --format json --fail-on never > .testscan/baseline.json
testscan tests --diff origin/main --baseline .testscan/baseline.json --fail-on warning

# Тренд vs вчерашний JSON (или --baseline как точка сравнения)
testscan tests --format json -o .testscan/prev.json --fail-on never
testscan tests --compare .testscan/prev.json --format text

# Всё: без focus, все тиры точности (триаж низкоточных эвристик / notes)
testscan path/to/tests --all --format html --open

# Opt-in project-rules (DRF / API)
testscan tests --enable error-contract-assert --enable raises-without-check

testscan path/to/tests --format text --fail-on error
testscan path/to/tests --format json --fail-on never
testscan path/to/tests --format sarif --fail-on never > testscan.sarif
testscan path/to/tests --format codequality -o gl-code-quality-report.json
testscan path --rule assert-equals-same
testscan path --disable no-assert --disable empty-test
testscan path --workers 4
testscan tests --diff origin/main...HEAD --format html --open
```

### Через Go

```bash
go install github.com/Nikita527/testscan/cmd/testscan@latest
# или: go build -o testscan.exe ./cmd/testscan

./testscan.exe .
```

### Локальный embed (для контрибьюторов)

Тонкий Python-launcher: см. [python/README.md](python/README.md) (EN) или [python/README.ru.md](python/README.ru.md).

```powershell
powershell -ExecutionPolicy Bypass -File python/scripts/embed_bin.ps1
uvx --from ./python testscan --help
uvx --from ./python testscan path --fail-on never
```

Go остаётся источником правды; Python только находит бинарник и пробрасывает argv/exit code.

Пример на mp-be:

```bash
./testscan.exe /c/Dev/mp-be/tests --fail-on never --format json
```

Замер у себя (не в CI):

```bash
time ./testscan.exe /c/Dev/mp-be/tests --fail-on never >/dev/null
```

Флаги:
- `--format text|json|sarif|html|codequality` (default: `text`) — `html` — самодостаточный отчёт (в hero одно главное число: **actionable** + trend; мелкая строка с числом provisional и просканированных тестов; бейдж тира и precision правила у каждой находки; кнопки TP/FP; фильтры **error / warning / note / tool error**; поиск; по умолчанию **файл → находки**, переключатель By rule; ссылки `vscode://file/…:line`; повторы `×N` — один клик TP/FP размечает всю группу; фрагменты ±5 строк; twin у near-duplicate). Notes и tool errors по умолчанию сняты. `codequality` — GitLab Code Quality. Пишите через `-o`/`--output` или stdout.
- `text` начинается одной строкой-заголовком, например `testscan: 7 actionable findings (0.19 per 100 tests) · 6 provisional · 3650 tests in 382 files` (+ ` · trend improved (Δ -2)` при сравнении), затем по строке на находку; provisional помечены `[provisional]`. `json` = `{"summary":{actionable_count,actionable_per_100_tests,provisional_count,shown_count,tests,files,trend?,errors,warnings,notes,parse_skipped,warnings_in_grade,warnings_ignored,health_score,grade,grade_deprecated,confirmed_count,confirmed_density},"findings":[…]}`; у каждой находки добавлены `tier` (`actionable|provisional|low`) и `precision` (`{value,n,source,wilson_lower}`) — аддитивные поля. `confirmed_count` / `confirmed_density` — **deprecated** синонимы `actionable_count` / `actionable_per_100_tests` (оставлены на один релиз); `health_score` / `grade` — **deprecated** (`grade_deprecated: true`), в text и HTML больше не выводятся. **`parse-error`** — tool errors (`parse_skipped`), не считаются ни actionable, ни provisional.
- `-o` / `--output PATH` — записать отчёт в файл
- `--open` — открыть HTML в браузере; без `-o` пишет в `.testscan/reports/report_<timestamp>.html` и создаёт локальный `.gitignore`
- `--fail-on error|warning|never` (default: `error`) — exit `1`, если есть finding ≥ порога; ошибки CLI → exit `2`. **Migration:** demoted-правила и `wall-clock-in-test` по умолчанию **note**, поэтому `--fail-on warning` на них больше не падает.
- `--rule ID` — только эти правила из `All()`; без флага — `Default()` плюс `--enable`
- `--enable ID` — включить opt-in Optional-правила; объединяется с `enable` из конфига
- `--disable ID` — выключить правило(а); объединяется с `disable` из конфига
- одно и то же ID в `--rule`/`--enable` и `--disable` → ошибка, exit `2`
- `--baseline path.json` — подавить findings по fingerprint; также точка тренда, если нет `--compare`. При наличии baseline тренд **current** считается по pre-baseline actionable (большой baseline не может нарисовать ложный `improved` vs `--compare`); в summary `actionable_*` остаются post-baseline.
- `--compare path.json` — тренд actionable (число и плотность) vs предыдущий JSON-отчёт. Отчёты до 0.5 содержат только `confirmed_*`: они читаются как точка сравнения только по числу (единицы плотности другие), поэтому первый тренд после обновления приблизителен.
- `--all` — показать всё: отключает focus-фильтр **и** показывает все тиры точности (и notes / tool errors). Это прежний полный вывод до 0.5. Конфиг: `all = true`.
- `--show-grade` — **deprecated, ничего не делает** (предупреждение в stderr). Health Score / оценка A–F больше не выводятся в text и HTML; JSON по-прежнему содержит `health_score`, `grade`, `grade_deprecated: true`.
- `--show-low-precision` — дополнительно показать находки **low**-тира (estimated / не измеренные / ниже порога provisional), focus-фильтр остаётся (по умолчанию скрыты). Конфиг: `show-low-precision = true`.
- `--diff <base-ref>` — только изменённые/добавленные тестовые файлы с `base-ref`
- `--focus` — **deprecated, ничего не делает** (предупреждение в stderr): focus-фильтр теперь включён по умолчанию. Он оставляет error/warning с весом precision ≥ 0.15 и убирает `note`, `parse-error`, правила с нулевой точностью и набор демотированных эвристик. `--all` отключает его. Порядок: scan → baseline → focus → фильтр по тирам → score (+ trend) → emit.
- `--workers N` — параллельные проверки (`0` → `runtime.NumCPU()`)

**Уже есть для CI:** `--diff` + `--baseline` + `codequality`/`sarif`/`json`. Ручки: `--enable`, `--compare`, `--show-low-precision`, `--all`. Exit code (`--fail-on`), SARIF и Code Quality считаются по **показанным** находкам (после baseline и display-фильтров): в виде по умолчанию билд могут валить только actionable/provisional правила; для гейта по всему используйте `--all`.

### Тиры точности, плотность и разметка

testscan доверяет только правилам, у которых точность **измерена** на размеченных находках. Каждое правило попадает в один тир (пороги — экспортируемые константы в `scan/score.go`):

| Тир | Условие | Показывается по умолчанию |
|-----|---------|---------------------------|
| **actionable** | измерено, нижняя граница Вильсона 95% ≥ `ActionableMinWilson` (0.7) и N ≥ `ActionableMinN` (20) | да — считается в главном числе |
| **provisional** | измерено, precision ≥ `ProvisionalMinPrecision` (0.8) и N ≥ `ProvisionalMinN` (5), но не actionable | да — тег `[provisional]` / бейдж |
| **low** | всё остальное: estimated, не измерено, неизвестное правило или ниже порогов | нет (`--show-low-precision` или `--all`) |

N — число размеченных TP + FP правила. Правила без измеренных данных (в том числе все, что в каталоге только *estimated*) — **low**.

**Плотность** — actionable-находки на **100 тестов** (тест-функции и методы классов из AST): `0.19 per 100 tests`. При 0 тестов (или если ни одному выбранному правилу не нужен AST) плотность `0`.

**Вид по умолчанию** = focus-фильтр + тиры actionable и provisional. `--show-low-precision` добавляет low, `--all` снимает все фильтры.

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
        entry: uvx testscan@latest
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
| no-assert | note | нет assert / pytest.raises / pytest.warns / assert-хелпера (пропускает no-raise имена) |
| assert-true | warning | `assert True` / `assert 1` / `assert "…"`, или `assertTrue`/`assertFalse` со сравнением |
| mock-only-assert | note | только mock-assert при игноре результата SUT (пропускает procedural / adapters) |
| todo-test | warning | pytest.skip / fail("TODO") / assert False, "TODO" |
| duplicate-test-name | error | AST: дубликаты имён test-функций (lineno второго); fallback — построчный разбор |
| only-happy-path | note | >min-tests на один SUT без негативных сигналов (расширенный словарь); пропускает чистые mapper; опционально `mode = "coverage"` / `--coverage` |
| assert-equals-same | warning | `assert <expr> == <expr>` с одинаковым текстом слева и справа |
| snapshot-only | warning | snapshot-инструменты без обычного (не-snapshot) `assert ` |
| overmocked-io | warning | пропатчен IO (`open` / pathlib / requests / httpx / urllib) и только mock-assert |
| test-imports-implementation-private | note | одна находка на import приватного `_name` / `pkg._sub` (не `_UPPER_CASE`); message — кандидат на публичный подмодуль |
| no-behavior-change | warning | все assert — только `isinstance` / `type(...)` |
| fake-mock-assert | warning | опечатка / несуществующий mock-assert |
| assert-tuple | warning | `assert (comparison, msg)` — tuple всегда truthy |
| broad-raises | warning | `raises(Exception)` без конкретного класса / match, или тело из нескольких stmt |
| swallowed-exception | warning | bare / `except Exception` без re-raise |
| assert-in-emptyable-loop | warning | assert только внутри for по emptyable (`other`); пропускает литералы / `range` / consts / pre-loop assert |
| weak-assert | note | только bare truthy / is not None / len vs 0; поверхностный rejects / GET 200; не `len==N` и не `accepts_*` is_valid |
| mock-tautology | note | assert на сам мок / patched self, повторяющий `return_value` |
| sleep-in-test | warning | `time.sleep` / `asyncio.sleep` |
| wall-clock-in-test | note | `datetime.now` / `date.today` без freeze |
| skip-without-reason | warning | skip/xfail без reason или strict |
| near-duplicate-test | note | почти одинаковые тела после нормализации литералов; пропускает противоположную полярность / разный SUT / enum; 3+ → набросок parametrize |
| name-body-mismatch | note | имя теста намекает на негатив, а в теле нет негативного сигнала |
| self-patched-sut | warning | патчит сам SUT и проверяет патч |
| expected-recomputed | warning | RHS assert пересчитывает ожидаемое через SUT/хелпер (не детерминизм `f(x)==f(x)`) |
| commented-assert | note | закомментированный `# assert` / `# self.assert` |
| overbroad-equality | note | assert равен огромному литералу dict/list/tuple (пропускает `response.data` / `*.json()`) |
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

1. `no-assert` — no-raise имена / тела `validate_*` / хелперы `_assert_*` пропускаются; слово `assert` в комментарии может считаться проверкой в heuristic-режиме.
2. `assert-true` — сработает на `assert True` / другие константы в docstring или строке (эвристика).
3. `mock-only-assert` — boundary-пути и bare procedural SUT пропускаются; присвоенный и неиспользованный return всё ещё warning.
4. `todo-test` — `pytest.skip` / `unittest.skip` без разбора причины; `assert False` только с `, "TODO"` / `pytest.fail("TODO")`.
5. `empty-test` — без AST: грубый разбор тела по отступам; с AST точнее, но helpers/`pytest.skip` в теле не делают тест «непустым» сами по себе.
6. `assert-equals-same` — `assert 1 == 1`; `assert "x==y" == z` (первый `==` внутри строки); сравнения в комментариях.
7. `only-happy-path` — не видит негативные кейсы через свои хелперы/фикстуры без `raises`/`warns`/`assertRaises`.
8. `duplicate-test-name` — одинаковое голое имя в разных классах OK (ключ — `qualname`); дубликат = class+name.
9. `snapshot-only` — любой не-snapshot `assert ` снимает правило; snapshot-API вне списка игл не видны.
10. `overmocked-io` — нужна строка с `patch` и IO-целью; другие стили mock могут промахнуться или сработать лишний раз.
11. `test-imports-implementation-private` — намеренный white-box импорт `_private` всё равно note; константы `_UPPER_CASE`, `tests.*` и relative `test_*` helpers игнорируются; top-level stdlib `import _thread` / `_ast` игнорируется; одна находка на import-statement.
12. `no-behavior-change` — один value-assert в файле снимает правило; type-check хелперы не через `isinstance`/`type` не учитываются.

## Тесты

```bash
go test ./...
```

## License

MIT
