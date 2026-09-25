# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

Major release candidate (accuracy roadmap + Health Score). CI that parsed `--format json` as a bare findings array must switch to the `findings` key (see **Changed**).

### Changed

- **Breaking:** `--format json` is now an object `{"summary":{…},"findings":[…]}` instead of a bare findings array. Baselines and CI parsers should read `findings` (or pass the file to `--baseline`, which still accepts both shapes). There is no `json-raw` format.
- Finding identity for baselines uses `fingerprint` (`rule` + relative file + `qual_name` + normalized snippet hash). Legacy baselines without fingerprints still match on `file+line+rule` once, with a one-time stderr warning; new baselines always write fingerprints.
- Paths in findings / HTML / SARIF are slash-normalized and relative to the project root or cwd.
- HTML output no longer auto-writes and opens on Windows for `--format html`; use `-o`/`--output` and optional `--open`.
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

[Unreleased]: https://github.com/Nikita527/testscan/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/Nikita527/testscan/releases/tag/v0.1.0
