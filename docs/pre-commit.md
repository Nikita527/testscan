# pre-commit

English | [Русский](pre-commit.ru.md)

Run testscan on staged `test_*.py` / `*_test.py` files. Hook id: `testscan` (see [`.pre-commit-hooks.yaml`](../.pre-commit-hooks.yaml)).

## Recommended: `uvx` (no global install)

Works without putting the binary on `PATH`. Good default for repos that already use `uv`.

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

Optional args (fail threshold, disable rules, etc.):

```yaml
        args: ["--fail-on", "error"]
```

## Official hook (binary on PATH)

Install once: `uv tool install testscan` or `pipx install testscan`, then:

```yaml
repos:
  - repo: https://github.com/Nikita527/testscan
    rev: v0.1.0 # pin a release tag
    hooks:
      - id: testscan
```

## Example: mp-be

From a Mapping Platform backend checkout (Windows-native shell; paths may differ):

```yaml
# C:/Dev/mp-be/.pre-commit-config.yaml (snippet)
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

Ad-hoc scan of the suite (outside pre-commit):

```bash
uvx testscan@latest /c/Dev/mp-be/tests --fail-on never
# SARIF for GitHub Code Scanning:
uvx testscan@latest /c/Dev/mp-be/tests --format sarif --fail-on never > testscan.sarif
```

## SARIF in CI

```bash
testscan tests/ --format sarif --fail-on never > testscan.sarif
```

Upload `testscan.sarif` with [github/codeql-action/upload-sarif](https://github.com/github/codeql-action) (or your forge’s SARIF importer). Exit code still follows `--fail-on`; use `--fail-on never` when the upload step should always run.
