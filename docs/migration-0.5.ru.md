# Миграция с 0.4 на 0.5 (и заметки про 0.6)

[English](migration-0.5.md) | Русский

В testscan 0.5 изменились вид по умолчанию и метрики. Здесь собраны детали миграции, которые раньше жили в разделе Flags README, плюс изменение гейта **0.5 → 0.6** для семантических правил.

## Вид по умолчанию

- **Focus включён по умолчанию.** Остаются только error/warning с весом precision ≥ `MinPrecisionForGrade` (0.15); notes, правила с нулевой точностью и набор демотированных эвристик отбрасываются. `--focus` — **deprecated no-op** (предупреждение в stderr). `--all` отключает focus и показывает все тиры.
- **Тиры точности.** Правила: `actionable`, `provisional`, `deterministic` (с 0.6) или `low`. По умолчанию показываются actionable, provisional и deterministic; `--show-low-precision` добавляет `low`, `--all` — всё.
- В **0.5** estimated-правила с precision 1.0 (включая empty-test, assert-tuple, …) были **low** и скрыты. В **0.6** шесть семантических правил снова **deterministic** и видны — без учёта в `actionable_count`. Остальные `est(1.0)`-эвристики (`todo-test`, `swallowed-exception`, `snapshot-only`, …) остаются low.

## Обновление с 0.5 на 0.6

- Шесть правил **deterministic** (`assert-tuple`, `fake-mock-assert`, `duplicate-test-name`, `empty-test`, `assert-equals-same`, `skip-without-reason`) снова **видны по умолчанию** и влияют на `--fail-on` / SARIF / Code Quality.
- Они по-прежнему **не** увеличивают `actionable_count` и тренд `--compare` (тренд только по actionable). Гейт смотрит на severity **показанного** набора; `deterministic_count` в header/JSON — отдельно.
- Deprecated JSON `health_score` может упасть, когда эти находки снова в shown set (полный вес precision).
- Если нужен тихий гейт как в 0.5: `--baseline`, `--disable <id>` или pin `testscan@0.5.x`.

## Гейт в CI

В виде **0.5** неизмеренные estimated-правила не влияли на `--fail-on`. Чтобы сохранить гейт как в 0.4 по всему, добавьте `--all` в команду CI.

С **0.6** шесть deterministic-правил входят в shown set по умолчанию и **влияют** на `--fail-on` (см. выше). Остальные low-эвристики скрыты, пока нет `--show-low-precision` / `--all`.

Вид по умолчанию сообщает, что скрыл (`· N hidden (use --all)`, JSON `summary.hidden_count`). С 0.6 в stderr всегда печатается подсказка, если что-то скрыто; если скрытые находки сработали бы на `--fail-on`, добавляется вторая строка gate-warning. Рецепты agent-hook не должны сливать этот stderr в payload модели.

`parse-error` (файл, который не удалось проанализировать) показывается всегда. Правила, запрошенные явно через `--enable` / `--rule`, обходят display-фильтры.

Сравнение с отчётом до 0.5 даёт тренд `incomparable` (`trend: n/a`).

## Deprecated-флаги

| Флаг | Поведение |
|------|-----------|
| `--focus` | Deprecated no-op; focus всегда включён, пока нет `--all`. Пишет предупреждение в stderr. |
| `--show-grade` | Deprecated no-op. Health Score / оценка A–F больше не выводятся в text и HTML; JSON по-прежнему содержит `health_score`, `grade`, `grade_deprecated: true`. |

Порядок: scan → baseline → focus → фильтр по тирам → score (+ trend) → emit.

## Плотность и JSON

- Плотность — **actionable-находки на 100 тестов** (не на `log10(files+1)`).
- JSON summary: `actionable_count`, `actionable_per_100_tests`, `provisional_count`, `deterministic_count` (с 0.6), `shown_count`, `tests`, `hidden_count`.
- `confirmed_count` / `confirmed_density` — **deprecated** синонимы `actionable_count` / `actionable_per_100_tests`.
- У находки могут быть аддитивные поля `tier` и `precision`.

## Go API (пользователи библиотеки)

См. раздел **Go API** в [CHANGELOG.md](../CHANGELOG.md) для 0.5.0: удалены `CountConfirmed`, `FilterLowPrecision`, `MinPrecisionForDisplay`, `Score.ShowGrade`; `RulePrecision` теперь `map[string]PrecisionInfo`; `FilterForDisplay` / `HiddenByDisplay` / `DisplayOptions` заменяют старые focus-хелперы. `Score.ConfirmedCount` / `ConfirmedDensity` остаются deprecated-синонимами actionable-значений.
