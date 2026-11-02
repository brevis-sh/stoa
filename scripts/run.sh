#!/usr/bin/env bash
# Dispara um run do workflow pelo mesmo caminho do botão do console, e espera
# o scheduler executá-lo. É o POST /workflows/{slug}/trigger, com o login do
# .env.local; `brevis backfill` não serve aqui porque materializa slots de
# agenda, e o workflow é manual até o gateway chegar.
#
#   scripts/run.sh [workflow] [último step]        # default: lake metrics
set -euo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
wf="${1:-lake}"; last="${2:-metrics}"
console="${STOA_CONSOLE:-http://localhost:8080}"
# shellcheck disable=SC1091
. "$here/.env.local"
jar="$(mktemp)"; trap 'rm -f "$jar"' EXIT

curl -fsS -o /dev/null -c "$jar" -d "username=${BREVIS_AUTH_USER}" --data-urlencode "password=${STOA_PASSWORD}" "$console/login"
location="$(curl -fsS -o /dev/null -w '%{redirect_url}' -b "$jar" -d when=now "$console/workflows/$wf/trigger")"
case "$location" in
  */runs/*) ;;
  *) echo "o disparo não devolveu um run: $location" >&2; exit 1 ;;
esac
id="${location##*/runs/}"
echo "run $id enfileirado: $console/runs/$id"

# O run acabou quando o último step disse `loaded`: o log do run traz a saída
# de cada step, e o SDK encerra cada pipeline com essa linha.
for i in $(seq 1 100); do
  log="$(curl -fsS -b "$jar" "$console/runs/$id/log.txt" 2>/dev/null || true)"
  if grep -q "msg=loaded pipeline=$last " <<<"$log"; then
    grep 'msg=loaded' <<<"$log" | sed -E 's/^.*pipeline=([a-z_]+).*records=([0-9]+).*table=([^ ]+).*table_created=([a-z]+).*duration=([^ ]+).*/  \1: \2 linhas em \3 (criada: \4) em \5/'
    echo "run terminou: $console/runs/$id"; exit 0
  fi
  if grep -q "✗ \|msg=\"load failed\"\|level=ERROR" <<<"$log"; then
    grep 'level=ERROR\|msg="load failed"\|✗ ' <<<"$log" | sed 's/^/  /' | tail -4
    echo "run falhou: $console/runs/$id" >&2; exit 1
  fi
  sleep 3
done
echo "o run não terminou em cinco minutos: $console/runs/$id" >&2; exit 1
