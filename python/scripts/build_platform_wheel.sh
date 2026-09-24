#!/usr/bin/env bash
# Build a platform-tagged wheel with the Go testscan binary embedded.
# Usage: build_platform_wheel.sh <platform> <wheel_platform>
# Example: build_platform_wheel.sh linux-amd64 manylinux_2_17_x86_64
set -euo pipefail

platform="${1:-}"
wheel_platform="${2:-}"

if [[ -z "$platform" || -z "$wheel_platform" ]]; then
  echo "Usage: build_platform_wheel.sh <platform> <wheel_platform>" >&2
  echo "Example: build_platform_wheel.sh linux-amd64 manylinux_2_17_x86_64" >&2
  exit 1
fi

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
python_dir="$(cd "$script_dir/.." && pwd)"
root_dir="$(cd "$python_dir/.." && pwd)"
bin_dir="$python_dir/src/testscan_py/bin"
dist_dir="$root_dir/dist"

mkdir -p "$bin_dir" "$dist_dir"
rm -f "$bin_dir/testscan" "$bin_dir/testscan.exe"
rm -f "$dist_dir"/testscan-*.whl

if [[ "$platform" == *windows* ]]; then
  out="$bin_dir/testscan.exe"
else
  out="$bin_dir/testscan"
fi

echo "Building Go binary for $platform -> $out"
echo "  GOOS=${GOOS:-$(go env GOOS)} GOARCH=${GOARCH:-$(go env GOARCH)} CGO_ENABLED=${CGO_ENABLED:-0}"

(
  cd "$root_dir"
  CGO_ENABLED="${CGO_ENABLED:-0}" go build -trimpath -ldflags="-s -w" -o "$out" ./cmd/testscan
)

if [[ ! -f "$out" ]]; then
  echo "error: binary not found at $out" >&2
  exit 1
fi
chmod +x "$out" 2>/dev/null || true
echo "Binary size: $(du -h "$out" | cut -f1)"

echo "Building wheel (hatchling + hatch-vcs)..."
python -m pip install -q build hatchling hatch-vcs wheel
(
  cd "$python_dir"
  rm -rf dist build *.egg-info src/*.egg-info
  python -m build --wheel --outdir "$dist_dir"
)

shopt -s nullglob
# hatchling may emit py3-none-any (pure_python=false still defaults tag) or a host tag
built_wheels=("$dist_dir"/testscan-*.whl)
if [[ ${#built_wheels[@]} -eq 0 ]]; then
  echo "error: no wheel produced in $dist_dir" >&2
  ls -la "$dist_dir" >&2 || true
  exit 1
fi

echo "Retagging wheels to py3-none-$wheel_platform..."
for whl in "${built_wheels[@]}"; do
  python -m wheel tags \
    --python-tag py3 \
    --abi-tag none \
    --platform-tag "$wheel_platform" \
    --remove \
    "$whl"
done

echo "Wheels:"
ls -la "$dist_dir"/testscan-*.whl
