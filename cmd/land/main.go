// Command land é o step que faz o lake existir: lê a landing NDJSON que ficou
// no S3, compõe as duas colunas de ingestão, e escreve na tabela Iceberg
// bronze.events pelo catálogo SQL em Postgres. A primeira rodada cria a
// tabela a partir da declaração; cada lote de FlushEvery é uma escrita e um
// snapshot.
//
//	go run ./cmd/land                      # a landing inteira, lotes de 20.000
//
// Este é o `stoa-land` do episódio 1, já com vinte linhas em vez de duzentas:
// o SDK aprendeu Iceberg (brevis-sh/brevis#230) antes do stoa precisar de cola.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"brevis.sh/brevis/sdk"
	"brevis.sh/brevis/sdk/from"
	"brevis.sh/brevis/sdk/iceberg"
	"github.com/brevis-sh/stoa/internal/lake"

	// O backend S3 do iceberg-go é um blank import do consumidor: quem escreve
	// em disco não compila a AWS. O stoa escreve no SeaweedFS, então importa.
	_ "github.com/apache/iceberg-go/io/gocloud/s3"
)

func main() {
	every := flag.Int("every", 20_000, "linhas por escrita, e portanto por snapshot")
	flag.Parse()
	ctx := context.Background()
	cfg := lake.FromEnv()

	store, err := cfg.Store(ctx)
	if err != nil {
		log.Fatal(err)
	}
	data, err := sdk.Extract(ctx, sdk.Source{
		From: from.Files{Path: cfg.Landing() + "*.ndjson", Format: sdk.FormatNDJSON, Store: store},
	})
	if err != nil {
		log.Fatalf("extract: %v", err)
	}
	data = sdk.Transform(data, sdk.IngestionID(), sdk.IngestionLoadedAt())

	yes := true
	res, err := sdk.Load(ctx, data, sdk.Target{
		To: iceberg.Table{
			Catalog:     cfg.Catalog(),
			Namespace:   "bronze",
			Name:        "events",
			PartitionBy: "occurred_at",
			CreateTable: &yes,
		},
		// Schema já é Columns com tipo em cada coluna; o SDK recusa os dois.
		Schema:     lake.Events,
		FlushEvery: *every,
	})
	if err != nil {
		log.Fatalf("load: %v", err)
	}
	fmt.Printf("land: %d linhas em %s (%s), tabela criada: %v, %d bytes Parquet, %s\n",
		res.Rows, res.Target, res.Strategy, res.TableCreated, res.Bytes, res.LoadTime.Round(time.Millisecond))
}
