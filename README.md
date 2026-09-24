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

## Правила (Default = 8)

| ID | Severity | Когда |
|----|----------|--------|
| empty-test | error | пустой файл / только pass или docstring |
| no-assert | error | нет assert / pytest.raises / pytest.warns |
| assert-true | warning | есть `assert True` |
| mock-only-assert | warning | есть mock-assert, нет обычного `assert ` |
| todo-test | warning | pytest.skip / fail("TODO") / assert False, "TODO" |
| duplicate-test-name | error | две+ `def test_…` с одним именем в файле |
| only-happy-path | warning | >3 тест-функций и нет raises/warns/assertRaises |
| assert-equals-same | warning | `assert <expr> == <expr>` с одинаковым текстом слева и справа |

## Парсинг

На M1 **нет AST**: правила — эвристики по тексту файла (`strings.Contains` / построчный поиск). Это быстрее старта, но возможны ложные срабатывания.

## False positives (кратко)

1. `no-assert` — слово `assert` в имени функции/комментарии считается проверкой.
2. `assert-true` — сработает на `assert True` в docstring или строке.
3. `mock-only-assert` — грубое разделение: `assert_called*` vs `assert ` (с пробелом).
4. `todo-test` — `pytest.skip` / `unittest.skip` без разбора причины; `assert False` только с `, "TODO"` / `pytest.fail("TODO")`.
5. `empty-test` — file-level: пустой файл, либо только `pass` / одиночный docstring под `def` (без полного AST тела теста).
6. `assert-equals-same` — `assert 1 == 1`; `assert "x==y" == z` (первый `==` внутри строки); сравнения в комментариях.
7. `only-happy-path` — не видит негативные кейсы через свои хелперы/фикстуры без `raises`/`warns`/`assertRaises`.
8. `duplicate-test-name` — одинаковые имена в разных классах одного файла считаются дубликатами.

## Тесты

```bash
go test ./...
```
