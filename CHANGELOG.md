# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Confirmed metrics + trend:** summary / text / HTML emphasize `confirmed_count` and `confirmed_density` (confirmed = rule precision ≥ `MinPrecisionForDisplay` = 0.3, after baseline + display filter; same `log10(files+1)` density denom as Health Score). `--compare path.json` trends vs a previous JSON report (`improved` / `worsened` / `unchanged` + deltas). If `--compare` is omitted but `--baseline` is set, the baseline file is the compare point. When `--baseline` is set (with or without a separate `--compare`), trend **current** uses pre-baseline confirmed metrics so suppressing known noise cannot fake an “improved” trend; `summary.confirmed_*` remains post-baseline (new issues only). Marked in JSON as `current_pre_baseline`, in text/HTML as `[pre-baseline]` / caption.
- `--show-grade`: show deprecated Health Score / A–F grade in text and the full HTML ring (grade remains in JSON with `grade_deprecated: true`).
- `--show-low-precision` / `show-low-precision = true`: emit findings below the 0.3 display floor (default: omit).
- `--enable ID` / `enable = ["…"]`: turn on opt-in project rules from `rules.Optional()` (off by default). Symmetrical with `--disable`.
- Opt-in rules: `error-contract-assert`, `raises-without-check`, `missing-mirror-test`, `rbac-mutation-guard` (configurable under `[rules.<id>]`).
- `wall-clock-in-test`: separate rule for `datetime.now` / `date.today` without freeze (note, precision ~0.50). `sleep-in-test` is only `time.sleep` / `asyncio.sleep` (warning, precision 1.0).
- HTML report UX: `vscode://file/…:line` links, default grouping **by file → findings** (toggle By rule), repeated messages collapsed to `×N`, hero shows confirmed density + trend.

### Changed

- Display / confirmed floor is **0.3** (`MinPrecisionForDisplay`); findings below the floor are not emitted unless `--show-low-precision`. `--focus` still uses `MinPrecisionForGrade` (0.15) and additionally drops notes / `neverFocusRules`.
- `name-body-mismatch`: narrower negative-name cues (fewer FP on names like `…_empty_…`).
- `no-assert`: mid-name no-raise tokens limited to `swallows|ignores|skips|allows|tolerates`; `ok`/`passes`/`accepts` stay edge-only (avoids FN on `test_password_passes_checks`).
- `overbroad-equality`: skips API-contract compares against `response.data` / `resp.data` / `*.json()`-like bodies.
- Opt-in rules have catalog precision entries (&lt; 1.0) until corpus-calibrated.
- `error-contract-assert`: `error-status-only = false` also gates on error-shaped tests (name / `errors` payload), not only non-2xx status asserts.
- Pipeline order remains: scan → baseline → focus → low-precision filter → score (+ trend) → emit.
- Daily CI docs: prefer `--diff` + `--baseline` + `--focus` (+ optional `--compare` / `codequality` / `sarif`).
### Deprecated

- **Health Score / A–F grade** as the primary summary signal: still computed and present in JSON (`health_score`, `grade`, `grade_deprecated: true`); text/HTML default to confirmed density + trend. Use `--show-grade` for the old emphasis. Planned removal of grade as a primary metric in a later release.

### Planned

- Mutation orchestration (`testscan mutate` → mutmut/cosmic-ray on `--diff`, survivors as findings) — separate subcommand; **not** in this release.

## [0.3.0] - 2026-09-28

Signal/noise + actionable UX for daily CI (`--diff` / `--baseline` / `--focus`). Demotes noisy heuristics to notes; Health Score is precision-weighted.

### Added

- **Precision-weighted Health Score**: error/warning findings are weighted by per-rule precision (precision < `MinPrecisionForGrade` = 0.15 → weight 0 for grade; mid-precision rules count proportionally); notes and `parse-error` tool errors are UI-only.
- `--focus`: keep only error/warning findings with rule precision ≥ `MinPrecisionForGrade` (0.15); drops notes, `parse-error`, zero-precision rules, and demoted heuristics listed in `neverFocusRules` (even if config bumps severity). Score is computed on the focused set. Compatible with `--baseline` / `--diff` (order: scan → baseline → focus → score → emit).
- HTML report: TOC/sections sorted by **precision desc** (then count, then id) with precision badges; note/tool-error filters off by default; hero caption clarifies Grade ≠ “tests are excellent”.
- HTML report: separate **tool error** chip/filter (not mixed into note), ±5-line code snippets, near-duplicate twin links (`qual:line`), collapsed repeated messages within a rule.
- `--diff <base-ref>`: scan only test files changed/added since the git base (PR-friendly for AI-generated tests). Accepts `main` or `origin/main...HEAD`.
- `--format codequality`: GitLab Code Quality / Code Climate JSON report.
- Synthetic corpus precision gate (`TestCorpus`): each rule’s hit/clean fixtures must stay ≥95% precision in CI.
- `--open` without `-o` writes to `.testscan/reports/report_<timestamp>.html` and ensures a local `.gitignore` (reports stay untracked).
- AI-oriented rules: `name-body-mismatch`, `self-patched-sut`, `expected-recomputed`, `commented-assert`, `overbroad-equality`.

### Changed

- Default severity demoted to **note** (noise / zero-precision UI honesty): `name-body-mismatch`, `no-assert`, `mock-only-assert`, `mock-tautology`, `overbroad-equality`.
- **Migration for `--fail-on warning`:** demoted rules (and wall-clock `sleep-in-test` notes) no longer fail CI; use baseline / note triage / config severity if you still want those gates.
- `no-assert`: skips no-raise names / `validate_*`-style bodies; follows same-module `_assert_*` / assert helpers.
- `mock-tautology`: flags only asserts on the mock itself or self-patched targets, not SUT calls that echo a dependency `return_value`.
- `mock-only-assert`: skips procedural SUT, constructor patches, and boundary paths (`adapters/`, `clients/`, …).
- `broad-raises`: message asks for a concrete exception class; skips specific exceptions with `exc_info.value.<attr>` checks.
- `assert-in-emptyable-loop`: flags only emptyable (`other`) iterables; skips nonempty literals, `range(N≥1)`, `UPPER_CASE` / enum consts, and pre-loop `assert iter` / `len > 0`.
- `weak-assert`: does not flag `len(x) == N` (N≠0) or `is_valid` in `accepts_*`/`passes_*`/`valid*`; adds shallow `rejects_*` / lone `is not None` / GET `status_code == 200` without body checks.
- `only-happy-path`: groups by SUT call (not only file); expanded negative signals (`status.HTTP_4xx_*`, names, `side_effect`, `not in` / `!=` / empty containers, `caplog` WARNING/ERROR); shorter messages; skips pure mappers without branches.
- `near-duplicate-test`: skips opposite polarity / different SUT / different enum literals; clusters of 3+ emit one finding with a `@pytest.mark.parametrize` sketch; twin qualname + line on pairs.
- `test-imports-implementation-private`: one finding per import statement; skips `_UPPER_CASE` constants; message frames the module as a submodule candidate.
- `assert-true`: also flags constant truthy asserts (`assert 1`, `assert "…"`).
- `sleep-in-test`: `time.sleep` / `asyncio.sleep` stay **warning**; `datetime.now` / `date.today` without freeze are **note**.
- `expected-recomputed`: does not flag determinism checks `f(x) == f(x)` (same call both sides).
- Health Score: removed parse penalty from grade; `parse_skipped` remains in JSON/HTML for tool visibility only.
- Daily workflow docs: prefer `--diff` + `--baseline` + `--focus`; full scan without `--focus` for heuristic triage.

### Fixed

- AST helper encoding: parse via UTF-8 bytes / `PYTHONUTF8=1` so Cyrillic sources no longer fail under `PYTHONIOENCODING=cp1251`.
- Fixtures named `test_*` with `@pytest.fixture` / `@fixture` are not collected as tests; classes with `__init__` skipped (pytest-aligned).
- HTML finding search is case-insensitive on both needle and haystack text.

## [0.2.0] - 2026-09-25

Accuracy roadmap + Health Score. CI that parsed `--format json` as a bare findings array must switch to the `findings` key (see **Changed**).

### Changed

- **Breaking:** `--format json` is now an object `{"summary":{…},"findings":[…]}` instead of a bare findings array. Baselines and CI parsers should read `findings` (or pass the file to `--baseline`, which still accepts both shapes). There is no `json-raw` format.
- Finding identity for baselines uses `fingerprint` (`rule` + relative file + `qual_name` + normalized snippet hash). Legacy baselines without fingerprints still match on `file+line+rule` once, with a one-time stderr warning; new baselines always write fingerprints.
- Paths in findings / HTML / SARIF are slash-normalized and relative to the project root or cwd.
- HTML output no longer auto-writes and opens on Windows for `--format html`; use `-o`/`--output` and optional `--open` (default path: `.testscan/reports/`).
- AST-backed rules no longer silently fall back to text heuristics when the model is unavailable (unless `Options.AllowHeuristicFallback`). A parse failure yields a single `parse-error` / `ast-unavailable` finding (`note`) per file.
- Several default rules are more precise (per-test / qualname / AST asserts): `duplicate-test-name`, `assert-equals-same`, `todo-test`, `only-happy-path` (default severity `note`), `no-assert`, `mock-only-assert`, `overmocked-io`, `empty-test`, `test-imports-implementation-private`, `assert-true`.

### Added

- **Health Score** (0–100, grades A–F): text footer `Health Score: N (G)`; JSON `summary`; HTML hero ring. Does not replace `--fail-on`.
- Batch AST helper (JSONL over a long-lived Python process) for much faster multi-file scans.
- Richer per-test AST model (`qualname`, decorators, fixtures, asserts, calls, raises, try/except, …).
- Inline suppress: `# testscan: ignore[rule-id]` / `ignore[a,b]` / `ignore-file[…]`.
- Config: `exclude`, `assert-helpers`, `python-files` / `python-functions` / `python-classes`, `respect-gitignore`, per-rule severity / options, `[[overrides]]` by path; optional `[tool.pytest.ini_options]` discovery.
- CLI: `-o`/`--output`, `--open`, `--coverage` (for coverage-mode only-happy-path).
- Fingerprint fields on findings (`qual_name`, `fingerprint`); SARIF `partialFingerprints`.
- New rules: `fake-mock-assert`, `assert-tuple`, `broad-raises`, `swallowed-exception`, `assert-in-emptyable-loop`, `weak-assert`, `mock-tautology`, `sleep-in-test`, `skip-without-reason`, `near-duplicate-test`.
- Optional `only-happy-path` coverage mode (`mode = "coverage"` / `--coverage`) via coverage.py JSON.

### Fixed

- False positives on real suites (e.g. same test name in different classes, conditional skips, call-vs-call `assert` equals-same, private stdlib imports).
- `weak-assert`: no longer flags Call/method truthy or `assert not …`; default severity `note`.
- `test-imports-implementation-private`: ignores `tests.*` and relative test helpers; message includes the private name; default severity `note`.
- `near-duplicate-test`: default severity `note`.
- Health Score density factor (`ScoreDensityK`) lowered so a large note-heavy suite (mp-be-like) lands in B–C after the accuracy fixes above.

## [0.1.0] - 2026-09-24

Initial tagged release: Go CLI + PyPI launcher, default AI-test smell rules, text/JSON/SARIF/HTML output, baseline, config, and pre-commit docs.

[Unreleased]: https://github.com/Nikita527/testscan/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/Nikita527/testscan/releases/tag/v0.3.0
[0.2.0]: https://github.com/Nikita527/testscan/releases/tag/v0.2.0
[0.1.0]: https://github.com/Nikita527/testscan/releases/tag/v0.1.0
