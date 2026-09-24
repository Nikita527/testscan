# testscan

[English](README.md) | Русский

Линтер запахов Python-тестов на Go. Обходит дерево, ищет `test_*.py` / `*_test.py`, применяет набор правил.

**Не общий инструмент качества кода.** Не заменяет [ruff](https://docs.astral.sh/ruff/), [pyscn](https://github.com/ludo-technologies/pyscn) или pytest-xdist. Фокус — запахи тестов, типичные для AI-сгенерированных suites (пустые assert, mock-only, только happy-path), а не complexity / clones / architecture.

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
testscan path/to/tests --format html --fail-on never > testscan.html   # открыть в браузере
# Windows: --format html пишет testscan.html в cwd и сразу открывает (без > / start)
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
- `--format text|json|sarif|html` (default: `text`) — `html` — самодостаточный интерактивный отчёт (фильтр по severity / поиск; группы rule → file), не score как у pyscn. На Windows `html` пишет `testscan.html` и открывает в браузере; на других ОС — в stdout (`> testscan.html`).
- `--fail-on error|warning|never` (default: `error`) — exit `1`, если есть finding ≥ порога; ошибки CLI → exit `2`
- `--rule ID` (можно повторять) — только указанные правила; без флага — все из `Default()`
- `--disable ID` (можно повторять) — выключить правило(а); объединяется с `disable` из конфига
- одно и то же ID в `--rule` и `--disable` → ошибка, exit `2`
- `--baseline path.json` — подавить findings, совпадающие с baseline по `file+line+rule` (нет файла / битый JSON → exit `2`)
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

Обход родителей от cwd. Предпочтительно `.testscan.toml`; иначе `[tool.testscan]` в `pyproject.toml`.

```toml
# .testscan.toml
fail-on = "warning"          # или fail_on
disable = ["only-happy-path"]
paths = ["tests"]
workers = 4
```

```toml
# pyproject.toml
[tool.testscan]
fail-on = "error"
disable = ["todo-test"]
paths = ["tests", "src"]
workers = 8
```

Явные флаги CLI перекрывают конфиг. Без path-аргументов берутся `paths` из конфига (иначе `.`). `paths` резолвятся относительно каталога файла конфига (не cwd процесса). `disable` из конфига объединяется с `--disable`.

Как библиотека:

```go
selected, err := rules.Select(rules.Default(), only, disable)
findings, err := scan.Run(ctx, []string{"tests"}, scan.Options{
    Rules:   selected,
    Workers: 0, // 0 → runtime.NumCPU(); параллель по файлам
})
baseline, err := scan.LoadBaseline("baseline.json")
findings = scan.FilterBaseline(findings, baseline)
```

`scan.Options.Workers` — размер пула для `Check` по файлам (Walk последовательный).

## AST-helper (`internal/parse`)

`empty-test` и `duplicate-test-name` по возможности используют AST через внешний Python-скрипт.

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
      "lineno": 3,
      "end_lineno": 10,
      "is_empty": false,
      "has_assert": true,
      "has_raises": false
    }
  ]
}
```

Go вызывает `parse.File(ctx, path, content)` один раз на файл внутри `scan.Run` (кэш в `scan.File.Model*`), затем AST-правила читают кэш — без двойного spawn Python.

Зависимость: установленный `uv` или `python3`/`python`. Остальные правила остаются текстовыми.

## Правила (Default = 12)

Примеры hit/clean по каждому правилу: [docs/rules.ru.md](docs/rules.ru.md).

| ID | Severity | Когда |
|----|----------|--------|
| empty-test | error | AST: пустое тело `test_*`/`Test*` (pass / только docstring); fallback — эвристика по функциям/файлу |
| no-assert | error | нет assert / pytest.raises / pytest.warns |
| assert-true | warning | есть `assert True` |
| mock-only-assert | warning | есть mock-assert, нет обычного `assert ` |
| todo-test | warning | pytest.skip / fail("TODO") / assert False, "TODO" |
| duplicate-test-name | error | AST: дубликаты имён test-функций (lineno второго); fallback — построчный разбор |
| only-happy-path | warning | >3 тест-функций и нет raises/warns/assertRaises |
| assert-equals-same | warning | `assert <expr> == <expr>` с одинаковым текстом слева и справа |
| snapshot-only | warning | snapshot-инструменты без обычного (не-snapshot) `assert ` |
| overmocked-io | warning | пропатчен IO (`open` / pathlib / requests / httpx / urllib) и только mock-assert |
| test-imports-implementation-private | warning | импорт приватного `_name` (не dunder) из реализации |
| no-behavior-change | warning | все assert — только `isinstance` / `type(...)` |

## Парсинг

Два режима: AST-helper для `empty-test` / `duplicate-test-name`; остальное — эвристики по тексту (`strings.Contains` / построчный поиск).

## False positives (кратко)

1. `no-assert` — слово `assert` в имени функции/комментарии считается проверкой.
2. `assert-true` — сработает на `assert True` в docstring или строке.
3. `mock-only-assert` — грубое разделение: `assert_called*` vs `assert ` (с пробелом).
4. `todo-test` — `pytest.skip` / `unittest.skip` без разбора причины; `assert False` только с `, "TODO"` / `pytest.fail("TODO")`.
5. `empty-test` — без AST: грубый разбор тела по отступам; с AST точнее, но helpers/`pytest.skip` в теле не делают тест «непустым» сами по себе.
6. `assert-equals-same` — `assert 1 == 1`; `assert "x==y" == z` (первый `==` внутри строки); сравнения в комментариях.
7. `only-happy-path` — не видит негативные кейсы через свои хелперы/фикстуры без `raises`/`warns`/`assertRaises`.
8. `duplicate-test-name` — одинаковые имена в разных классах одного файла считаются дубликатами.
9. `snapshot-only` — любой не-snapshot `assert ` снимает правило; snapshot-API вне списка игл не видны.
10. `overmocked-io` — нужна строка с `patch` и IO-целью; другие стили mock могут промахнуться или сработать лишний раз.
11. `test-imports-implementation-private` — намеренный white-box импорт `_private` всё равно warning; top-level stdlib `import _thread` / `_ast` игнорируется, но `import pkg._internal` ловится.
12. `no-behavior-change` — один value-assert в файле снимает правило; type-check хелперы не через `isinstance`/`type` не учитываются.

## Тесты

```bash
go test ./...
```

## License

MIT
