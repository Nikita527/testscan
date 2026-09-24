# testscan

Линтер запахов Python-тестов на Go. Обходит дерево, ищет `test_*.py` / `*_test.py`, применяет набор правил.

## Запуск

```bash
go build -o testscan.exe ./cmd/testscan

./testscan.exe .
./testscan.exe path/to/tests --format text --fail-on error
./testscan.exe path/to/tests --format json --fail-on never
./testscan.exe path --rule assert-equals-same
./testscan.exe path --disable no-assert --disable empty-test

# снять baseline и использовать
./testscan.exe path --format json --fail-on never > baseline.json
./testscan.exe path --baseline baseline.json
```

Пример на mp-be:

```bash
./testscan.exe /c/Dev/mp-be/tests --fail-on never --format json
```

Замер у себя (не в CI):

```bash
time ./testscan.exe /c/Dev/mp-be/tests --fail-on never >/dev/null
```

Флаги:
- `--format text|json` (default: `text`)
- `--fail-on error|warning|never` (default: `error`) — exit `1`, если есть finding ≥ порога; ошибки CLI → exit `2`
- `--rule ID` (можно повторять) — только указанные правила; без флага — все из `Default()`
- `--disable ID` (можно повторять) — выключить правило(а)
- одно и то же ID в `--rule` и `--disable` → ошибка, exit `2`
- `--baseline path.json` — подавить findings, совпадающие с baseline по `file+line+rule` (нет файла / битый JSON → exit `2`)

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

Зависимость: установленный `uv` или `python3`/`python`. Остальные 6 правил остаются текстовыми.

## Правила (Default = 8)

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

## Тесты

```bash
go test ./...
```
