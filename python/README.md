# testscan (Python launcher)

Тонкая обёртка: ищет Go-бинарник и пробрасывает `argv` / exit code.  
**Источник правды — Go CLI** (`cmd/testscan`). В Python нет правил, парсинга и разбора флагов.

Пакет на PyPI/локально: имя **`testscan`**, версия `0.1.0`.

## 1. Embed бинарника

Бинарники в `src/testscan_py/bin/` **не коммитятся** (`.gitignore` / `*.exe`). Перед `uvx` сделай embed. В `pyproject.toml` стоит `ignore-vcs = true` для wheel, иначе hatchling выкинет `.exe` из-за gitignore.

```powershell
powershell -ExecutionPolicy Bypass -File python/scripts/embed_bin.ps1
```

```bash
bash python/scripts/embed_bin.sh
```

Эквивалент вручную:

```bash
go build -o python/src/testscan_py/bin/testscan.exe ./cmd/testscan
```

## 2. Запуск через uvx

```bash
uvx --from ./python testscan --help
uvx --from ./python testscan rules/testdata/empty --fail-on never
uvx --from ./python testscan path --format json --fail-on never
```

Без embed — через env (удобно для отладки wrapper):

```bash
go build -o /tmp/testscan.exe ./cmd/testscan
TESTSCAN_BIN=/tmp/testscan.exe uvx --from ./python testscan --help
```

## 3. Поиск бинарника (приоритет)

1. `TESTSCAN_BIN`
2. bundled: `testscan_py/bin/testscan.exe` (Windows) / `testscan` (unix)
3. `PATH` (`shutil.which("testscan")`)

Если не найден → сообщение в stderr, exit `2`.

## Чеклист ручной проверки

- [x] `uvx --from ./python testscan --help` → usage (exit 0)
- [x] `uvx --from ./python testscan rules/testdata/empty/hit --rule empty-test --fail-on error` → exit 1
- [x] `uvx --from ./python testscan rules/testdata/empty/clean --fail-on error` → exit 0
- [x] bundled `testscan.exe` в wheel после embed + `ignore-vcs`
- [x] `go test ./...` зелёный
