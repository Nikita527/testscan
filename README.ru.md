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
testscan path/to/tests --format text --fail-on error
testscan path/to/tests --format json --fail-on never
testscan path/to/tests --format sarif --fail-on never > testscan.sarif
testscan path/to/tests --format html -o testscan.html --open
testscan path --rule assert-equals-same
testscan path --disable no-assert --disable empty-test
testscan path --workers 4

# снять baseline и использовать
testscan path --format json --fail-on never > baseline.json
testscan path --baseline baseline.json
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
- `--format text|json|sarif|html` (default: `text`) — `html` — самодостаточный интерактивный отчёт (кольцо Health Score + фильтры). Пишите через `-o`/`--output` или stdout (`> report.html`).
- `text` заканчивается строкой `Health Score: N (G)`; `json` = `{"summary":{health_score,grade,errors,warnings,notes,files},"findings":[…]}` (breaking относительно голого массива — для baseline/CI берите `findings`).
- `-o` / `--output PATH` — записать отчёт в файл
- `--open` — открыть HTML в браузере (нужен файл: `-o` или `testscan.html`)
- `--fail-on error|warning|never` (default: `error`) — exit `1`, если есть finding ≥ порога; ошибки CLI → exit `2`
- `--rule ID` (можно повторять) — только указанные правила; без флага — все из `Default()`
- `--disable ID` (можно повторять) — выключить правило(а); объединяется с `disable` из конфига
- одно и то же ID в `--rule` и `--disable` → ошибка, exit `2`
- `--baseline path.json` — подавить findings по fingerprint (legacy `file+line+rule` — с предупреждением)
- `--workers N` — параллельные проверки файлов (`0` → `runtime.NumCPU()`)

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
paths = ["tests"]
workers = 4
exclude = ["**/conftest.py"]
assert-helpers = ["assert_*", "check_*"]
python-files = ["test_*.py", "*_test.py"]
respect-gitignore = true

[rules.only-happy-path]
severity = "note"
min-tests = 3
# mode = "coverage"          # опционально: coverage.py JSON вместо эвристик
# coverage = "coverage.json" # путь к отчёту (также: --coverage PATH)
# negative-names = ["invalid", "forbidden", "missing"]

[rules.todo-test]
severity = "warning"

[[overrides]]
path = "tests/integration/**"
disable = ["only-happy-path"]
```

Inline: `# testscan: ignore[rule-id]` / `ignore-file[...]`.

Явные флаги CLI перекрывают конфиг. Без path-аргументов берутся `paths` из конфига (иначе `.`). `paths` резолвятся относительно каталога файла конфига (не cwd процесса). `disable` из конфига объединяется с `--disable`.

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

`scan.Options.Workers` — размер пула для `Check` по файлам (Walk последовательный). Health Score не заменяет `--fail-on` (exit code по severity).

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

## Правила (Default = 22)

Примеры hit/clean по каждому правилу: [docs/rules.ru.md](docs/rules.ru.md).

| ID | Severity | Когда |
|----|----------|--------|
| empty-test | error | AST: пустое тело `test_*`/`Test*` (pass / только docstring); fallback — эвристика по функциям/файлу |
| no-assert | error | нет assert / pytest.raises / pytest.warns |
| assert-true | warning | `assert True`, или `assertTrue`/`assertFalse` со сравнением |
| mock-only-assert | warning | есть mock-assert, нет обычного `assert ` |
| todo-test | warning | pytest.skip / fail("TODO") / assert False, "TODO" |
| duplicate-test-name | error | AST: дубликаты имён test-функций (lineno второго); fallback — построчный разбор |
| only-happy-path | note | >min-tests функций без признаков негатива; опционально `mode = "coverage"` через coverage.py JSON / `--coverage` |
| assert-equals-same | warning | `assert <expr> == <expr>` с одинаковым текстом слева и справа |
| snapshot-only | warning | snapshot-инструменты без обычного (не-snapshot) `assert ` |
| overmocked-io | warning | пропатчен IO (`open` / pathlib / requests / httpx / urllib) и только mock-assert |
| test-imports-implementation-private | note | импорт приватного `_name` (не dunder) из реализации; пропускает `tests.*` / relative test helpers; в message есть имя |
| no-behavior-change | warning | все assert — только `isinstance` / `type(...)` |
| fake-mock-assert | warning | опечатка / несуществующий mock-assert |
| assert-tuple | warning | `assert (comparison, msg)` — tuple всегда truthy |
| broad-raises | warning | `raises(Exception)` без match, или тело из нескольких stmt |
| swallowed-exception | warning | bare / `except Exception` без re-raise |
| assert-in-emptyable-loop | warning | assert только внутри for по potentially empty коллекции |
| weak-assert | note | только bare truthy / is not None / len vs 0 (не Call / `assert not …`) |
| mock-tautology | warning | assert повторяет присвоение `return_value` |
| sleep-in-test | warning | `time.sleep` / `asyncio.sleep` в тесте |
| skip-without-reason | warning | skip/xfail без reason или strict |
| near-duplicate-test | note | одинаковые тела тестов после нормализации литералов |

## Парсинг

AST-helper (batch) кормит правила с `NeedsAST()`; остальное — текстовые эвристики. Walk по умолчанию пропускает venv/`__pycache__` и учитывает `exclude` + `.gitignore` (`respect-gitignore`).

## False positives (кратко)

1. `no-assert` — слово `assert` в имени функции/комментарии считается проверкой.
2. `assert-true` — сработает на `assert True` в docstring или строке.
3. `mock-only-assert` — грубое разделение: `assert_called*` vs `assert ` (с пробелом).
4. `todo-test` — `pytest.skip` / `unittest.skip` без разбора причины; `assert False` только с `, "TODO"` / `pytest.fail("TODO")`.
5. `empty-test` — без AST: грубый разбор тела по отступам; с AST точнее, но helpers/`pytest.skip` в теле не делают тест «непустым» сами по себе.
6. `assert-equals-same` — `assert 1 == 1`; `assert "x==y" == z` (первый `==` внутри строки); сравнения в комментариях.
7. `only-happy-path` — не видит негативные кейсы через свои хелперы/фикстуры без `raises`/`warns`/`assertRaises`.
8. `duplicate-test-name` — одинаковое голое имя в разных классах OK (ключ — `qualname`); дубликат = class+name.
9. `snapshot-only` — любой не-snapshot `assert ` снимает правило; snapshot-API вне списка игл не видны.
10. `overmocked-io` — нужна строка с `patch` и IO-целью; другие стили mock могут промахнуться или сработать лишний раз.
11. `test-imports-implementation-private` — намеренный white-box импорт `_private` всё равно note; `tests.*` и relative `test_*` helpers игнорируются; top-level stdlib `import _thread` / `_ast` игнорируется, но `import pkg._internal` ловится (каждый match в файле).
12. `no-behavior-change` — один value-assert в файле снимает правило; type-check хелперы не через `isinstance`/`type` не учитываются.

## Тесты

```bash
go test ./...
```

## License

MIT
