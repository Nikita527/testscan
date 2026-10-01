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
testscan tests --diff origin/main --format html --open

# Snapshot or refresh baseline noise on the whole tree, then gate the PR
testscan tests --format json --fail-on never > .testscan/baseline.json
testscan tests --diff origin/main --baseline .testscan/baseline.json --fail-on warning

# Trend vs yesterday’s JSON (or use --baseline alone as the compare point)
testscan tests --format json -o .testscan/prev.json --fail-on never
testscan tests --compare .testscan/prev.json --format text

# Everything: no focus filter, every precision tier (triage low-precision heuristics / notes)
testscan path/to/tests --all --format html --open

# Opt-in project rules (DRF / API-style)
testscan tests --enable error-contract-assert --enable raises-without-check

testscan path/to/tests --format text --fail-on error
testscan path/to/tests --format json --fail-on never
testscan path/to/tests --format sarif --fail-on never > testscan.sarif
testscan path/to/tests --format codequality -o gl-code-quality-report.json
testscan path --rule assert-equals-same
testscan path --disable weak-assert --disable empty-test
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

- `--format text|json|sarif|html|codequality|agent` (default: `text`) — `agent` prints one line per finding (`file:line rule — problem → fix`) plus a one-line summary, for AI agents and hooks (see [docs/agent-hook.md](docs/agent-hook.md)); every finding carries a concrete `fix` hint (JSON `fix`, SARIF help/`properties.fix`, HTML and text "fix" line); `html` is a self-contained interactive report (one headline number: **actionable** findings + trend; a small secondary line with provisional count and tests scanned; per-finding tier badge and rule precision; TP/FP labelling buttons; filters for **error / warning / note / tool error**; search; default grouping **file → findings**, toggle By rule; `vscode://file/…:line` links; repeated messages as `×N` — one TP/FP click labels the whole group; code snippets ±5 lines; near-duplicate twin links). Notes and tool errors are unchecked by default. `codequality` is GitLab Code Quality (Code Climate JSON). Write with `-o`/`--output`, or redirect stdout (`> report.html`).
- `text` starts with one header line, e.g. `testscan: 7 actionable findings (0.19 per 100 tests) · 6 provisional · 3650 tests in 382 files` (plus ` · trend improved (Δ -2)` when comparing), then one line per finding; provisional findings carry a `[provisional]` tag. `json` is `{"summary":{actionable_count,actionable_per_100_tests,provisional_count,shown_count,tests,files,trend?,errors,warnings,notes,parse_skipped,warnings_in_grade,warnings_ignored,health_score,grade,grade_deprecated,confirmed_count,confirmed_density},"findings":[…]}`; every finding additionally has `tier` (`actionable|provisional|low`) and `precision` (`{value,n,source,wilson_lower}`) — additive fields. `confirmed_count` / `confirmed_density` are **deprecated** aliases of `actionable_count` / `actionable_per_100_tests` (kept for one release); `health_score` / `grade` are **deprecated** (`grade_deprecated: true`) and are no longer rendered in text or HTML. **notes** are UI-only. **`parse-error` / AST failures** are **tool errors**: `parse_skipped` / separate HTML chip — they never count as actionable or provisional.
- `-o` / `--output PATH` — write report to a file (any format)
- `--open` — open HTML report in the default browser; without `-o` writes to `.testscan/reports/report_<timestamp>.html` and creates a local `.gitignore` so reports stay out of git
- `--fail-on error|warning|never` (default: `error`) — exit `1` if any finding ≥ threshold; CLI errors → exit `2`. **Migration:** in v0.5.0 `name-body-mismatch`, `no-assert`, `mock-only-assert`, and `mock-tautology` are now **opt-in** (moved from default); use `--enable` / `enable` to activate them. The default view shows only actionable and provisional findings (see Precision tiers); `wall-clock-in-test` and `overbroad-equality` remain default at **note** severity.
- `--rule ID` (repeatable) — only these rules (from `All()` = Default ∪ Optional); omit to use `Default()` plus `--enable`
- `--enable ID` (repeatable) — turn on opt-in Optional rules; merged with config `enable`
- `--disable ID` (repeatable) — turn rule(s) off; merged with config `disable`
- same ID in both `--rule`/`--enable` and `--disable` → error, exit `2`
- `--baseline path.json` — suppress findings matching baseline by fingerprint (legacy `file+line+rule` still works once with a warning); also used as the trend compare point when `--compare` is omitted. When baseline is set, trend **current** is pre-baseline actionable (so a large baseline cannot fake `improved` vs `--compare`); emitted `actionable_*` stay post-baseline.
- `--compare path.json` — trend of actionable count / density vs a previous JSON report (wrapper or findings array). Reports written before 0.5 only carry `confirmed_*`: they are read as a count-only compare point (density units differ), so the first trend after upgrading is approximate.
- `--all` — show everything: disables the focus filter **and** shows every precision tier (and notes / tool errors). This is the old pre-0.5 default output. Config: `all = true`.
- `--show-grade` — **deprecated no-op** (prints a warning to stderr). Health Score / A–F grade is no longer rendered in text or HTML; JSON still carries `health_score`, `grade`, `grade_deprecated: true`.
- `--show-low-precision` — additionally show **low**-tier findings (rules that are estimated / unmeasured / below the provisional bar) while keeping the focus filter (default: hidden). Config: `show-low-precision = true`.
- `--diff <base-ref>` — scan only test files changed or added since `base-ref` (`git diff --name-only --diff-filter=ACMR`). Primary PR workflow for reviewing AI-generated tests; use `origin/main` or `origin/main...HEAD`.
- `--focus` — **deprecated no-op** (prints a warning to stderr): the focus filter is now the default. Focus keeps only error/warning findings whose rule precision weight is ≥ `MinPrecisionForGrade` (0.15) and drops `note`, zero-precision rules and a fixed set of demoted heuristics even if config bumps their severity. Use `--all` to disable it. `parse-error` is always shown. Pipeline order: scan → baseline → focus → tier filter → score (+ trend) → emit.
- `--workers N` — parallel file checks (`0` → `runtime.NumCPU()`)

**Already available for CI:** `--diff` + `--baseline` + `--format codequality|sarif|json`. Knobs: `--enable`, `--compare`, `--show-low-precision`, `--all`. The exit code (`--fail-on`), SARIF and Code Quality output are all evaluated on the **shown** findings (after baseline and display filters) — with the default view that means only actionable/provisional rules can fail a build; use `--all` to gate on everything.

**Migrating CI from 0.4 to 0.5:** deterministic but not-yet-measured rules (empty-test, assert-tuple, swallowed-exception, …) are hidden by default and no longer affect `--fail-on`; add `--all` to your CI command to keep the 0.4 gating behaviour. The default view reports what it hid (`· N hidden (use --all)`, JSON `summary.hidden_count`) and prints one stderr line when hidden findings would have tripped `--fail-on`. `parse-error` (a file that could not be analyzed) is always shown. Rules requested explicitly via `--enable` / `--rule` bypass the display filters. Comparing against a pre-0.5 report gives trend `incomparable` (`trend: n/a`). Go library users: see "Go API" in the CHANGELOG.

### Precision tiers, density and labelling

testscan only trusts rules whose precision has been **measured** on labelled findings. Each rule falls in one tier (thresholds are exported constants in `scan/score.go`):

| Tier | Condition | Shown by default |
|------|-----------|------------------|
| **actionable** | measured, Wilson 95% lower bound ≥ `ActionableMinWilson` (0.7) and N ≥ `ActionableMinN` (20) | yes — counted in the headline number |
| **provisional** | measured, precision ≥ `ProvisionalMinPrecision` (0.8) and N ≥ `ProvisionalMinN` (5), not actionable | yes — tagged `[provisional]` / badge |
| **low** | everything else: estimated, unmeasured, unknown or below the bars | no (`--show-low-precision` or `--all`) |

N is the number of labelled true + false positives of the rule. Rules without measured data (including every rule that is only *estimated* in the catalog) are **low**: they no longer count as findings you should act on.

**Density** is actionable findings per **100 tests** (test functions and class methods found by the AST helper): `0.19 per 100 tests`. With 0 tests (or when no selected rule needs the AST) density is `0`.

**Default view** = focus filter + actionable and provisional tiers. `--show-low-precision` adds the low tier, `--all` removes every filter.

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

## License

MIT
