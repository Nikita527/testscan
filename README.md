# testscan

English | [Русский](README.ru.md)

**Static analysis for AI-generated Python tests — catch empty asserts, mock-only checks, and happy-path-only suites before they land in CI.**

[![PyPI](https://img.shields.io/pypi/v/testscan?style=flat-square&logo=pypi)](https://pypi.org/project/testscan/)
[![Downloads](https://img.shields.io/pypi/dm/testscan?style=flat-square&logo=pypi&label=downloads)](https://pypi.org/project/testscan/)
[![Go](https://img.shields.io/github/go-mod/go-version/Nikita527/testscan?style=flat-square&logo=go)](https://go.dev/)
[![CI](https://img.shields.io/github/actions/workflow/status/Nikita527/testscan/ci.yml?branch=main&style=flat-square&label=CI)](https://github.com/Nikita527/testscan/actions/workflows/ci.yml)
[![License](https://img.shields.io/github/license/Nikita527/testscan?style=flat-square)](LICENSE)

A Go linter that walks a tree, finds `test_*.py` / `*_test.py`, and applies a focused set of rules. Findings go to stdout; exit code / baseline fit CI gates.

**Not a general code quality tool.** It does not replace [ruff](https://docs.astral.sh/ruff/), [pyscn](https://github.com/ludo-technologies/pyscn), or pytest-xdist. Scope is test smells typical of AI-written suites — not complexity, clones, or architecture layers.

| | pyscn | **testscan** |
|--|--|--|
| Scope | Whole Python codebase | Only `test_*.py` / `*_test.py` |
| Pain | “AI wrote bad/complex code” | “AI wrote empty / mock-only / happy-path tests” |
| Output | HTML score + gate | HTML score + findings / exit code / baseline / SARIF |

CLI messages are in English (standard for CLIs). See [README.ru.md](README.ru.md) for Russian docs.

## Quick Start

### From PyPI (recommended)

No Go toolchain required — platform wheels ship the Go binary:

```bash
uvx testscan@latest tests/
# or: pipx run testscan tests/
```

```bash
# Daily / PR — changed tests, high-signal only
testscan tests --diff origin/main --focus --format html --open

# Snapshot or refresh baseline noise on the whole tree, then gate the PR
testscan tests --format json --fail-on never > .testscan/baseline.json
testscan tests --diff origin/main --baseline .testscan/baseline.json --focus --fail-on warning

# Full scan without --focus (triage low-precision heuristics / notes)
testscan path/to/tests --format html --open

testscan path/to/tests --format text --fail-on error
testscan path/to/tests --format json --fail-on never
testscan path/to/tests --format sarif --fail-on never > testscan.sarif
testscan path/to/tests --format codequality -o gl-code-quality-report.json
testscan path --rule assert-equals-same
testscan path --disable no-assert --disable empty-test
testscan path --workers 4
testscan tests --diff origin/main...HEAD --format html --open
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

- `--format text|json|sarif|html|codequality` (default: `text`) — `html` is a self-contained interactive report (Health Score ring + filters for **error / warning / note / tool error**; search; grouped by rule → file sorted by **precision desc**; code snippets ±5 lines; near-duplicate twin links). Notes and tool errors are unchecked by default so the first view is trusted warnings. `codequality` is GitLab Code Quality (Code Climate JSON). Write with `-o`/`--output`, or redirect stdout (`> report.html`).
- `text` ends with `Health Score: N (G)`; `json` is `{"summary":{health_score,grade,errors,warnings,notes,files,parse_skipped,warnings_in_grade,warnings_ignored},"findings":[…]}` (breaking vs a bare array — use `findings` for baselines / CI parsers). Health Score weights **error+warning** by per-rule precision (precision < `MinPrecisionForGrade` = 0.15 → weight 0 for grade); mid-precision rules count proportionally. **notes** are UI-only (no penalty). **`parse-error` / AST failures** are **tool errors**: counted in `parse_skipped` and shown separately in HTML — they do **not** affect the grade.
- `-o` / `--output PATH` — write report to a file (any format)
- `--open` — open HTML report in the default browser; without `-o` writes to `.testscan/reports/report_<timestamp>.html` and creates a local `.gitignore` so reports stay out of git
- `--fail-on error|warning|never` (default: `error`) — exit `1` if any finding ≥ threshold; CLI errors → exit `2`. **Migration:** demoted rules (`name-body-mismatch`, `no-assert`, `mock-only-assert`, `mock-tautology`, `overbroad-equality`, wall-clock `sleep-in-test`) are **note** by default, so `--fail-on warning` no longer fails on them; use notes triage, baseline, or config severity overrides if you still want gates.
- `--rule ID` (repeatable) — only these rules; omit to use `Default()`
- `--disable ID` (repeatable) — turn rule(s) off; merged with config `disable`
- same ID in both `--rule` and `--disable` → error, exit `2`
- `--baseline path.json` — suppress findings matching baseline by fingerprint (legacy `file+line+rule` still works once with a warning)
- `--diff <base-ref>` — scan only test files changed or added since `base-ref` (`git diff --name-only --diff-filter=ACMR`). Primary PR workflow for reviewing AI-generated tests; use `origin/main` or `origin/main...HEAD`.
- `--focus` — after baseline, keep only error/warning findings whose rule precision is ≥ `MinPrecisionForGrade` (0.15); drops `note`, `parse-error`, zero-precision rules, and a fixed set of demoted heuristics even if config bumps their severity. Health Score is computed on the focused set so the number matches what you see. Combine with `--diff` / `--baseline` for daily CI (order: scan → baseline → focus → score → emit).
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

Walks parents from the current working directory. Prefers `.testscan.toml`; otherwise `[tool.testscan]` in `pyproject.toml`. Falls back to `[tool.pytest.ini_options]` for `python_files` / `python_functions` / `python_classes` when unset.

```toml
# .testscan.toml
fail-on = "warning"
disable = ["only-happy-path"]
paths = ["tests"]
workers = 4
exclude = ["**/conftest.py"]
assert-helpers = ["assert_*", "check_*"]
python-files = ["test_*.py", "*_test.py"]
python-functions = ["test_*"]
python-classes = ["Test*"]
respect-gitignore = true

[rules.only-happy-path]
severity = "note"
min-tests = 3
# mode = "coverage"          # optional: use coverage.py JSON instead of heuristics
# coverage = "coverage.json" # path to report (also: --coverage PATH)
# negative-names = ["invalid", "forbidden", "missing"]

[rules.todo-test]
severity = "warning"

[[overrides]]
path = "tests/integration/**"
disable = ["only-happy-path"]
```

Inline suppress: `# testscan: ignore[rule-id]` (or `ignore[a,b]`) on the finding line, the line before/`def` of the test, or `# testscan: ignore-file[rule-id]` / ignore in the module header.

CLI flags override config when set. If no path args are given, `paths` from config is used (else `.`). Config `paths` are resolved relative to the config file’s directory (not the process cwd). Config `disable` is unioned with `--disable`.

`--coverage path` enables coverage mode for `only-happy-path` (uncovered `raise`/`except` in measured code).

### As a library

```go
selected, err := rules.Select(rules.Default(), only, disable)
result, err := scan.Run(ctx, []string{"tests"}, scan.Options{
    Rules:   selected,
    Workers: 0, // 0 → runtime.NumCPU(); parallel per file
})
findings := result.Findings
score := scan.CalculateScore(findings, result.Files)
baseline, err := scan.LoadBaseline("baseline.json")
findings = scan.FilterBaseline(findings, baseline).Findings
```

`scan.Options.Workers` is the pool size for `Check` per file (Walk stays sequential). Health Score does not replace `--fail-on` (exit code stays severity-based).

## AST helper (`internal/parse`)

AST-backed rules prefer an external Python helper when available (batch JSONL for speed). The model includes per-test fields plus file-level `imports`.

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

Go calls `parse.File` / batch once per file inside `scan.Run` (cached on `scan.File.Model*`); AST rules read the cache — no double Python spawn.

Requires installed `uv` or `python3`/`python`. Text heuristics remain only for unit tests with `AllowHeuristicFallback` or when AST is unavailable under that flag.

## Rules (Default = 27)

Hit/clean examples for every rule: [docs/rules.md](docs/rules.md).

| ID | Severity | When |
|----|----------|------|
| empty-test | error | AST: empty `test_*` / `Test*` body (`pass` / docstring only); fallback — function/file heuristics |
| no-assert | note | no assert / pytest.raises / pytest.warns / assert-helper (skips no-raise names) |
| assert-true | warning | `assert True` / `assert 1` / `assert "…"`, or `assertTrue`/`assertFalse` with a comparison |
| mock-only-assert | note | mock-assert only while SUT return is ignored (skips procedural / adapters) |
| todo-test | warning | pytest.skip / fail("TODO") / assert False, "TODO" |
| duplicate-test-name | error | AST: duplicate test function names (lineno of second); fallback — line scan |
| only-happy-path | note | >min-tests for the same SUT with no negative-path signals (expanded dict); skips pure mappers; optional `mode = "coverage"` / `--coverage` |
| assert-equals-same | warning | `assert <expr> == <expr>` with identical left and right text |
| snapshot-only | warning | snapshot tooling without a non-snapshot `assert ` |
| overmocked-io | warning | IO patched (`open` / pathlib / requests / httpx / urllib) and only mock asserts |
| test-imports-implementation-private | note | one finding per import of private `_name` / `pkg._sub` (not `_UPPER_CASE`); message suggests a public submodule |
| no-behavior-change | warning | every assert is only `isinstance` / `type(...)` |
| fake-mock-assert | warning | typo / non-existent mock assert method |
| assert-tuple | warning | `assert (comparison, msg)` tuple always truthy |
| broad-raises | warning | `raises(Exception)` without concrete class / match, or multi-stmt body |
| swallowed-exception | warning | bare / `except Exception` without re-raise |
| assert-in-emptyable-loop | warning | asserts only inside for over emptyable (`other`) iterable; skips literals / `range` / consts / pre-loop assert |
| weak-assert | note | only bare truthy / is not None / len vs 0; shallow rejects / GET 200; not `len==N` or `accepts_*` is_valid |
| mock-tautology | note | assert on mock itself / patched self echoing `return_value` |
| sleep-in-test | warning / note | `time.sleep` → warning; `datetime.now` / `date.today` without freeze → note |
| skip-without-reason | warning | skip/xfail without reason or strict |
| near-duplicate-test | note | near-identical bodies after literal norm; skips opposite polarity / different SUT / enums; 3+ → parametrize sketch |
| name-body-mismatch | note | test name implies negative case but body has no negative signal |
| self-patched-sut | warning | patches the SUT under test and asserts the patch |
| expected-recomputed | warning | assert RHS recomputes expected via SUT/helper call (not `f(x)==f(x)` determinism) |
| commented-assert | note | commented-out `# assert` / `# self.assert` |
| overbroad-equality | note | assert equals a huge dict/list/tuple literal (often a valid snapshot; review) |

## Parsing

AST helper (batch) feeds rules that `NeedsAST()`; remaining rules use text heuristics. Walk skips venv/`__pycache__` by default and honors `exclude` + `.gitignore` (`respect-gitignore`).

The helper parses source as UTF-8 (forces `PYTHONUTF8=1` / `PYTHONIOENCODING=utf-8` on the subprocess) and skips `@pytest.fixture` / `@fixture` functions and classes with `__init__` (same as pytest collection). A parse failure emits one `parse-error` tool-error finding per file.

## Beyond static analysis

testscan is a cheap pre-filter for AI-written tests. **Mutation orchestration** (`testscan --mutate` → [mutmut](https://mutmut.readthedocs.io/) / cosmic-ray on `--diff`, survivors as findings) is **planned for the next release** — not in this release. Until then, run mutation tools separately on changed lines if you need an objective “does this test catch a bug?” check.

## False positives (short)

1. `no-assert` — no-raise names / `validate_*` bodies / `_assert_*` helpers are skipped; word `assert` in a comment may still count in heuristic mode.
2. `assert-true` — fires on `assert True` / other constants in a docstring or string (heuristic mode).
3. `mock-only-assert` — boundary paths and bare procedural SUT calls are skipped; assigned unused returns still warn.
4. `todo-test` — `pytest.skip` / `unittest.skip` without reason parsing; `assert False` only with `, "TODO"` / `pytest.fail("TODO")`.
5. `empty-test` — without AST: rough indent-based body parse; with AST more accurate, but helpers / `pytest.skip` alone do not make a test “non-empty”.
6. `assert-equals-same` — `assert 1 == 1`; `assert "x==y" == z` (first `==` inside a string); comparisons in comments.
7. `only-happy-path` — misses negative cases via custom helpers/fixtures without `raises` / `warns` / `assertRaises`.
8. `duplicate-test-name` — same bare name in different classes is OK (key is `qualname`); duplicates share class+name.
9. `snapshot-only` — any non-snapshot `assert ` line clears the rule; snapshot APIs outside the needle list are missed.
10. `overmocked-io` — line must mention `patch` plus an IO target; other mock styles may miss or over-fire.
11. `test-imports-implementation-private` — intentional private imports (white-box tests) still note; `_UPPER_CASE` constants, `tests.*`, and relative `test_*` helpers are ignored; top-level stdlib `import _thread` / `_ast` is ignored; one finding per import statement.
12. `no-behavior-change` — a single value assert anywhere in the file clears it; type-check helpers not named `isinstance`/`type` are missed.

## Tests

```bash
go test ./...
```

## License

MIT
