# Onde a API do stoa roda: um Alpine com a libstdc++ que o CLI do DuckDB pede
# e os certificados do sistema. O binário da API e o CLI vêm montados de bin/,
# compilados no host, até o módulo sdk/iceberg ter tag e este repo ganhar um
# build próprio.
FROM alpine:3.24
RUN apk add --no-cache libstdc++ ca-certificates && adduser -D -u 65532 stoa
USER stoa
