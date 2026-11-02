# stoa

**Um lake 100% em Go.** Brevis, Apache Iceberg e DuckDB, do ingest à API, sem
JVM, sem Spark, sem um binário que não seja Go, e sem Kubernetes para começar:
`make up` e `make demo` numa máquina.

A *stoa* era o pórtico onde Zenão ensinava em Atenas, de graça, para quem
passasse. Este repositório é o pórtico de entrada do Brevis para times de
dados: tudo o que roda aqui roda com peças publicadas, e cada episódio da
série de posts e vídeos tira um pedaço de cola daqui porque ele virou
produto.

## O que já roda hoje

```sh
make up      # SeaweedFS (o S3, em Go) e Postgres (o catálogo Iceberg)
make seed    # 100.000 eventos NDJSON na landing do S3, em cinco objetos
make land    # a landing vira a tabela Iceberg bronze.events: cinco escritas, cinco snapshots
make read    # o DuckDB lê a tabela pelo metadata.json: contagem, soma, snapshots
```

O `land` é o step `stoa-land` do episódio 1. Ele usa o destino Iceberg do SDK
do Brevis, [`brevis.sh/brevis/sdk/iceberg`](https://github.com/brevis-sh/brevis/tree/master/sdk/iceberg):
lê a landing com `from.Files`, compõe `ingestion_id` e `ingestion_loaded_at`
com os transformadores do SDK, e escreve com `iceberg.Table`, que cria a tabela
a partir da declaração na primeira rodada, formato v2, Parquet zstd,
particionada por `day(occurred_at)`. Cada lote de `FlushEvery` é uma escrita e
um snapshot. O DuckDB lê o resultado sem catálogo, pelo caminho do
`metadata.json` que o catálogo SQL guarda no Postgres: é assim que Spark e
Trino também achariam a tabela.

## O que sobe

| peça | o que é | de onde vem |
|---|---|---|
| SeaweedFS | o S3 do lake, um binário em Go | imagem oficial, `chrislusf/seaweedfs:4.52` |
| Postgres | o catálogo Iceberg: duas tabelas e um compare-and-swap no ponteiro de metadados | imagem oficial, `postgres:17-alpine` |
| `seed` | eventos NDJSON na landing, como o gateway deixa | este repositório, `cmd/seed` |
| `land` | a landing para a tabela Iceberg | este repositório, `cmd/land` |
| DuckDB | o leitor que não é o escritor | CLI pinado em 2.0.0, baixado por `scripts/duckdb.sh` |

Configuração por ambiente, com os defaults do `docker-compose.yml`:
`STOA_S3_ENDPOINT`, `STOA_S3_KEY`, `STOA_S3_SECRET`, `STOA_S3_REGION`,
`STOA_BUCKET`, `STOA_PG_DSN`.

## A escada

| episódio | o que entra | o que deixa de ser cola |
|---|---|---|
| 1 | o lake sobe; `land` escreve, DuckDB lê | o próprio `land`: já são vinte linhas sobre `sdk/iceberg`, não duzentas sobre iceberg-go |
| 2 | o brevis-gateway recebe `POST /v1/events` e deixa a landing no S3 | o `seed` |
| 3 | `stoa-metrics`: DuckDB sobre `iceberg_scan`, ouro em Postgres via `sdk.Load`, brevis-sql modela, a API serve | |
| 4 | o gateway escreve direto no Iceberg com o sink `iceberg` | a landing NDJSON |
| 5 | o snapshot do Iceberg é a geração que invalida o cache da API | |

## Estado, com honestidade

O `go.mod` tem um `replace` **temporário** de `brevis.sh/brevis/sdk/iceberg`
para um checkout vizinho do brevis: o destino Iceberg está em
[brevis-sh/brevis#230](https://github.com/brevis-sh/brevis/pull/230), e o Go
só resolve esse caminho quando o site servir a página `go-import` dele, que é
gerada do master. Quando o PR pousar e a tag `sdk/iceberg/v0.1.0` existir, o
`replace` sai e o stoa compila sozinho em qualquer máquina.

## Licença

MIT, como o Brevis.
