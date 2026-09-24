# testscan

English | [Русский](README.ru.md)

**Static analysis for AI-generated Python tests — catch empty asserts, mock-only checks, and happy-path-only suites before they land in CI.**

A Go linter that walks a tree, finds `test_*.py` / `*_test.py`, and applies a focused set of rules. Findings go to stdout; exit code / baseline fit CI gates.

**Not a general code quality tool.** It does not replace [ruff](https://docs.astral.sh/ruff/), [pyscn](https://github.com/ludo-technologies/pyscn), or pytest-xdist. Scope is test smells typical of AI-written suites — not complexity, clones, or architecture layers.

| | pyscn | **testscan** |
|--|--|--|
| Scope | Whole Python codebase | Only `test_*.py` / `*_test.py` |
| Pain | “AI wrote bad/complex code” | “AI wrote empty / mock-only / happy-path tests” |
| Output | HTML score + gate | Findings + exit code / baseline / SARIF |

CLI messages are in English (standard for CLIs). See [README.ru.md](README.ru.md) for Russian docs.

## Quick Start

### From PyPI (recommended)

No Go toolchain required — platform wheels ship the Go binary:

```bash
uvx testscan@latest tests/
# or: pipx run testscan tests/
```

```bash
testscan path/to/tests --format text --fail-on error
testscan path/to/tests --format json --fail-on never
testscan path/to/tests --format sarif --fail-on never > testscan.sarif
testscan path/to/tests --format html --fail-on never > testscan.html   # open in browser
# Windows: --format html writes testscan.html in cwd and opens it (no need for > / start)
testscan path --rule assert-equals-same
testscan path --disable no-assert --disable empty-test
testscan path --workers 4

# snapshot a baseline, then suppress known findings
testscan path --format json --fail-on never > baseline.json
testscan path --baseline baseline.json
```

### With Go

```bash
go install github.com/Nikita527/testscan/cmd/testscan@latest
# or: go build -o testscan ./cmd/testscan

testscan .
```

### Local embed (contributors)

Thin Python launcher — see [python/README.md](python/README.md). Embed a local binary, then run from the package path:

```powershell
powershell -ExecutionPolicy Bypass -File python/scripts/embed_bin.ps1
uvx --from ./python testscan --help
uvx --from ./python testscan path --fail-on never
```

Go remains the source of truth; Python only locates the binary and forwards argv / exit code.

### Flags

- `--format text|json|sarif|html` (default: `text`) — `html` is a self-contained interactive report (filter by severity / search; grouped by rule → file), not a pyscn-style score. On Windows, `html` writes `testscan.html` and opens it in the default browser; on other OS write to stdout (`> testscan.html`).
- `--fail-on error|warning|never` (default: `error`) — exit `1` if any finding ≥ threshold; CLI errors → exit `2`
- `--rule ID` (repeatable) — only these rules; omit to use `Default()`
- `--disable ID` (repeatable) — turn rule(s) off; merged with config `disable`
- same ID in both `--rule` and `--disable` → error, exit `2`
- `--baseline path.json` — suppress findings matching baseline by `file+line+rule` (missing / invalid JSON → exit `2`)
- `--workers N` — parallel file checks (`0` → `runtime.NumCPU()`)

### pre-commit

See [docs/pre-commit.md](docs/pre-commit.md) (mp-be example included). Short form with `uvx`:

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

### Configuration

Walks parents from the current working directory. Prefers `.testscan.toml`; otherwise `[tool.testscan]` in `pyproject.toml`.

```toml
# .testscan.toml
fail-on = "warning"          # or fail_on
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

CLI flags override config when set. If no path args are given, `paths` from config is used (else `.`). Config `paths` are resolved relative to the config file’s directory (not the process cwd). Config `disable` is unioned with `--disable`.

### As a library

```go
selected, err := rules.Select(rules.Default(), only, disable)
findings, err := scan.Run(ctx, []string{"tests"}, scan.Options{
    Rules:   selected,
    Workers: 0, // 0 → runtime.NumCPU(); parallel per file
})
baseline, err := scan.LoadBaseline("baseline.json")
findings = scan.FilterBaseline(findings, baseline)
```

`scan.Options.Workers` is the pool size for `Check` per file (Walk stays sequential).

## AST helper (`internal/parse`)

`empty-test` and `duplicate-test-name` prefer AST via an external Python script when available.

Run the helper manually:

```bash
uv run --no-project python internal/parse/ast_dump.py path/to/test_foo.py
# or: python internal/parse/ast_dump.py path/to/test_foo.py
```

Stdout schema (one JSON object per file):

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

Go calls `parse.File(ctx, path, content)` once per file inside `scan.Run` (cached on `scan.File.Model*`); AST rules read the cache — no double Python spawn.

Requires installed `uv` or `python3`/`python`. The other rules stay text-based.

## Rules (Default = 12)

Hit/clean examples for every rule: [docs/rules.md](docs/rules.md).

| ID | Severity | When |
|----|----------|------|
| empty-test | error | AST: empty `test_*` / `Test*` body (`pass` / docstring only); fallback — function/file heuristics |
| no-assert | error | no assert / pytest.raises / pytest.warns |
| assert-true | warning | has `assert True` |
| mock-only-assert | warning | has mock-assert, no plain `assert ` |
| todo-test | warning | pytest.skip / fail("TODO") / assert False, "TODO" |
| duplicate-test-name | error | AST: duplicate test function names (lineno of second); fallback — line scan |
| only-happy-path | warning | >3 test functions and no raises/warns/assertRaises |
| assert-equals-same | warning | `assert <expr> == <expr>` with identical left and right text |
| snapshot-only | warning | snapshot tooling without a non-snapshot `assert ` |
| overmocked-io | warning | IO patched (`open` / pathlib / requests / httpx / urllib) and only mock asserts |
| test-imports-implementation-private | warning | imports private `_name` (not dunder) from implementation |
| no-behavior-change | warning | every assert is only `isinstance` / `type(...)` |

## Parsing

Two modes: AST helper for `empty-test` / `duplicate-test-name`; everything else — text heuristics (`strings.Contains` / line scan).

## False positives (short)

1. `no-assert` — the word `assert` in a function name or comment counts as a check.
2. `assert-true` — fires on `assert True` in a docstring or string.
3. `mock-only-assert` — coarse split: `assert_called*` vs `assert ` (with a space).
4. `todo-test` — `pytest.skip` / `unittest.skip` without reason parsing; `assert False` only with `, "TODO"` / `pytest.fail("TODO")`.
5. `empty-test` — without AST: rough indent-based body parse; with AST more accurate, but helpers / `pytest.skip` alone do not make a test “non-empty”.
6. `assert-equals-same` — `assert 1 == 1`; `assert "x==y" == z` (first `==` inside a string); comparisons in comments.
7. `only-happy-path` — misses negative cases via custom helpers/fixtures without `raises` / `warns` / `assertRaises`.
8. `duplicate-test-name` — same names in different classes of one file count as duplicates.
9. `snapshot-only` — any non-snapshot `assert ` line clears the rule; snapshot APIs outside the needle list are missed.
10. `overmocked-io` — line must mention `patch` plus an IO target; other mock styles may miss or over-fire.
11. `test-imports-implementation-private` — intentional private imports (white-box tests) still warn; top-level stdlib `import _thread` / `_ast` is ignored, but `import pkg._internal` still hits.
12. `no-behavior-change` — a single value assert anywhere in the file clears it; type-check helpers not named `isinstance`/`type` are missed.

## Tests

```bash
go test ./...
```

## License

MIT
