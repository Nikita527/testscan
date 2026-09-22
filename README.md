# testscan

Линтер запахов Python-тестов на Go. Обходит дерево, ищет `test_*.py` / `*_test.py`, применяет набор правил.

## Запуск

```bash
go build -o testscan.exe ./cmd/testscan

./testscan.exe .
./testscan.exe path/to/tests --format text --fail-on error
./testscan.exe path/to/tests --format json --fail-on never
```

Пример на mp-be:

```bash
./testscan.exe /c/Dev/mp-be/tests --fail-on never --format json
```

Флаги:
- `--format text|json` (default: `text`)
- `--fail-on error|warning|never` (default: `error`) — exit `1`, если есть finding ≥ порога; ошибки CLI → exit `2`

Как библиотека:

```go
findings, err := scan.Run(ctx, []string{"tests"}, scan.Options{
    Rules: rules.Default(),
})
```

## Парсинг

На M1 **нет AST**: правила — эвристики по тексту файла (`strings.Contains` / построчный поиск). Это быстрее старта, но возможны ложные срабатывания.

## False positives (кратко)

1. `no-assert` — слово `assert` в имени функции/комментарии считается проверкой.
2. `assert-true` — сработает на `assert True` в docstring или строке.
3. `mock-only-assert` — грубое разделение: `assert_called*` vs `assert ` (с пробелом).
4. `todo-test` — `pytest.skip` / `unittest.skip` без разбора причины; `assert False` только с `, "TODO"` / `pytest.fail("TODO")`.
5. `empty-test` — file-level: пустой файл, либо только `pass` / одиночный docstring под `def` (без полного AST тела теста).

## Тесты

```bash
go test ./...
```
