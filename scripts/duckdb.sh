#!/usr/bin/env bash
# Fetches the DuckDB CLI at the PINNED version. Pinned because spike S2
# measured 1.4.5, 1.5.6 and 2.0.0 reading the same table, and 2.0.0 without the
# documented iceberg_metadata bug.
#
#   scripts/duckdb.sh                       # this machine's CLI, in .bin/duckdb; prints the path
#   scripts/duckdb.sh linux-arm64-musl bin  # the scheduler container's CLI, in bin/duckdb
#
# The `linux-<arch>-musl` target is the one that runs on Brevis's Alpine
# worker, with the libstdc++ deploy/scheduler/Dockerfile adds.
set -euo pipefail
unset AI_AGENT CLAUDECODE  # DuckDB 2.0 changes its output when it sees these
version="${DUCKDB_VERSION:-2.0.0}"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
target="${1:-host}"; dest="$here/${2:-.bin}"
bin="$dest/duckdb"
case "$target" in
  host)
    case "$(uname -s)-$(uname -m)" in
      Darwin-*)      asset="duckdb-cli-osx-universal.tar.gz" ;;
      Linux-x86_64)  asset="duckdb-cli-linux-amd64.tar.gz" ;;
      Linux-aarch64) asset="duckdb-cli-linux-arm64.tar.gz" ;;
      *) echo "no DuckDB CLI for $(uname -s)-$(uname -m)" >&2; exit 1 ;;
    esac
    if [ -x "$bin" ] && "$bin" --version 2>/dev/null | grep -q "v$version"; then echo "$bin"; exit 0; fi ;;
  linux-*-musl)
    asset="duckdb-cli-$target.tar.gz"
    # A binary for another platform cannot run here to report its version; the
    # stamp beside it says.
    if [ -x "$bin" ] && [ "$(cat "$dest/duckdb.version" 2>/dev/null)" = "$version-$target" ]; then echo "$bin"; exit 0; fi ;;
  *) echo "unknown target: $target (host | linux-amd64-musl | linux-arm64-musl)" >&2; exit 1 ;;
esac
mkdir -p "$dest"
curl -fsSL "https://github.com/duckdb/duckdb/releases/download/v$version/$asset" | tar -xz -C "$dest"
echo "$version-$target" > "$dest/duckdb.version"
echo "$bin"
