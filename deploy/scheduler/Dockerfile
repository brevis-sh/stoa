# O worker publicado do brevis, mais uma biblioteca: o CLI musl do DuckDB,
# que o step `metrics` chama, é ligado dinamicamente à libstdc++, e a imagem
# Alpine do worker não a traz. Nada mais muda: mesmo usuário, mesmo entrypoint,
# mesmo scheduler. É o único Dockerfile deste repositório que não é do
# produto, e ele existe para o DuckDB computar dentro do engine sem cgo.
FROM brevissh/brevis:0.41.0-worker
USER root
RUN apk add --no-cache libstdc++
USER brevis
