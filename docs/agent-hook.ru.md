# Хуки для агентов

[English](agent-hook.md) | Русский

Запускайте testscan на каждом тестовом файле, который ИИ-агент создал или изменил, и возвращайте находки агенту, чтобы он исправил их в том же ходе.

## Вывод: `--format agent`

Одна строка на находку, без заголовков: `file:line rule — проблема → fix`, затем одна итоговая строка.

```text
tests/test_orders.py:42 sleep-in-test — time.sleep / asyncio.sleep in test; prefer event waits or freezegun / time-machine → remove `time.sleep(...)`; wait on the condition (poll with a timeout or an event) or use a fake clock
tests/test_orders.py:88 broad-raises — pytest.raises(Exception) is too broad; specify a concrete exception class → replace `pytest.raises(Exception)` with the specific exception, e.g. `pytest.raises(ValueError, match=...)`, and keep only the failing call in the block
testscan: 2 findings (2 warning) · 31 tests
```

Действует фильтр показа по умолчанию (actionable / provisional / deterministic); `--all` показывает всё. Тот же текст `fix` есть в `--format json` (`findings[].fix`), SARIF (`rules[].help`, `results[].properties.fix`), HTML и text.

**Stderr и payload агента:** если что-то скрыто, testscan пишет подсказку в stderr. **Не** сливайте stderr в находки для модели (`2>&1`). Захватывайте только **stdout** (или глушите stderr через `2>/dev/null`, если stderr процесса хука потом показывают агенту). Подсказка может остаться в логах хука.

---

## Claude Code

### Как работает хук

После `Write` / `Edit` Claude Code запускает команду `PostToolUse` и передаёт JSON в stdin (путь — `tool_input.file_path`). Код выхода `2` возвращает stderr агенту; `0` — тишина. testscan при находках (`--fail-on warning`) выходит с кодом `1`, поэтому скрипт превращает `1` в `2`.

### `.claude/settings.json`

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Write|Edit",
        "hooks": [
          { "type": "command", "command": "bash .claude/hooks/testscan.sh" }
        ]
      }
    ]
  }
}
```

### Bash (`.claude/hooks/testscan.sh`)

Нужны `jq` (если его нет — `python3`) и `testscan` в `PATH` (или замените на `uvx testscan@0.5.0`).

```bash
#!/usr/bin/env bash
input=$(cat)
if command -v jq >/dev/null 2>&1; then
  file=$(printf '%s' "$input" | jq -r '.tool_input.file_path // empty')
else
  file=$(printf '%s' "$input" | python3 -c 'import sys,json; print(json.load(sys.stdin).get("tool_input",{}).get("file_path",""))')
fi

# Только тестовые файлы: test_*.py / *_test.py
case "$(basename "$file")" in
  test_*.py|*_test.py) ;;
  *) exit 0 ;;
esac

# только stdout — глушим stderr, чтобы hidden-hint не попал к агенту
out=$(testscan "$file" --format agent --fail-on warning 2>/dev/null)
rc=$?
if [ "$rc" -eq 1 ]; then
  printf '%s\n' "$out" >&2
  exit 2          # есть находки: вернуть их агенту
fi
exit 0
```

### PowerShell (`.claude/hooks/testscan.ps1`)

Команда в настройках: `powershell -NoProfile -File .claude/hooks/testscan.ps1`.

```powershell
$payload = [Console]::In.ReadToEnd() | ConvertFrom-Json
$file = $payload.tool_input.file_path
if (-not $file) { exit 0 }

$name = Split-Path $file -Leaf
if ($name -notlike 'test_*.py' -and $name -notlike '*_test.py') { exit 0 }

# Только stdout; stderr глушим, чтобы hidden-hint не попал в payload агента
$out = & testscan $file --format agent --fail-on warning 2>$null | Out-String
if ($LASTEXITCODE -eq 1) {
  [Console]::Error.WriteLine($out.TrimEnd())
  exit 2          # есть находки: вернуть их агенту
}
exit 0
```

Примечания: `--fail-on error` блокирует только на ошибках; код выхода testscan `2` (ошибка аргументов/конфига) скрипты считают тишиной, чтобы сломанная настройка не блокировала агента.

Cursor также может подхватывать хуки Claude Code, если включено **Include Third-Party Plugins, Skills, and Other Configs** (Agents → Third-Party Imports). Для нативной подстановки находок в модель Cursor используйте рецепт ниже.

---

## Cursor

Нативные хуки живут в [`.cursor/hooks.json`](https://cursor.com/docs/hooks) (проект) или `~/.cursor/hooks.json` (пользователь). После агентского **Write** / **Edit**-подобного tool use хук `postToolUse` может вернуть `additional_context` — Cursor вставляет его в разговор; это канал, через который находки попадают к модели.

`afterFileEdit` удобен для форматтеров/аудита, но в документированной схеме **нет** поля обратной связи агенту; для правок запахов в том же ходе используйте `postToolUse`.

### `.cursor/hooks.json`

```json
{
  "version": 1,
  "hooks": {
    "postToolUse": [
      {
        "command": ".cursor/hooks/testscan.sh",
        "matcher": "Write|Edit|StrReplace"
      }
    ]
  }
}
```

### Bash (`.cursor/hooks/testscan.sh`)

```bash
#!/usr/bin/env bash
# Сделайте исполняемым: chmod +x .cursor/hooks/testscan.sh
input=$(cat)
if command -v jq >/dev/null 2>&1; then
  file=$(printf '%s' "$input" | jq -r '
    .tool_input.path // .tool_input.file_path // .file_path // empty
  ')
else
  file=$(printf '%s' "$input" | python3 -c '
import sys, json
d = json.load(sys.stdin)
ti = d.get("tool_input") or {}
if isinstance(ti, str):
    try: ti = json.loads(ti)
    except Exception: ti = {}
print(ti.get("path") or ti.get("file_path") or d.get("file_path") or "")
')
fi

case "$(basename "$file")" in
  test_*.py|*_test.py) ;;
  *) exit 0 ;;
esac

# stdout → additional_context; stderr (hidden-hint) остаётся в логах хука
out=$(testscan "$file" --format agent --fail-on warning)
rc=$?
if [ "$rc" -eq 1 ]; then
  ctx=$(printf '%s' "$out" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))')
  printf '{"additional_context": %s}\n' "$ctx"
fi
exit 0
```

Опциональное правило проекта (`.cursor/rules` / Agent rule): *После правок `test_*.py` / `*_test.py` запускай `testscan <file> --format agent` и исправляй находки до завершения.* Полезно, если хуки выключены или нужен явный шаг агента.

Ограничения: matcher — JS-подобные regex (расширьте, если агент пишет через другие tools); правки Tab идут через `afterTabFileEdit` (без `additional_context`). В `PATH` процесса хука должен быть `testscan` (или `uvx testscan@0.5.0`).

---

## Windsurf / Cascade

Хуки Cascade настраиваются в [`.windsurf/hooks.json`](https://docs.devin.ai/desktop/cascade/hooks) (workspace) или в user/system-файле. Предпочтительный event — **`post_write_code`**: срабатывает после записи файла и получает `tool_info.file_path` в stdin.

**Ограничение:** блокировать действие кодом `2` могут только *pre*-хуки. Post-хук не отменяет запись. Код `2` всё равно отдаёт stderr агенту Cascade; `"show_output": true` показывает находки и в UI Cascade.

### `.windsurf/hooks.json`

```json
{
  "hooks": {
    "post_write_code": [
      {
        "command": "bash .windsurf/hooks/testscan.sh",
        "show_output": true
      }
    ]
  }
}
```

На Windows можно добавить `"powershell": "powershell -NoProfile -File .windsurf/hooks/testscan.ps1"` рядом с `command`.

### Bash (`.windsurf/hooks/testscan.sh`)

```bash
#!/usr/bin/env bash
input=$(cat)
if command -v jq >/dev/null 2>&1; then
  file=$(printf '%s' "$input" | jq -r '.tool_info.file_path // empty')
else
  file=$(printf '%s' "$input" | python3 -c 'import sys,json; print(json.load(sys.stdin).get("tool_info",{}).get("file_path",""))')
fi

case "$(basename "$file")" in
  test_*.py|*_test.py) ;;
  *) exit 0 ;;
esac

# только stdout — глушим stderr, чтобы hidden-hint не попал к агенту
out=$(testscan "$file" --format agent --fail-on warning 2>/dev/null)
rc=$?
if [ "$rc" -eq 1 ]; then
  printf '%s\n' "$out" >&2
  exit 2          # stderr → Cascade (post-хук не блокирует запись)
fi
exit 0
```

Часть сборок Devin Desktop предпочитает `.devin/hooks.json` и откатывается к `.windsurf/hooks.json`, если первого нет — используйте путь из документации вашей IDE.

---

## CLI-агенты (Continue, Aider, …)

Инструменты, у которых в цикле агента есть только shell и нет отдельного post-edit hook API, вызывают testscan явно после генерации или правки тестов:

```bash
# Изменённые тесты с merge-base (PR / ветка агента)
testscan tests/ --diff origin/main --format agent --fail-on warning

# Или один файл, который агент только что написал
testscan tests/test_orders.py --format agent --fail-on warning
```

Оформите это как custom command в Continue / shell-шаг Aider или вставьте в инструкции агента («после написания тестов выполни команду выше и исправь находки»). Не выдумывайте IDE-специфичный hook JSON, если его нет в документации инструмента.
