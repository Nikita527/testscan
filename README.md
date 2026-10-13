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

## See it

Default text output surfaces **deterministic** smells (empty tests, tuple asserts, fake mock methods, …) with a concrete fix line:

```text
testscan: 0 actionable findings (0.00 per 100 tests) · 0 provisional · 3 deterministic · 4 tests in 3 files
rules/testdata/empty/hit/test_empty.py:1: error empty-test: empty test function: test_vacuous [deterministic]
  → fix: add the missing arrange/act/assert body, or delete the test
rules/testdata/assert_tuple/hit/test_tuple.py:3: warning assert-tuple: assert on a tuple is always True; did you mean a comparison? [deterministic]
  → fix: remove the parentheses/comma so the assert compares values: `assert a == b`
rules/testdata/fake_mock_assert/hit/test_fake.py:2: warning fake-mock-assert: suspicious mock assert (typo or non-existent method) [deterministic]
  → fix: fix the mock assertion to a real method, e.g. `mock.assert_called_once_with(...)`
```

HTML reports add tier badges and TP/FP labelling (`--format html --open`). Optional screenshot: [docs/assets/](docs/assets/).

## Quick Start

```bash
uvx testscan@latest tests/                          # PyPI wheel — no Go required
testscan tests --diff origin/main --format html --open
testscan tests --format json --fail-on never > .testscan/baseline.json
testscan tests --diff origin/main --baseline .testscan/baseline.json --fail-on warning
testscan tests --format agent                       # one line per finding for AI hooks
```

Install with Go: `go install github.com/Nikita527/testscan/cmd/testscan@latest`. Local embed for contributors: [python/README.md](python/README.md).

## Use with AI agents

`--format agent` prints one compact line per finding (`file:line rule — problem → fix`) plus a summary — meant for Claude Code / Cursor / Windsurf hooks that feed findings back into the same turn:

```text
tests/test_orders.py:42 sleep-in-test — time.sleep in test → remove `time.sleep(...)`; wait on the condition or use a fake clock
tests/test_orders.py:88 empty-test — empty test function: test_vacuous → add the missing arrange/act/assert body, or delete the test
testscan: 2 findings (1 error, 1 warning) · 31 tests
```

Wire it as a post-edit hook on changed `test_*.py` files. Recipes: [docs/agent-hook.md](docs/agent-hook.md).

## What it catches (sample)

| Rule | Smell | Hit | Clean |
|------|-------|-----|-------|
| `empty-test` | empty `test_*` body | `def test_vacuous(): pass` | `assert 1 == 1` in the body |
| `assert-tuple` | `assert (x == 1, "msg")` always true | parentheses + comma | `assert x == 1, "msg"` |
| `fake-mock-assert` | typo / non-existent mock assert | `mock.assert_called_once_wiht()` | `mock.assert_called_once_with()` |
| `assert-equals-same` | assert compares expr to itself | `assert x == x` | `assert x == y` |

Full hit/clean examples for every rule: [docs/rules.md](docs/rules.md).

### Flags

- `--format text|json|sarif|html|codequality|agent` (default: `text`) — `agent` for AI hooks; `html` interactive report; `codequality` = GitLab Code Quality. Write with `-o`/`--output` or redirect stdout.
- `--fail-on error|warning|never` (default: `error`) — exit `1` if any **shown** finding ≥ threshold; CLI errors → exit `2`.
- `--diff <base-ref>` — only test files changed since `base-ref` (PR workflow).
- `--baseline path.json` / `--compare path.json` — suppress known findings / trend vs a previous JSON report.
- `--all` — show everything (pre-0.5 full output). `--show-low-precision` — also show **low**-tier rules (focus stays on).
- `--rule` / `--enable` / `--disable` — select rules; same ID in enable and disable → exit `2`.
- `--workers N` — parallel file checks (`0` → `runtime.NumCPU()`).
- `-o` / `--output PATH`, `--open` — write / open HTML (default path under `.testscan/reports/`).

CI: `--diff` + `--baseline` + `codequality|sarif|json`. Exit code, SARIF and Code Quality use the **shown** set (actionable / provisional / deterministic by default). `--compare` trend stays **actionable-only** — a spike in `deterministic_count` can fail the gate without moving trend.

**0.4 → 0.5 / 0.5 → 0.6 migration** (deprecated flags, deterministic gate): [docs/migration-0.5.md](docs/migration-0.5.md).

### Precision tiers, density and labelling

Each rule falls in one tier (thresholds in `scan/score.go`):

| Tier | Condition | Shown by default |
|------|-----------|------------------|
| **actionable** | measured, Wilson 95% lower bound ≥ `ActionableMinWilson` (0.7) and N ≥ `ActionableMinN` (20) | yes — counted in the headline number |
| **provisional** | measured, precision ≥ `ProvisionalMinPrecision` (0.8) and N ≥ `ProvisionalMinN` (5), not actionable | yes — tagged `[provisional]` / badge |
| **deterministic** | catalog `SourceDeterministic` — follows from Python/pytest semantics (allowlist of 6 rules) | yes — tagged `[deterministic]` / badge; **not** in `actionable_count` |
| **low** | everything else: estimated, unmeasured, unknown or below the bars | no (`--show-low-precision` or `--all`) |

N is the number of labelled true + false positives of the rule. Estimated catalog entries (including remaining `est(1.0)` heuristics) are **low**.

**Density** is actionable findings per **100 tests**: `0.19 per 100 tests`. With 0 tests density is `0`. **Trend** (`--compare`) uses the same actionable metrics; gate/`deterministic_count` are separate.

**Default view** = focus filter + actionable, provisional and deterministic tiers. `--show-low-precision` adds low, `--all` removes every filter.

**Measured precision.** Label findings, then compute per-rule precision:

1. Write an HTML report (`testscan tests --all --format html -o report.html`) and mark findings **TP** or **FP** with the buttons next to each finding (a collapsed `×N` row labels every finding in the group; state is kept in the browser's localStorage). **Export labels** downloads `labels.json`.
2. `labels.json` (default location `.testscan/labels.json`) is a JSON array; the last entry per fingerprint wins:

   ```json
   [
     {"fingerprint": "9d17e1409e5c531b81c3858012540f41", "rule": "empty-test",
      "file": "tests/test_a.py", "line": 12, "label": "tp", "note": "optional"}
   ]
   ```

   `label` is `tp`, `fp` or `skip` (skips are ignored by the statistics).
3. `testscan precision --labels .testscan/labels.json [--report out.json] [--format text|json]` prints per rule `N`, `TP`, `FP`, precision and the Wilson 95% lower bound. With `--report` (a JSON report from `--format json`) only labels whose fingerprint appears in that report are counted, so the number describes exactly what that run shows.
4. Copy measured values into the catalog (`scan.RulePrecision`, e.g. `scan.Measured(tp, n)`); the tier is derived from them.


### pre-commit

See [docs/pre-commit.md](docs/pre-commit.md) (mp-be example included). Short form with `uvx`:

```yaml
# .pre-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: testscan
        name: testscan
        entry: uvx testscan@0.5.0   # pin a release; see docs/pre-commit.md
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
# show-low-precision = true  # also show low-tier findings (estimated / unmeasured rules); focus stays on
# all = true                 # show everything (same as --all): no focus filter, all tiers
# defer_to_ruff = false      # report broad-raises even when ruff enables PT011 (default: true, see docs/rules.md)

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

`scan.Options.Workers` is the pool size for `Check` per file (Walk stays sequential). The exit code stays severity-based (`--fail-on`) and is evaluated on the shown findings.

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
| assert-true | warning | `assert True` / `assert 1` / `assert "…"`, or `assertTrue`/`assertFalse` with a comparison |
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
| sleep-in-test | warning | `time.sleep` / `asyncio.sleep` |
| wall-clock-in-test | note | `datetime.now` / `date.today` without freeze |
| skip-without-reason | warning | skip/xfail without reason or strict |
| near-duplicate-test | note | near-identical bodies after literal norm; skips opposite polarity / different SUT / enums; 3+ → parametrize sketch |
| self-patched-sut | warning | patches the SUT under test and asserts the patch |
| expected-recomputed | warning | assert RHS recomputes expected via SUT/helper call (not `f(x)==f(x)` determinism) |
| commented-assert | note | commented-out `# assert` / `# self.assert` |
| overbroad-equality | note | assert equals a huge dict/list/tuple literal (skips `response.data` / `*.json()` contract bodies) |
| no-assert | *opt-in* | data-flow: no assert depends on the SUT result (traces through assignments/attributes/subscripts back to project-code calls) |
| mock-only-assert | *opt-in* | data-flow: assert on mock that is not wired into the SUT |
| mock-tautology | *opt-in* | data-flow: assert compares only values derived from mock configuration |
| name-body-mismatch | *opt-in* | test name implies negative case but body lacks error signal (status FAILED/REJECTED, error_code, non-empty errors/issues) |
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

1. `no-assert` — data-flow only traces back to SUT (calls imported from the project package); calls on fixtures or test-local helpers after the SUT count as observing state, so recall is intentionally low. No-raise names still skipped.
2. `assert-true` — fires on `assert True` / other constants in a docstring or string (heuristic mode).
3. `mock-only-assert` — data-flow only traces back to SUT (calls imported from the project package); recall is intentionally low, depends on control-flow recovery from the AST.
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

## Contributing

Build, AST helper, embed wheels, rule testdata, and precision/labels workflow: [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT
