#!/usr/bin/env bash
# Um leitor que não é o iceberg-go lê a tabela: o DuckDB, pelo caminho do
# metadata.json corrente, que o catálogo SQL guarda em Postgres. Sem catálogo
# do lado da leitura: é assim que Spark, Trino e o DuckDB acham uma tabela
# Iceberg quando só têm o caminho.
set -euo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ns="${1:-bronze}"; table="${2:-events}"
duckdb="$("$here/scripts/duckdb.sh")"

meta="$(docker compose -f "$here/docker-compose.yml" exec -T postgres \
  psql -U stoa -d lake -tAc "select metadata_location from iceberg_tables where table_namespace='$ns' and table_name='$table'")"
if [ -z "$meta" ]; then
  echo "a tabela $ns.$table não está no catálogo; rode make land primeiro" >&2; exit 1
fi

endpoint="${STOA_S3_ENDPOINT:-http://localhost:8333}"
host="${endpoint#http://}"; host="${host#https://}"
ssl=false; case "$endpoint" in https://*) ssl=true;; esac

unset AI_AGENT CLAUDECODE
echo "== $ns.$table"
echo "   $meta"
"$duckdb" -c "
INSTALL iceberg; LOAD iceberg; INSTALL httpfs; LOAD httpfs;
CREATE SECRET lake (TYPE s3, KEY_ID '${STOA_S3_KEY:-stoa}', SECRET '${STOA_S3_SECRET:-stoa-secret}',
  REGION '${STOA_S3_REGION:-us-east-1}', ENDPOINT '$host', URL_STYLE 'path', USE_SSL $ssl);
SELECT count(*) AS rows, sum(quantity) AS sum_quantity, min(occurred_at) AS first, max(occurred_at) AS last
  FROM iceberg_scan('$meta');
SELECT count(*) AS snapshots FROM iceberg_snapshots('$meta');
"
