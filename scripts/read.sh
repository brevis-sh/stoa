#!/usr/bin/env bash
# A reader that is not iceberg-go reads a table of the lake: DuckDB, by the
# current metadata.json path the SQL catalog keeps in Postgres. No catalog on
# the reading side: that is how Spark, Trino and DuckDB find an Iceberg table
# when all they have is the path.
#
#   scripts/read.sh bronze events
#   scripts/read.sh gold daily_revenue
set -euo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ns="${1:-bronze}"; table="${2:-events}"
duckdb="$("$here/scripts/duckdb.sh")"

meta="$(docker compose -f "$here/docker-compose.yml" exec -T postgres \
  psql -U stoa -d lake -tAc "select metadata_location from iceberg_tables where table_namespace='$ns' and table_name='$table'")"
if [ -z "$meta" ]; then
  echo "the table $ns.$table is not in the catalog; run the step that writes it" >&2; exit 1
fi

endpoint="${STOA_S3_ENDPOINT:-http://localhost:8333}"
host="${endpoint#http://}"; host="${host#https://}"
ssl=false; case "$endpoint" in https://*) ssl=true;; esac

unset AI_AGENT CLAUDECODE
echo "== $ns.$table"
echo "   $meta"
"$duckdb" -c "
INSTALL iceberg; LOAD iceberg; INSTALL httpfs; LOAD httpfs; SET TimeZone='UTC';
CREATE SECRET lake (TYPE s3, KEY_ID '${STOA_S3_KEY:-stoa}', SECRET '${STOA_S3_SECRET:-stoa-secret}',
  REGION '${STOA_S3_REGION:-us-east-1}', ENDPOINT '$host', URL_STYLE 'path', USE_SSL $ssl);
SELECT (SELECT count(*) FROM iceberg_scan('$meta')) AS rows, (SELECT count(*) FROM iceberg_snapshots('$meta')) AS snapshots;
SELECT * FROM iceberg_scan('$meta') ORDER BY 1, 2 LIMIT 5;
" | grep -Ev "^│ (Success|true) +│$|^│ boolean +│$|^[┌├└]─{9}[┐┤┘]$"
