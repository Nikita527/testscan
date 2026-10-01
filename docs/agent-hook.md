# Agent hook (Claude Code)

English | [Русский](agent-hook.ru.md)

Run testscan on every test file an AI agent writes or edits, and feed the findings straight back to the agent so it fixes them in the same turn.

## Output: `--format agent`

One line per finding, no headers: `file:line rule — problem → fix`, then one summary line.

```text
tests/test_orders.py:42 sleep-in-test — time.sleep / asyncio.sleep in test; prefer event waits or freezegun / time-machine → remove `time.sleep(...)`; wait on the condition (poll with a timeout or an event) or use a fake clock
tests/test_orders.py:88 broad-raises — pytest.raises(Exception) is too broad; specify a concrete exception class → replace `pytest.raises(Exception)` with the specific exception, e.g. `pytest.raises(ValueError, match=...)`, and keep only the failing call in the block
testscan: 2 findings (2 warning) · 31 tests
```

The default display filter applies (actionable/provisional rules only); add `--all` to see everything. The same `fix` text is also in `--format json` (`findings[].fix`), SARIF (`rules[].help`, `results[].properties.fix`), HTML and text output.

## How the hook works

Claude Code runs a `PostToolUse` command hook after `Write` / `Edit`, passing JSON on stdin (the edited path is `tool_input.file_path`). Exit code `2` feeds stderr back to Claude; exit code `0` is silent. testscan exits `1` when it finds something (`--fail-on warning`), so the script maps `1` to `2`.

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

Needs `jq` (falls back to `python3` if `jq` is missing) and `testscan` on `PATH` (or replace it with `uvx testscan@latest`).

```bash
#!/usr/bin/env bash
input=$(cat)
if command -v jq >/dev/null 2>&1; then
  file=$(printf '%s' "$input" | jq -r '.tool_input.file_path // empty')
else
  file=$(printf '%s' "$input" | python3 -c 'import sys,json; print(json.load(sys.stdin).get("tool_input",{}).get("file_path",""))')
fi

# Only test files: test_*.py / *_test.py
case "$(basename "$file")" in
  test_*.py|*_test.py) ;;
  *) exit 0 ;;
esac

out=$(testscan "$file" --format agent --fail-on warning 2>&1)
rc=$?
if [ "$rc" -eq 1 ]; then
  printf '%s\n' "$out" >&2
  exit 2          # findings: send them back to the agent
fi
exit 0
```

## PowerShell (`.claude/hooks/testscan.ps1`)

Settings command: `powershell -NoProfile -File .claude/hooks/testscan.ps1`.

```powershell
$payload = [Console]::In.ReadToEnd() | ConvertFrom-Json
$file = $payload.tool_input.file_path
if (-not $file) { exit 0 }

$name = Split-Path $file -Leaf
if ($name -notlike 'test_*.py' -and $name -notlike '*_test.py') { exit 0 }

$out = & testscan $file --format agent --fail-on warning 2>&1 | Out-String
if ($LASTEXITCODE -eq 1) {
  [Console]::Error.WriteLine($out.TrimEnd())
  exit 2          # findings: send them back to the agent
}
exit 0
```

Notes: use `--fail-on error` to only block on errors; testscan exit code `2` (usage or config error) is treated as silent by these scripts so a broken setup never blocks the agent.
