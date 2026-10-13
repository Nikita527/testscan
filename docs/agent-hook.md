# Agent hooks

English | [Русский](agent-hook.ru.md)

Run testscan on every test file an AI agent writes or edits, and feed the findings straight back to the agent so it fixes them in the same turn.

## Output: `--format agent`

One line per finding, no headers: `file:line rule — problem → fix`, then one summary line.

```text
tests/test_orders.py:42 sleep-in-test — time.sleep / asyncio.sleep in test; prefer event waits or freezegun / time-machine → remove `time.sleep(...)`; wait on the condition (poll with a timeout or an event) or use a fake clock
tests/test_orders.py:88 broad-raises — pytest.raises(Exception) is too broad; specify a concrete exception class → replace `pytest.raises(Exception)` with the specific exception, e.g. `pytest.raises(ValueError, match=...)`, and keep only the failing call in the block
testscan: 2 findings (2 warning) · 31 tests
```

The default display filter applies (actionable / provisional / deterministic rules); add `--all` to see everything. The same `fix` text is also in `--format json` (`findings[].fix`), SARIF (`rules[].help`, `results[].properties.fix`), HTML and text output.

**Stderr vs agent payload:** when anything is hidden, testscan prints a hint on stderr. Do **not** merge stderr into the findings you feed the model (`2>&1`). Capture **stdout** only (or discard stderr with `2>/dev/null` when the hook process stderr is later shown to the agent). The hint can stay in hook logs.

---

## Claude Code

### How the hook works

Claude Code runs a `PostToolUse` command hook after `Write` / `Edit`, passing JSON on stdin (the edited path is `tool_input.file_path`). Exit code `2` feeds stderr back to Claude; exit code `0` is silent. testscan exits `1` when it finds something (`--fail-on warning`), so the script maps `1` to `2`.

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

Needs `jq` (falls back to `python3` if `jq` is missing) and `testscan` on `PATH` (or replace it with `uvx testscan@0.5.0`).

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

# stdout only — discard stderr so the hidden-hint never reaches the agent
out=$(testscan "$file" --format agent --fail-on warning 2>/dev/null)
rc=$?
if [ "$rc" -eq 1 ]; then
  printf '%s\n' "$out" >&2
  exit 2          # findings: send them back to the agent
fi
exit 0
```

### PowerShell (`.claude/hooks/testscan.ps1`)

Settings command: `powershell -NoProfile -File .claude/hooks/testscan.ps1`.

```powershell
$payload = [Console]::In.ReadToEnd() | ConvertFrom-Json
$file = $payload.tool_input.file_path
if (-not $file) { exit 0 }

$name = Split-Path $file -Leaf
if ($name -notlike 'test_*.py' -and $name -notlike '*_test.py') { exit 0 }

# Capture stdout only; suppress stderr so the hidden-hint stays out of the agent payload
$out = & testscan $file --format agent --fail-on warning 2>$null | Out-String
if ($LASTEXITCODE -eq 1) {
  [Console]::Error.WriteLine($out.TrimEnd())
  exit 2          # findings: send them back to the agent
}
exit 0
```

Notes: use `--fail-on error` to only block on errors; testscan exit code `2` (usage or config error) is treated as silent by these scripts so a broken setup never blocks the agent.

Cursor can also load Claude Code hooks when **Include Third-Party Plugins, Skills, and Other Configs** is enabled (Agents → Third-Party Imports). For native Cursor feedback into the model, prefer the recipe below.

---

## Cursor

Native hooks live in [`.cursor/hooks.json`](https://cursor.com/docs/hooks) (project) or `~/.cursor/hooks.json` (user). After an agent **Write** / **Edit**-style tool use, `postToolUse` can return `additional_context`, which Cursor injects into the conversation — that is the channel that feeds findings back to the model.

`afterFileEdit` is useful for formatters/audit, but the documented schema has **no** agent-feedback field; use `postToolUse` when you want the agent to fix smells in the same turn.

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
# Make executable: chmod +x .cursor/hooks/testscan.sh
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

# stdout → additional_context; stderr (hidden hint) stays in hook logs
out=$(testscan "$file" --format agent --fail-on warning)
rc=$?
if [ "$rc" -eq 1 ]; then
  # Escape for JSON string
  ctx=$(printf '%s' "$out" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))')
  printf '{"additional_context": %s}\n' "$ctx"
fi
exit 0
```

Optional project rule (`.cursor/rules` / Agent rule): *After editing `test_*.py` / `*_test.py`, run `testscan <file> --format agent` and fix any findings before finishing.* Useful if hooks are disabled or you prefer an explicit agent step.

Limitations: matcher values are JS-style regexes (extend if your agent uses other write tools); Tab edits use `afterTabFileEdit` (no `additional_context`). Keep `testscan` (or `uvx testscan@0.5.0`) on `PATH` for the hook process.

---

## Windsurf / Cascade

Cascade hooks are configured in [`.windsurf/hooks.json`](https://docs.devin.ai/desktop/cascade/hooks) (workspace) or the user/system hooks file. Prefer **`post_write_code`**: it runs after Cascade writes a file and receives `tool_info.file_path` on stdin.

**Limitation:** only *pre*-hooks can block an action with exit code `2`. Post-hooks cannot undo the write. Exit `2` still surfaces stderr to the Cascade agent; set `"show_output": true` so you also see findings in the Cascade UI.

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

On Windows you can add `"powershell": "powershell -NoProfile -File .windsurf/hooks/testscan.ps1"` alongside `command`.

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

# stdout only — discard stderr so the hidden-hint never reaches the agent
out=$(testscan "$file" --format agent --fail-on warning 2>/dev/null)
rc=$?
if [ "$rc" -eq 1 ]; then
  printf '%s\n' "$out" >&2
  exit 2          # feed stderr to Cascade (post-hook cannot block the write)
fi
exit 0
```

Some Devin Desktop builds prefer `.devin/hooks.json` and fall back to `.windsurf/hooks.json` when the former is absent — use whichever your IDE documents for the workspace.

---

## CLI agents (Continue, Aider, …)

Tools that only expose a shell in the agent loop — no dedicated post-edit hook API — should call testscan explicitly after generating or editing tests:

```bash
# Changed tests since merge base (PR / agent branch)
testscan tests/ --diff origin/main --format agent --fail-on warning

# Or a single file the agent just wrote
testscan tests/test_orders.py --format agent --fail-on warning
```

Wire that as a Continue custom command / Aider shell step, or paste it into the agent’s instructions (“after writing tests, run the command above and fix findings”). Do not invent IDE-specific hook JSON for these tools unless their docs document one.
