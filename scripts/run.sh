#!/usr/bin/env bash
# Triggers a run of the workflow through the same path the console's button
# takes, and waits for the scheduler to execute it. It is the
# POST /workflows/{slug}/trigger, with the login from .env.local; `brevis
# backfill` does not serve here, because it materialises schedule slots and
# this workflow is manual.
#
#   scripts/run.sh [workflow] [last step]        # default: lake metrics
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
  *) echo "the trigger did not return a run: $location" >&2; exit 1 ;;
esac
id="${location##*/runs/}"
echo "run $id queued: $console/runs/$id"

# The run is over when the last step said `loaded`: the run's log carries each
# step's output, and the SDK closes every pipeline with that line.
for i in $(seq 1 100); do
  log="$(curl -fsS -b "$jar" "$console/runs/$id/log.txt" 2>/dev/null || true)"
  if grep -q "msg=loaded pipeline=$last " <<<"$log"; then
    grep 'msg=loaded' <<<"$log" | sed -E 's/^.*pipeline=([a-z_]+).*records=([0-9]+).*table=([^ ]+).*table_created=([a-z]+).*duration=([^ ]+).*/  \1: \2 rows into \3 (created: \4) in \5/'
    echo "run finished: $console/runs/$id"; exit 0
  fi
  if grep -q "✗ \|msg=\"load failed\"\|level=ERROR" <<<"$log"; then
    grep 'level=ERROR\|msg="load failed"\|✗ ' <<<"$log" | sed 's/^/  /' | tail -4
    echo "run failed: $console/runs/$id" >&2; exit 1
  fi
  sleep 3
done
echo "the run did not finish within five minutes: $console/runs/$id" >&2; exit 1
