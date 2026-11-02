# Where the stoa API runs: an Alpine with the libstdc++ the DuckDB CLI asks
# for and the system's certificates. The API binary and the CLI are mounted
# from bin/, built on the host, until the sdk/iceberg module has a tag and
# this repository gets a build of its own.
FROM alpine:3.24
RUN apk add --no-cache libstdc++ ca-certificates && adduser -D -u 65532 stoa
USER stoa
