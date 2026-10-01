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

# Trend vs yesterday’s JSON (or use --baseline alone as the compare point)
testscan tests --format json -o .testscan/prev.json --fail-on never
testscan tests --compare .testscan/prev.json --format text

# Full scan without --focus (triage low-precision heuristics / notes)
testscan path/to/tests --format html --open

# Opt-in project rules (DRF / API-style)
testscan tests --enable error-contract-assert --enable raises-without-check

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

- `--format text|json|sarif|html|codequality` (default: `text`) — `html` is a self-contained interactive report (confirmed density + trend in the hero; optional Health Score ring with `--show-grade`; filters for **error / warning / note / tool error**; search; default grouping **file → findings**, toggle By rule; `vscode://file/…:line` links; repeated messages as `×N`; code snippets ±5 lines; near-duplicate twin links). Notes and tool errors are unchecked by default. `codequality` is GitLab Code Quality (Code Climate JSON). Write with `-o`/`--output`, or redirect stdout (`> report.html`).
- `text` ends with `Confirmed: N (density X.XX)` and optional trend; `json` is `{"summary":{confirmed_count,confirmed_density,trend?,health_score,grade,grade_deprecated,errors,warnings,notes,files,parse_skipped,warnings_in_grade,warnings_ignored},"findings":[…]}` (breaking vs a bare array — use `findings` for baselines / CI parsers). **Confirmed** findings have rule precision ≥ `MinPrecisionForDisplay` (0.3) after baseline / display filter. **Health Score** (precision-weighted error+warning; floor `MinPrecisionForGrade` = 0.15) is **deprecated** as the primary signal — still in JSON with `grade_deprecated: true`; show in text/HTML via `--show-grade`. **notes** are UI-only. **`parse-error` / AST failures** are **tool errors**: `parse_skipped` / separate HTML chip — they do **not** affect confirmed metrics or grade.
- `-o` / `--output PATH` — write report to a file (any format)
- `--open` — open HTML report in the default browser; without `-o` writes to `.testscan/reports/report_<timestamp>.html` and creates a local `.gitignore` so reports stay out of git
- `--fail-on error|warning|never` (default: `error`) — exit `1` if any finding ≥ threshold; CLI errors → exit `2`. **Migration:** demoted rules (`name-body-mismatch`, `no-assert`, `mock-only-assert`, `mock-tautology`, `overbroad-equality`) and `wall-clock-in-test` are **note** by default, so `--fail-on warning` no longer fails on them; use notes triage, baseline, or config severity overrides if you still want gates.
- `--rule ID` (repeatable) — only these rules (from `All()` = Default ∪ Optional); omit to use `Default()` plus `--enable`
- `--enable ID` (repeatable) — turn on opt-in Optional rules; merged with config `enable`
- `--disable ID` (repeatable) — turn rule(s) off; merged with config `disable`
- same ID in both `--rule`/`--enable` and `--disable` → error, exit `2`
- `--baseline path.json` — suppress findings matching baseline by fingerprint (legacy `file+line+rule` still works once with a warning); also used as the trend compare point when `--compare` is omitted. When baseline is set, trend **current** is pre-baseline confirmed (so a large baseline cannot fake `improved` vs `--compare`); emitted `confirmed_*` stay post-baseline.
- `--compare path.json` — trend confirmed metrics vs a previous JSON report (wrapper or findings array)
- `--show-grade` — print deprecated Health Score / show full HTML grade ring
- `--show-low-precision` — emit findings with precision &lt; 0.3 (default: omit)
- `--diff <base-ref>` — scan only test files changed or added since `base-ref` (`git diff --name-only --diff-filter=ACMR`). Primary PR workflow for reviewing AI-generated tests; use `origin/main` or `origin/main...HEAD`.
- `--focus` — after baseline, keep only error/warning findings whose rule precision is ≥ `MinPrecisionForGrade` (0.15); drops `note`, `parse-error`, zero-precision rules, and a fixed set of demoted heuristics even if config bumps their severity. Score is computed on the focused set. Combine with `--diff` / `--baseline` for daily CI (order: scan → baseline → focus → low-precision filter → score → emit).
- `--workers N` — parallel file checks (`0` → `runtime.NumCPU()`)

**Already available for CI:** `--diff` + `--baseline` + `--focus` + `--format codequality|sarif|json`. New knobs: `--enable`, `--compare`, `--show-low-precision`, `--show-grade`.

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
enable = ["error-contract-assert", "raises-without-check"]
paths = ["tests"]
workers = 4
exclude = ["**/conftest.py"]
assert-helpers = ["assert_*", "check_*"]
python-files = ["test_*.py", "*_test.py"]
python-functions = ["test_*"]
python-classes = ["Test*"]
respect-gitignore = true
# show-low-precision = true  # emit findings below precision 0.3

[rules.only-happy-path]
severity = "note"
min-tests = 3
# mode = "coverage"          # optional: use coverage.py JSON instead of heuristics
# coverage = "coverage.json" # path to report (also: --coverage PATH)
# negative-names = ["invalid", "forbidden", "missing"]

[rules.todo-test]
severity = "warning"

# Opt-in API / DRF-style project rules (enable above or via --enable)
[rules.error-contract-assert]
error-code-path = "errors[].code"
# error-status-only = true

[rules.raises-without-check]
error-attr = "code"
# exception-classes = ["DomainError"]

[rules.missing-mirror-test]
source-glob = "app/**/domain/*.py"
mirror-template = "tests/{x}/domain/test_{m}.py"

[rules.rbac-mutation-guard]
# name-cues = ["rbac", "permission", "forbidden", "role", "protected"]
# mutating-methods = ["post", "put", "patch", "delete"]
# forbidden-signals = ["401", "403", "forbidden", "permission"]

[[overrides]]
path = "tests/integration/**"
disable = ["only-happy-path"]
```

Inline suppress: `# testscan: ignore[rule-id]` (or `ignore[a,b]`) on the finding line, the line before/`def` of the test, or `# testscan: ignore-file[rule-id]` / ignore in the module header.

CLI flags override config when set. If no path args are given, `paths` from config is used (else `.`). Config `paths` are resolved relative to the config file’s directory (not the process cwd). Config `disable` / `enable` are unioned with `--disable` / `--enable`.

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

## Rules (Default + Optional)

Hit/clean examples for every rule: [docs/rules.md](docs/rules.md).

Default rules are on unless `--disable` / `disable`. Opt-in rules are off until `--enable` / `enable`.

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
| sleep-in-test | warning | `time.sleep` / `asyncio.sleep` |
| wall-clock-in-test | note | `datetime.now` / `date.today` without freeze |
| skip-without-reason | warning | skip/xfail without reason or strict |
| near-duplicate-test | note | near-identical bodies after literal norm; skips opposite polarity / different SUT / enums; 3+ → parametrize sketch |
| name-body-mismatch | note | test name implies negative case but body has no negative signal |
| self-patched-sut | warning | patches the SUT under test and asserts the patch |
| expected-recomputed | warning | assert RHS recomputes expected via SUT/helper call (not `f(x)==f(x)` determinism) |
| commented-assert | note | commented-out `# assert` / `# self.assert` |
| overbroad-equality | note | assert equals a huge dict/list/tuple literal (skips `response.data` / `*.json()` contract bodies) |
| error-contract-assert | *opt-in* | non-2xx status assert without configured error-code path |
| raises-without-check | *opt-in* | `pytest.raises` without `match=` / domain error attr check |
| missing-mirror-test | *opt-in* | domain source file without mirror test path |
| rbac-mutation-guard | *opt-in* | RBAC-ish test mutates without asserting forbid (401/403 / raises) |

## Parsing

AST helper (batch) feeds rules that `NeedsAST()`; remaining rules use text heuristics. Walk skips venv/`__pycache__` by default and honors `exclude` + `.gitignore` (`respect-gitignore`).

The helper parses source as UTF-8 (forces `PYTHONUTF8=1` / `PYTHONIOENCODING=utf-8` on the subprocess) and skips `@pytest.fixture` / `@fixture` functions and classes with `__init__` (same as pytest collection). A parse failure emits one `parse-error` tool-error finding per file.

## Beyond static analysis

testscan is a cheap pre-filter for AI-written tests. **Mutation orchestration** (`testscan mutate` → [mutmut](https://mutmut.readthedocs.io/) / cosmic-ray on `--diff`, survivors as findings) is **planned** as a separate subcommand — **not** in this release. Until then, run mutation tools separately on changed lines if you need an objective “does this test catch a bug?” check.

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
