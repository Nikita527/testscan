# testscan (Python launcher)

[English](README.md) | Русский

Тонкая обёртка: ищет Go-бинарник и пробрасывает `argv` / exit code.  
**Источник правды — Go CLI** (`cmd/testscan`). В Python нет правил, парсинга и разбора флагов.

Пакет на PyPI: **`testscan`**. Версия берётся из git-тегов (`v*`) через hatch-vcs.

## Установка (PyPI)

```bash
uvx testscan@latest tests/
# или: pipx run testscan tests/
# или: pip install testscan && testscan tests/
```

Release CI собирает platform wheels (`linux/amd64`, `darwin/arm64`, `windows/amd64`) с Go-бинарником в `testscan_py/bin/`.

## Локальный embed (для контрибьюторов)

Бинарники в `src/testscan_py/bin/` **не коммитятся** (`.gitignore`). Перед `uvx --from ./python` сделай embed. В `pyproject.toml` стоят `ignore-vcs = true` и `force-include` для `bin/*`, иначе hatchling может выкинуть бинарник из-за gitignore.

```powershell
powershell -ExecutionPolicy Bypass -File python/scripts/embed_bin.ps1
```

```bash
bash python/scripts/embed_bin.sh
```

Эквивалент вручную:

```bash
go build -o python/src/testscan_py/bin/testscan ./cmd/testscan
# Windows: .../bin/testscan.exe
```

Дальше:

```bash
uvx --from ./python testscan --help
uvx --from ./python testscan rules/testdata/empty --fail-on never
```

Без embed — через env (удобно для отладки wrapper):

```bash
go build -o /tmp/testscan ./cmd/testscan
TESTSCAN_BIN=/tmp/testscan uvx --from ./python testscan --help
```

### Platform wheel (релиз / мейнтейнеры)

```bash
bash python/scripts/build_platform_wheel.sh linux-amd64 manylinux_2_17_x86_64
```

Тег релиза (`v0.1.0`) запускает `.github/workflows/release.yml` → multi-OS wheels → PyPI + GitHub Release.

Перед первой публикацией: заведи проект на PyPI и [Trusted Publisher](https://docs.pypi.org/trusted-publishers/) для этого репо (workflow `Release`, environment `pypi`), иначе publish-job упадёт на OIDC.

## Поиск бинарника (приоритет)

1. `TESTSCAN_BIN`
2. bundled: `testscan_py/bin/testscan.exe` (Windows) / `testscan` (unix)
3. `PATH` (`shutil.which("testscan")`)

Если не найден → сообщение в stderr, exit `2`.

## Чеклист ручной проверки

- [x] `uvx testscan@latest --help` (после публикации на PyPI) → usage (exit 0)
- [x] `uvx --from ./python testscan --help` после embed → usage (exit 0)
- [x] `uvx --from ./python testscan rules/testdata/empty/hit --rule empty-test --fail-on error` → exit 1
- [x] `uvx --from ./python testscan rules/testdata/empty/clean --fail-on error` → exit 0
- [x] bundled binary в wheel после embed + `ignore-vcs` / `force-include`
- [x] `go test ./...` зелёный
