#!/usr/bin/env bash
# Baixa o CLI do DuckDB na versão PINADA, para o sistema e a arquitetura desta
# máquina, em .bin/duckdb. Pinada porque o spike S2 mediu 1.4.5, 1.5.6 e 2.0.0
# lendo a mesma tabela, e a 2.0.0 sem o bug documentado do iceberg_metadata.
set -euo pipefail
unset AI_AGENT CLAUDECODE  # o DuckDB 2.0 muda a saída quando vê estas variáveis
version="${DUCKDB_VERSION:-2.0.0}"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
bin="$here/.bin/duckdb"
if [ -x "$bin" ] && "$bin" --version 2>/dev/null | grep -q "v$version"; then
  echo "$bin"; exit 0
fi
case "$(uname -s)-$(uname -m)" in
  Darwin-*)        asset="duckdb-cli-osx-universal.tar.gz" ;;
  Linux-x86_64)    asset="duckdb-cli-linux-amd64.tar.gz" ;;
  Linux-aarch64)   asset="duckdb-cli-linux-arm64.tar.gz" ;;
  *) echo "sem CLI do DuckDB para $(uname -s)-$(uname -m)" >&2; exit 1 ;;
esac
mkdir -p "$here/.bin"
curl -fsSL "https://github.com/duckdb/duckdb/releases/download/v$version/$asset" | tar -xz -C "$here/.bin"
"$bin" --version >&2
echo "$bin"
