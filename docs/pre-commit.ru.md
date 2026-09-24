# pre-commit

[English](pre-commit.md) | Русский

Запуск testscan на staged-файлах `test_*.py` / `*_test.py`. Id хука: `testscan` (см. [`.pre-commit-hooks.yaml`](../.pre-commit-hooks.yaml)).

## Рекомендуется: `uvx` (без глобальной установки)

Не требует `testscan` в `PATH`. Удобно, если в проекте уже есть `uv`.

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

Опциональные аргументы:

```yaml
        args: ["--fail-on", "error"]
```

## Официальный хук (бинарник в PATH)

Один раз: `uv tool install testscan` или `pipx install testscan`, затем:

```yaml
repos:
  - repo: https://github.com/Nikita527/testscan
    rev: v0.1.0 # зафиксируйте тег релиза
    hooks:
      - id: testscan
```

## Пример: mp-be

В checkout Mapping Platform backend (нативный Windows shell; пути могут отличаться):

```yaml
# C:/Dev/mp-be/.pre-commit-config.yaml (фрагмент)
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
        args: ["--fail-on", "error"]
```

Разовый прогон suite (вне pre-commit):

```bash
uvx testscan@latest /c/Dev/mp-be/tests --fail-on never
# SARIF для GitHub Code Scanning:
uvx testscan@latest /c/Dev/mp-be/tests --format sarif --fail-on never > testscan.sarif
```

## SARIF в CI

```bash
testscan tests/ --format sarif --fail-on never > testscan.sarif
```

Загрузите `testscan.sarif` через [github/codeql-action/upload-sarif](https://github.com/github/codeql-action) (или импортер вашего forge). Код выхода по-прежнему зависит от `--fail-on`; для шага upload обычно `--fail-on never`.
