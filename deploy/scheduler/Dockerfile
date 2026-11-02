# Brevis's published worker, plus one library: the musl DuckDB CLI the
# `metrics` step calls is dynamically linked against libstdc++, and the
# worker's Alpine image does not carry it. Nothing else changes: same user,
# same entrypoint, same scheduler. It exists so DuckDB can compute inside the
# engine with no cgo.
FROM brevissh/brevis:0.41.0-worker
USER root
RUN apk add --no-cache libstdc++
USER brevis
