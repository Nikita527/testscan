# Хук для агента (Claude Code)

[English](agent-hook.md) | Русский

Запускайте testscan на каждом тестовом файле, который ИИ-агент создал или изменил, и возвращайте находки агенту, чтобы он исправил их в том же ходе.

## Вывод: `--format agent`

Одна строка на находку, без заголовков: `file:line rule — проблема → fix`, затем одна итоговая строка.

```text
tests/test_orders.py:42 sleep-in-test — time.sleep / asyncio.sleep in test; prefer event waits or freezegun / time-machine → remove `time.sleep(...)`; wait on the condition (poll with a timeout or an event) or use a fake clock
tests/test_orders.py:88 broad-raises — pytest.raises(Exception) is too broad; specify a concrete exception class → replace `pytest.raises(Exception)` with the specific exception, e.g. `pytest.raises(ValueError, match=...)`, and keep only the failing call in the block
testscan: 2 findings (2 warning) · 31 tests
```

Действует фильтр показа по умолчанию (только actionable/provisional); `--all` показывает всё. Тот же текст `fix` есть в `--format json` (`findings[].fix`), SARIF (`rules[].help`, `results[].properties.fix`), HTML и text.

## Как работает хук

После `Write` / `Edit` Claude Code запускает команду `PostToolUse` и передаёт JSON в stdin (путь — `tool_input.file_path`). Код выхода `2` возвращает stderr агенту; `0` — тишина. testscan при находках (`--fail-on warning`) выходит с кодом `1`, поэтому скрипт превращает `1` в `2`.

## `.claude/settings.json`

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

## Bash (`.claude/hooks/testscan.sh`)

Нужны `jq` (если его нет — `python3`) и `testscan` в `PATH` (или замените на `uvx testscan@latest`).

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

out=$(testscan "$file" --format agent --fail-on warning 2>&1)
rc=$?
if [ "$rc" -eq 1 ]; then
  printf '%s\n' "$out" >&2
  exit 2          # есть находки: вернуть их агенту
fi
exit 0
```

## PowerShell (`.claude/hooks/testscan.ps1`)

Команда в настройках: `powershell -NoProfile -File .claude/hooks/testscan.ps1`.

```powershell
$payload = [Console]::In.ReadToEnd() | ConvertFrom-Json
$file = $payload.tool_input.file_path
if (-not $file) { exit 0 }

$name = Split-Path $file -Leaf
if ($name -notlike 'test_*.py' -and $name -notlike '*_test.py') { exit 0 }

$out = & testscan $file --format agent --fail-on warning 2>&1 | Out-String
if ($LASTEXITCODE -eq 1) {
  [Console]::Error.WriteLine($out.TrimEnd())
  exit 2          # есть находки: вернуть их агенту
}
exit 0
```

Примечания: `--fail-on error` блокирует только на ошибках; код выхода testscan `2` (ошибка аргументов/конфига) скрипты считают тишиной, чтобы сломанная настройка не блокировала агента.
