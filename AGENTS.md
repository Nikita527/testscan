# AGENTS.md — testscan

Guidance for coding agents (Cursor cloud/local, other AGENTS.md consumers). This file is the **repo SSOT** for agent conventions. Local Cursor may also load the user’s global Go rules when editing `*.go` / `go.mod`.

## Language

- Match the user’s language in conversation.
- Code, comments, commit messages, and user-facing CLI strings: **English**.

## Layout

- `cmd/testscan` — CLI entrypoint
- `internal/` — private packages (config, parse, …)
- `scan/` — scan orchestration
- `rules/` — **scanner rule implementations and fixtures** (not Cursor rules)
- `python/` — optional PyPI wheel / embed helpers
- `testdata/` — shared fixtures

Do not confuse `rules/` (testscan findings) with editor/agent rule files.

## Toolchain (required check-loop)

Go version: read the `go` directive in [`go.mod`](go.mod). Local loop also documented in [`CONTRIBUTING.md`](CONTRIBUTING.md).

After changing Go code, from the repo root, in order:

1. `gofmt -w` on touched files (or `gofmt -l .` clean)
2. `go vet ./...`
3. `golangci-lint run` — config: [`.golangci.yml`](.golangci.yml)
4. `go test ./...` (or the touched packages)

Useful builds:

```bash
go build -o testscan ./cmd/testscan
go install ./cmd/testscan
```

## Errors and context

- Wrap errors with `%w`; use `errors.Is` / `errors.As` for sentinels/types.
- `context.Context` is the first parameter for cancelable / I/O work; honor cancel and timeouts.

## Tests and concurrency

- Prefer table-driven tests.
- Do not start goroutines without cancel or wait (`WaitGroup` / `errgroup` / owned channel).
- Rule fixtures live under `rules/testdata/<rule_id>/{hit,clean}/` — see CONTRIBUTING.

## Modules

- Add deps with `go get`; after changes run `go mod tidy`.
- Do not vendor unless the repo already has `vendor/`.

## Python AST helper

AST-backed rules may spawn `internal/parse/ast_dump.py` via `uv` or `python3`. That helper is a small script — **not** a Django layered app. Do not apply mp-be domain/adapters/DRF patterns here.

```bash
uv run --no-project python internal/parse/ast_dump.py rules/testdata/empty/hit/test_empty.py
```

## Docs

- User/contributor flow: `README.md`, `CONTRIBUTING.md`
- Rule catalog: `docs/rules.md`
