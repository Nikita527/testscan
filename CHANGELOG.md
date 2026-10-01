# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## 0.5.0 (unreleased)

Honest metrics and a quiet default: only rules with **measured** precision are trusted, and the default output shows only those.

### Breaking changes / Migration

- **Focus is now the default.** The old `--focus` filter (error/warning only, no notes / tool errors / zero-precision or demoted rules) always applies. Use **`--all`** (or `all = true` in config) to get the old full output; `--focus` is still accepted but is a deprecated no-op that prints a warning on stderr.
- **Precision tiers replace "confirmed".** Rules are `actionable` (measured, Wilson 95% lower bound >= 0.7, N >= 20), `provisional` (measured, precision >= 0.8, N >= 5) or `low` (everything else). By default only actionable and provisional findings are shown; `--show-low-precision` adds `low` (focus stays on), `--all` shows everything. Thresholds are exported constants in `scan/score.go`.
- **Unknown / unmeasured rules are no longer trusted by default.** Every catalog entry that is only *estimated* (and every rule missing from the catalog) is `low`, so its findings are hidden by default until precision is measured (label findings in the HTML report → `testscan precision`) and the catalog entry becomes `measured`. The shipped catalog is measured on the mp-be corpus (see below). Use `--all` if you need the previous behaviour.
- **Density is now per 100 tests**, not per `log10(files+1)`: `actionable_per_100_tests = actionable_count * 100 / tests` (0 when no tests were counted). Tests are counted from the AST, so they are 0 when no selected rule needs the AST.
- **JSON summary:** new `actionable_count`, `actionable_per_100_tests`, `provisional_count`, `shown_count`, `tests`. `confirmed_count` / `confirmed_density` are **deprecated** aliases of `actionable_count` / `actionable_per_100_tests` (note: the values changed meaning) and will be removed in the next release. Trend gains `prev_actionable_count` / `prev_actionable_density`; `prev_confirmed_*` stay as deprecated aliases with the same values. `--compare` still reads older reports that only have `confirmed_*` (count-only comparison, so the first trend after upgrading is approximate).
- **JSON findings** gain additive `tier` and `precision` (`value`, `n`, `source`, `wilson_lower`) fields.
- **Health Score / A-F grade removed from text and HTML.** JSON keeps `health_score`, `grade` and `grade_deprecated: true` for one more release. `--show-grade` is a deprecated no-op (stderr warning).
- **Text output** starts with a header line (`testscan: 7 actionable findings (0.19 per 100 tests) · 6 provisional · 3650 tests in 382 files`) instead of ending with `Confirmed: N (density X.XX)`; provisional findings are tagged `[provisional]`.
- **Exit code, SARIF and Code Quality** are evaluated on the **shown** findings (after baseline, focus and tier filtering). With the default view only actionable/provisional rules can fail `--fail-on`; use `--all` to gate on everything.
- **CI gating:** with the default view, deterministic but not-yet-measured rules (empty-test, assert-tuple, swallowed-exception, ...) are hidden and do not affect `--fail-on`; add `--all` to your CI command to keep the 0.4 gating behaviour. When hidden findings would have tripped `--fail-on`, one stderr line says so: `testscan: N findings at or above --fail-on <level> are hidden by the default view (unmeasured precision); use --all to gate on them`.
- **Hidden findings are counted:** JSON `summary.hidden_count` (additive) is the number of findings removed by the default filters (focus + tier; 0 with `--all`). The text header ends with `· N hidden (use --all)`; the HTML hero line and the agent summary line show it too.
- **`parse-error` is always shown** in the default view (exempt from focus and tier filtering) and counts toward `--fail-on` according to its severity (note). It stays out of the actionable / provisional counts. The header (text, HTML, agent) adds `N files not analyzed (parse error)` when N > 0.
- **Explicit `--enable <id>` / `--rule <id>`** (CLI or the config `enable` list) bypass the focus and tier display filters: the findings of a rule you asked for by name are shown (and can gate). Library: `scan.DisplayOptions.ExplicitRules` / `scan.ExplicitSet`.
- **Trend `incomparable`:** when the `--compare` / `--baseline` report predates 0.5 (only `confirmed_*` metrics, or a bare findings array) the trend direction is `incomparable` (JSON `trend.direction`) instead of improved / worsened; text and HTML show `trend: n/a (previous report uses pre-0.5 metrics)`. Deltas are still reported. Whenever the current test count or the previous density is unknown, the comparison is count-only.
- **Go API (library users):** removed `scan.CountConfirmed`, `scan.ConfirmedDensity`, `scan.FilterLowPrecision`, `scan.MinPrecisionForDisplay` and `Score.ShowGrade`; `scan.ComparePoint.ConfirmedCount` / `ConfirmedDensity` are renamed `ActionableCount` / `ActionableDensity` (new `Legacy` flag); `scan.RulePrecision` is now `map[string]PrecisionInfo` (was `map[string]float64`); `scan.TrendCurrentMetrics` now takes `DisplayOptions` and the test count instead of `focus, filterLowPrecision, fileCount`. `Score.ConfirmedCount` / `ConfirmedDensity` remain as deprecated aliases of the actionable values. Added: `scan.CountActionable`, `CountProvisional`, `CountTier`, `DensityPer100`, `FilterForDisplay`, `HiddenByDisplay`, `ComputeTrendPoint`, `Score.HiddenCount`.

### Precision before / after (mp-be)

Measured on `testdata/corpus/mp-be.labels.json` (382 files, 3649 tests). Cells: findings / TP / FP / precision / Wilson 95% lower bound.

| Rule | 0.4.0 default | 0.4.0 `--show-low-precision` | 0.5.0 default | 0.5.0 `--all` + opt-in |
|---|---|---|---|---|
| assert-in-emptyable-loop | 1 / 1 / 0 / 1.00 / 0.21 | 1 / 1 / 0 / 1.00 / 0.21 | — | 1 / 1 / 0 / 1.00 / 0.21 |
| broad-raises | 2 / 2 / 0 / 1.00 / 0.34 | 2 / 2 / 0 / 1.00 / 0.34 | — | 2 / 2 / 0 / 1.00 / 0.34 |
| commented-assert | 1 / 0 / 1 / 0.00 / 0.00 | 1 / 0 / 1 / 0.00 / 0.00 | — | — |
| mock-only-assert | — | 7 / 0 / 7 / 0.00 / 0.00 | — | — |
| name-body-mismatch | 52 / 0 / 52 / 0.00 / 0.00 | 52 / 0 / 52 / 0.00 / 0.00 | — | 1 / 0 / 1 / 0.00 / 0.00 |
| near-duplicate-test | 43 / 9 / 34 / 0.21 / 0.11 | 43 / 9 / 34 / 0.21 / 0.11 | — | 7 / 6 / 1 / 0.86 / 0.49 |
| no-assert | — | 4 / 0 / 4 / 0.00 / 0.00 | — | — |
| only-happy-path | 4 / 0 / 4 / 0.00 / 0.00 | 4 / 0 / 4 / 0.00 / 0.00 | — | — |
| overbroad-equality | 17 / 0 / 17 / 0.00 / 0.00 | 17 / 0 / 17 / 0.00 / 0.00 | — | 5 / 0 / 5 / 0.00 / 0.00 |
| sleep-in-test | 1 / 1 / 0 / 1.00 / 0.21 | 1 / 1 / 0 / 1.00 / 0.21 | — | 1 / 1 / 0 / 1.00 / 0.21 |
| test-imports-implementation-private | — | 39 / 7 / 32 / 0.18 / 0.09 | — | 39 / 7 / 32 / 0.18 / 0.09 |
| wall-clock-in-test | 10 / 6 / 4 / 0.60 / 0.31 | 10 / 6 / 4 / 0.60 / 0.31 | 6 / 6 / 0 / 1.00 / 0.61 | 6 / 6 / 0 / 1.00 / 0.61 |
| weak-assert | — | 13 / 0 / 13 / 0.00 / 0.00 | — | 13 / 0 / 13 / 0.00 / 0.00 |
| **total** | **131 / 19 / 112 / 0.15 / 0.09** | **194 / 26 / 168 / 0.13 / 0.09** | **6 / 6 / 0 / 1.00 / 0.61** | **75 / 23 / 52 / 0.31 / 0.21** |

Notes on the numbers:
- `wall-clock-in-test` and `near-duplicate-test` are **provisional** (precision ≥ 0.8 at n ≥ 5); no rule is **actionable** yet (needs Wilson lower ≥ 0.7 at n ≥ 20 — more labeled corpora are needed). `near-duplicate-test` is a note and is hidden by the default focus filter; it shows with `--all`.
- `broad-raises`, `sleep-in-test`, `assert-in-emptyable-loop` are 100% precise on mp-be but n < 5, so they are hidden by default until more labels exist (`--all` shows them).
- `name-body-mismatch` (0/52), `no-assert`, `mock-only-assert`, `mock-tautology` are now **opt-in**. The latter three were rewritten on the data-flow model and produce 0 findings on mp-be (all 10 previous FPs gone), so their precision is unmeasured.
- `near-duplicate-test` ignores tests longer than 15 lines (`nearDupMaxSpan`); this threshold was chosen while looking at the corpus, treat it as tunable.
- Labeling criterion for `near-duplicate-test` TP: adjacent tests, identical assertions, differing only in inputs, without distinct scenario docstrings.
- These measurements are in-sample: the rule fixes were developed against the same corpus. Treat provisional status as preliminary until confirmed on other labeled corpora.


### Added

- `--all` flag and `all = true` config key.
- **Ruff overlap:** `broad-raises` is skipped by default when the project's ruff config enables `PT011` (`select`/`extend-select` with `PT`, `PT0`, `PT01`, `PT011` or `ALL`; the most specific matching selector wins, a tie goes to ignore; any `per-file-ignores` entry covering PT011 or a prefix of it, or a malformed ruff config, counts as not enabled so testscan keeps the rule); set `defer_to_ruff = false` to keep it. Docs: "Overlap with ruff" in docs/rules.md.
- **`fix` hint on every finding:** one concrete imperative action (`scan.Finding.Fix`, JSON `fix`). Rules that know specifics set it themselves (wall-clock names the call and replacement, near-duplicate names the twin, sleep names the call); everything else gets a per-rule default from `scan.RuleFix`. Also shown in SARIF (`rules[].help`, `results[].properties.fix`), HTML and as an indented `  → fix:` line in text output.
- **`--format agent`:** one line per finding, `file:line rule — problem → fix`, plus a single summary line (`testscan: 6 findings (6 warning) · 3649 tests`); honors the default display filters and `--all`.
- Docs: [docs/agent-hook.md](docs/agent-hook.md) — Claude Code `PostToolUse` hook (bash and PowerShell) that scans edited test files and returns findings to the agent.
- `scan.RuleTier`, `scan.Measured`, `scan.FilterForDisplay`, `scan.Result.Tests`, `scan.CalculateScoreWithTests`.
- HTML: single headline number (actionable + trend), secondary provisional / tests line, per-finding tier badge and rule precision (`p=0.85 n=24 measured` / `estimated`).
- Docs: precision tiers, per-100-tests density, `testscan precision` and the labels.json / HTML labelling workflow.

### Fixed

- HTML labelling: a collapsed `x N` row now applies a TP/FP click to **all** fingerprints in the group (previously only the first).
- HTML report: the table-of-contents lookup no longer builds CSS selectors from file names (paths with `"` or `\` broke the script and the TP/FP labelling), and each init block is guarded so one failure cannot disable labelling/export.
- `wall-clock-in-test`: project timezone detection is bounded (20000 visited entries), stops once settings and `timezone.localdate` usage are resolved, and `from django.utils.timezone import ...` sets the now/localdate flags from the imported names.



## [0.4.0] - 2026-10-01

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

[Unreleased]: https://github.com/Nikita527/testscan/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/Nikita527/testscan/releases/tag/v0.4.0
[0.3.0]: https://github.com/Nikita527/testscan/releases/tag/v0.3.0
[0.2.0]: https://github.com/Nikita527/testscan/releases/tag/v0.2.0
[0.1.0]: https://github.com/Nikita527/testscan/releases/tag/v0.1.0
