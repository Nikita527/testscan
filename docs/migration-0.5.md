# Migrating from 0.4 to 0.5 (and notes for 0.6 / 0.8)

English | [Русский](migration-0.5.ru.md)

testscan 0.5 changed the default view and metrics. This page collects the migration details that used to live in the README Flags section, plus the **0.5 → 0.6** gate change for semantic rules and **Upgrading to 0.8.0**.

## Default view

- **Focus is the default.** Only error/warning findings whose rule precision weight is ≥ `MinPrecisionForGrade` (0.15) are kept; notes, zero-precision rules and a fixed set of demoted heuristics are dropped. `--focus` is a **deprecated no-op** (stderr warning). Use `--all` to disable focus and show every tier.
- **Precision tiers.** Rules are `actionable`, `provisional`, `deterministic` (from 0.6) or `low`. By default only actionable, provisional and deterministic findings are shown; `--show-low-precision` adds `low`, `--all` shows everything.
- In **0.5**, estimated rules at precision 1.0 (including empty-test, assert-tuple, …) were **low** and hidden. In **0.6**, six semantic rules are **deterministic** and shown again — without counting toward `actionable_count`. Remaining `est(1.0)` heuristics (`swallowed-exception`, `snapshot-only`, …) stay low. From **0.8**, `todo-test` and `assert-true` are also **deterministic** (see [Upgrading to 0.8.0](#upgrading-to-080)).

## Upgrading from 0.5 to 0.6

- The six **deterministic** rules (`assert-tuple`, `fake-mock-assert`, `duplicate-test-name`, `empty-test`, `assert-equals-same`, `skip-without-reason`) are **shown by default** and again affect `--fail-on` / SARIF / Code Quality.
- They still do **not** inflate `actionable_count` or `--compare` trend (trend remains actionable-only). Gate on severity of the **shown** set; watch `deterministic_count` in the header/JSON separately.
- Deprecated JSON `health_score` may drop when these findings reappear (they carry full precision weight).
- Mitigation if you need the quiet 0.5 gate: `--baseline`, `--disable <id>`, or pin `testscan@0.5.x`.

## Upgrading to 0.8.0

Precision-first default view after the 0.7.0 agent-repo benchmark (6 repos, ~42k tests — 3/30 default findings were true positives).

### Breaking

- **`todo-test`** and **`assert-true`** are tier **`deterministic`**: shown by default again and affect `--fail-on` / SARIF / Code Quality (they still do **not** count toward `actionable_count` / `--compare` trend).
- **`overbroad-equality`** is **opt-in** (off unless `--enable overbroad-equality` / `enable = ["overbroad-equality"]`).
- **`wall-clock-in-test`** is **hidden by default** (`neverFocus`, including note/warning). Re-enable with `--all`, or `--enable wall-clock-in-test` / `--rule wall-clock-in-test` (explicit rules bypass focus).
- **Finding fingerprints** include a 1-based **occurrence index** among identical rule+file+qualname+normalized-snippet shapes (stable under line shifts). Existing `--baseline` files need a **one-time re-save**.

### Display

- **`near-duplicate-test`** remains a provisional note shown by default; default **text** output collapses to **one summary line per file** (`--all` / JSON / SARIF keep every finding). The text header counts match the collapsed lines.

### Migration

If CI starts failing on newly visible `todo-test` / `assert-true` findings, fix them, add `# testscan: ignore[...]`, use `--baseline`, or `--disable todo-test --disable assert-true`. To keep huge-literal equality checks, add `overbroad-equality` to `enable`. Re-save baselines after upgrade.

## CI gating

With the **0.5** default view, unmeasured estimated rules did not affect `--fail-on`. To keep 0.4-style gating on everything, add `--all` to the CI command.

From **0.6**, the six deterministic rules are in the default shown set and **do** affect `--fail-on` (see above). Other low-tier heuristics stay hidden unless `--show-low-precision` / `--all`.

The default view reports what it hid (`· N hidden (use --all)`, JSON `summary.hidden_count`). From 0.6, stderr always prints one hint when anything is hidden; when hidden findings would have tripped `--fail-on`, a second gate-warning line is printed. Agent-hook recipes should not merge that stderr into the model payload.

`parse-error` (a file that could not be analyzed) is always shown. Rules requested explicitly via `--enable` / `--rule` bypass the display filters.

Comparing against a pre-0.5 report gives trend `incomparable` (`trend: n/a`).

## Deprecated flags

| Flag | Behaviour |
|------|-----------|
| `--focus` | Deprecated no-op; focus is always on unless `--all`. Prints a stderr warning. |
| `--show-grade` | Deprecated no-op. Health Score / A–F grade is no longer rendered in text or HTML; JSON still carries `health_score`, `grade`, `grade_deprecated: true`. |

Pipeline order: scan → baseline → focus → tier filter → score (+ trend) → emit.

## Density and JSON

- Density is **actionable findings per 100 tests** (not per `log10(files+1)`).
- JSON summary: `actionable_count`, `actionable_per_100_tests`, `provisional_count`, `deterministic_count` (0.6+), `shown_count`, `tests`, `hidden_count`.
- `confirmed_count` / `confirmed_density` are **deprecated** aliases of `actionable_count` / `actionable_per_100_tests`.
- Each finding may include additive `tier` and `precision` fields.

## Go API (library users)

See the **Go API** notes in [CHANGELOG.md](../CHANGELOG.md) for 0.5.0: removed `CountConfirmed`, `FilterLowPrecision`, `MinPrecisionForDisplay`, `Score.ShowGrade`; `RulePrecision` is now `map[string]PrecisionInfo`; `FilterForDisplay` / `HiddenByDisplay` / `DisplayOptions` replace the old focus helpers. `Score.ConfirmedCount` / `ConfirmedDensity` remain as deprecated aliases of the actionable values.
