# Contributing

Thanks for helping improve testscan. This doc covers the local Go/Python loop, rule fixtures, and the precision corpus.

## Prerequisites

- Go (see `go.mod` for the toolchain version)
- Python 3 (or [`uv`](https://docs.astral.sh/uv/)) for the AST helper and optional wheel embed
- Optional: `jq` for agent-hook scripts in [docs/agent-hook.md](docs/agent-hook.md)

## Build and test

From the repo root:

```bash
go build -o testscan ./cmd/testscan
go test ./...
go vet ./...
```

Install the CLI into your Go bin:

```bash
go install ./cmd/testscan
```

## AST helper

AST-backed rules spawn `internal/parse/ast_dump.py` (via `uv` or `python3`). Smoke it manually:

```bash
uv run --no-project python internal/parse/ast_dump.py rules/testdata/empty/hit/test_empty.py
# or: python internal/parse/ast_dump.py path/to/test_foo.py
```

Details and JSON schema: [README.md](README.md#ast-helper-internalparse).

## Python package / embed wheels

PyPI ships platform wheels with a bundled Go binary. Contributors who want `uvx --from ./python`:

```bash
bash python/scripts/embed_bin.sh
# Windows: powershell -ExecutionPolicy Bypass -File python/scripts/embed_bin.ps1
uvx --from ./python testscan --help
```

Binaries under `python/src/testscan_py/bin/` are gitignored. Full notes: [python/README.md](python/README.md).

## Rule testdata

Each default/opt-in rule has fixtures under `rules/testdata/<rule_id>/`:

- `hit/` — should produce findings for that rule
- `clean/` — should stay quiet for that rule

Quick checks:

```bash
go test ./rules/...
testscan rules/testdata/empty/hit --rule empty-test --fail-on error
testscan rules/testdata/empty/clean --rule empty-test --fail-on error   # expect exit 0
```

Hit/clean prose for every rule: [docs/rules.md](docs/rules.md).

## Precision and labels

1. Scan a real suite with everything visible:

   ```bash
   testscan path/to/tests --all --format html --open
   ```

2. Mark findings **TP** / **FP** in the HTML UI → **Export labels** → save as `labels.json` (often under `.testscan/`).
3. Summarize:

   ```bash
   testscan precision --labels .testscan/labels.json
   ```

4. Measured values land in the catalog (`scan.RulePrecision` / `scan.Measured(...)` in `scan/`).

Shared corpus labels live in [`testdata/corpus/`](testdata/corpus/) (example: `mp-be.labels.json`). New labelled dumps from popular repos are welcome as PRs into that directory — see [#1 Label a corpus](https://github.com/Nikita527/testscan/issues/1) (`good first issue` / `help wanted`). Planned mutation work: [#2 testscan mutate](https://github.com/Nikita527/testscan/issues/2).

## Docs worth reading

- [docs/agent-hook.md](docs/agent-hook.md) — Claude Code / Cursor / Windsurf / CLI agents
- [docs/pre-commit.md](docs/pre-commit.md) — pre-commit (pin a release with `uvx testscan@x.y.z`)
- [docs/migration-0.5.md](docs/migration-0.5.md) — 0.4 → 0.5 CI migration

## Pull requests

- Prefer small, focused PRs (conventional commits: `feat`, `fix`, `chore`, `docs`, `refactor`, `test`).
- Keep `go test ./...` green.
- Do not commit secrets, local baselines under `.testscan/`, or embedded binaries under `python/src/testscan_py/bin/`.
