# testscan (Python launcher)

English | [Русский](README.ru.md)

Thin wrapper: finds the Go binary and forwards `argv` / exit code.  
**Source of truth is the Go CLI** (`cmd/testscan`). Python has no rules, parsing, or flag handling.

Package name on PyPI: **`testscan`**. Version comes from git tags (`v*`) via hatch-vcs.

## Install (PyPI)

```bash
uvx testscan@latest tests/
# or: pipx run testscan tests/
# or: pip install testscan && testscan tests/
```

Release CI builds platform wheels (`linux/amd64`, `darwin/arm64`, `windows/amd64`) with the Go binary under `testscan_py/bin/`.

## Local embed (contributors)

Binaries under `src/testscan_py/bin/` are **not committed** (`.gitignore`). Embed before `uvx --from ./python`. `pyproject.toml` sets `ignore-vcs = true` and `force-include` for `bin/*`, otherwise hatchling can drop the binary because of gitignore.

```powershell
powershell -ExecutionPolicy Bypass -File python/scripts/embed_bin.ps1
```

```bash
bash python/scripts/embed_bin.sh
```

Manual equivalent:

```bash
go build -o python/src/testscan_py/bin/testscan ./cmd/testscan
# Windows: .../bin/testscan.exe
```

Then:

```bash
uvx --from ./python testscan --help
uvx --from ./python testscan rules/testdata/empty --fail-on never
```

Without embed — via env (handy for debugging the wrapper):

```bash
go build -o /tmp/testscan ./cmd/testscan
TESTSCAN_BIN=/tmp/testscan uvx --from ./python testscan --help
```

### Platform wheel (release / maintainers)

```bash
# from repo root; needs git tag history for hatch-vcs
bash python/scripts/build_platform_wheel.sh linux-amd64 manylinux_2_17_x86_64
```

Tag a release (`v0.1.0`) to trigger `.github/workflows/release.yml` → multi-OS wheels → PyPI + GitHub Release.

Before the first publish: create a PyPI project + [Trusted Publisher](https://docs.pypi.org/trusted-publishers/) for this repo (workflow `Release`, environment `pypi`), or the publish job will fail on OIDC.

## Binary lookup (priority)

1. `TESTSCAN_BIN`
2. bundled: `testscan_py/bin/testscan.exe` (Windows) / `testscan` (unix)
3. `PATH` (`shutil.which("testscan")`)

If not found → message on stderr, exit `2`.

## Manual check list

- [x] `uvx testscan@latest --help` (after PyPI publish) → usage (exit 0)
- [x] `uvx --from ./python testscan --help` after embed → usage (exit 0)
- [x] `uvx --from ./python testscan rules/testdata/empty/hit --rule empty-test --fail-on error` → exit 1
- [x] `uvx --from ./python testscan rules/testdata/empty/clean --fail-on error` → exit 0
- [x] bundled binary in wheel after embed + `ignore-vcs` / `force-include`
- [x] `go test ./...` green
