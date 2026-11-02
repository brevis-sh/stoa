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
make up      # SeaweedFS (o S3), Postgres (catálogo e engine), o brevis com o console, o brevis-gateway; workflow e gateway publicados no console
make seed    # 100.000 eventos em cinco POSTs ao gateway, que responde 202 e deixa a landing NDJSON no S3
make run     # dispara um run do workflow `lake`; o scheduler do brevis executa o step `land`
make read    # o DuckDB lê a tabela pelo metadata.json: contagem, soma, snapshots
```

O console fica em `http://localhost:8080`, usuário `stoa`, senha impressa pelo
`make up` e guardada em `.env.local`, que nunca é comitado. O run aparece lá
com o step `land`, suas fases e seus números; `make logs` mostra o scheduler
executando. `make land` roda o mesmo step na mão, fora do engine.

O gateway é o `brevissh/brevis-gateway` publicado, com uma rota,
`POST /v1/events`, e um stream `events` configurado em
`deploy/gateway/gateway.yaml`: formato NDJSON, identidade sobre `event_id` e
`occurred_at`, `ingestion_id` e `ingestion_loaded_at` carimbados ao aceitar, e
um objeto na landing a cada 20.000 eventos ou dois segundos. O console o
mostra em `/data` e `/ingest/streams` porque o `make up` publica a descrição
dele, como faz com o workflow. Localmente a porta fica aberta, sem token, e o
gateway diz isso no log; fora de `BREVIS_ENV=local` ele se recusa a subir sem
`listen.auth`.

O `land` é o step `stoa-land` do episódio 1. Ele usa o destino Iceberg do SDK
do Brevis, [`brevis.sh/brevis/sdk/iceberg`](https://github.com/brevis-sh/brevis/tree/master/sdk/iceberg):
lê a landing com `from.Files`, já com `ingestion_id` e `ingestion_loaded_at`
que o gateway escreveu, e escreve com `iceberg.Table`, que cria a tabela
a partir da declaração na primeira rodada, formato v2, Parquet zstd,
particionada por `day(occurred_at)`. Cada lote de `FlushEvery` é uma escrita e
um snapshot. O DuckDB lê o resultado sem catálogo, pelo caminho do
`metadata.json` que o catálogo SQL guarda no Postgres: é assim que Spark e
Trino também achariam a tabela.

## O que sobe

| peça | o que é | de onde vem |
|---|---|---|
| SeaweedFS | o S3 do lake, um binário em Go | imagem oficial, `chrislusf/seaweedfs:4.52` |
| Postgres | dois bancos: `lake`, o catálogo Iceberg, e `brevis`, o engine | imagem oficial, `postgres:17-alpine` |
| brevis | o engine: fila, scheduler, console; os steps rodam como processos no container do scheduler | imagens publicadas, `brevissh/brevis:0.41.0` e `:0.41.0-worker` |
| brevis-gateway | `POST /v1/events`, 202, a landing NDJSON no S3; descrito e publicado no console pelo `make up` | imagem publicada, `brevissh/brevis-gateway:0.33.0`, config em `deploy/gateway/gateway.yaml` |
| `workflows/lake.yaml` | o workflow com o step `land`, publicado no projeto `stoa` | este repositório |
| `seed` | um produtor: eventos inventados, enviados ao gateway em lotes NDJSON | este repositório, `cmd/seed` |
| `land` | a landing para a tabela Iceberg, um `sdk.Pipeline` | este repositório, `cmd/land`, compilado em `bin/` pelo `make build` |
| DuckDB | o leitor que não é o escritor | CLI pinado em 2.0.0, baixado por `scripts/duckdb.sh` |

Configuração por ambiente, com os defaults do `docker-compose.yml`:
`STOA_S3_ENDPOINT`, `STOA_S3_KEY`, `STOA_S3_SECRET`, `STOA_S3_REGION`,
`STOA_BUCKET`, `STOA_PG_DSN`.

## A escada

| episódio | o que entra | o que deixa de ser cola |
|---|---|---|
| 1 | o lake sobe; `land` escreve, DuckDB lê | o próprio `land`: já são vinte linhas sobre `sdk/iceberg`, não duzentas sobre iceberg-go |
| 2 | o brevis-gateway recebe `POST /v1/events` e deixa a landing no S3 **(feito)** | o `seed` que escrevia no S3; agora ele é só um produtor |
| 3 | `stoa-metrics`: DuckDB sobre `iceberg_scan`, ouro em Postgres via `sdk.Load`, brevis-sql modela, a API serve | |
| 4 | o gateway escreve direto no Iceberg com o sink `iceberg` | a landing NDJSON |
| 5 | o snapshot do Iceberg é a geração que invalida o cache da API | |

## Estado, com honestidade

O workflow não tem `schedule` ainda: o `land` relê a landing inteira a cada
rodada, e duas rodadas sobre a mesma landing dobrariam a tabela. Um run por
seed, até o episódio 2 particionar a landing por hora e o step ler só o que
ainda não pousou.

O `/data` do console ainda não mostra `iceberg://lake/bronze/events`: o engine
publicado (0.41.0) não conhece o esquema `iceberg://`, que entra em
[brevis-sh/brevis#229](https://github.com/brevis-sh/brevis/pull/229). O step
declara a landing e o scheduler registra "a step declared landings the engine
did not keep, refused=1" no log. Quando esse PR sair numa versão do engine,
basta subir a tag no `docker-compose.yml`.

Cada run do `land` escreve 140 arquivos Parquet, e não cinco: a tabela é
particionada por `day(occurred_at)` e os eventos cobrem 28 dias, então cada
um dos cinco lotes vira um arquivo por dia. É o layout que faz "os eventos de
ontem" ser uma leitura de um diretório, e é o que um leitor por partição
espera encontrar.

O `go.mod` tem um `replace` **temporário** de `brevis.sh/brevis/sdk/iceberg`
para um checkout vizinho do brevis: o destino Iceberg está em
[brevis-sh/brevis#230](https://github.com/brevis-sh/brevis/pull/230), e o Go
só resolve esse caminho quando o site servir a página `go-import` dele, que é
gerada do master. Quando o PR pousar e a tag `sdk/iceberg/v0.1.0` existir, o
`replace` sai e o stoa compila sozinho em qualquer máquina.

## Licença

MIT, como o Brevis.
