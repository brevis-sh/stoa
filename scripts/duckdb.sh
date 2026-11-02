#!/usr/bin/env bash
# Baixa o CLI do DuckDB na versão PINADA. Pinada porque o spike S2 mediu 1.4.5,
# 1.5.6 e 2.0.0 lendo a mesma tabela, e a 2.0.0 sem o bug documentado do
# iceberg_metadata.
#
#   scripts/duckdb.sh                       # o CLI desta máquina, em .bin/duckdb; imprime o caminho
#   scripts/duckdb.sh linux-arm64-musl bin  # o CLI do container do scheduler, em bin/duckdb
#
# O alvo `linux-<arch>-musl` é o que roda no Alpine do worker do brevis, com
# a libstdc++ que deploy/scheduler/Dockerfile acrescenta.
set -euo pipefail
unset AI_AGENT CLAUDECODE  # o DuckDB 2.0 muda a saída quando vê estas variáveis
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
      *) echo "sem CLI do DuckDB para $(uname -s)-$(uname -m)" >&2; exit 1 ;;
    esac
    if [ -x "$bin" ] && "$bin" --version 2>/dev/null | grep -q "v$version"; then echo "$bin"; exit 0; fi ;;
  linux-*-musl)
    asset="duckdb-cli-$target.tar.gz"
    # Um binário de outra plataforma não roda aqui para dizer a versão; o
    # carimbo ao lado dele diz.
    if [ -x "$bin" ] && [ "$(cat "$dest/duckdb.version" 2>/dev/null)" = "$version-$target" ]; then echo "$bin"; exit 0; fi ;;
  *) echo "alvo desconhecido: $target (host | linux-amd64-musl | linux-arm64-musl)" >&2; exit 1 ;;
esac
mkdir -p "$dest"
curl -fsSL "https://github.com/duckdb/duckdb/releases/download/v$version/$asset" | tar -xz -C "$dest"
echo "$version-$target" > "$dest/duckdb.version"
echo "$bin"
