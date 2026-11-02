// Command land é o step que faz o lake existir: lê a landing NDJSON que o
// gateway deixou no S3, já com ingestion_id e ingestion_loaded_at, e escreve
// na tabela Iceberg bronze.events pelo catálogo SQL em Postgres. A primeira rodada cria a
// tabela a partir da declaração; cada lote de STOA_FLUSH_EVERY linhas é uma
// escrita e um snapshot.
//
// É um sdk.Pipeline, e não um Extract e um Load soltos, porque é assim que o
// engine o vê: as fases, os contadores e a linha `landed` com
// iceberg://lake/bronze/events saem do Run, e o /data do console é feito
// dessa linha. Roda igual na mão:
//
//	go run ./cmd/land
//
// Este é o `stoa-land` do episódio 1, já com vinte linhas em vez de duzentas:
// o SDK aprendeu Iceberg (brevis-sh/brevis#230) antes do stoa precisar de cola.
package main

import (
	"context"
	"log"
	"os"
	"strconv"

	"brevis.sh/brevis/sdk"
	"brevis.sh/brevis/sdk/from"
	"brevis.sh/brevis/sdk/iceberg"
	"github.com/brevis-sh/stoa/internal/lake"

	// O backend S3 do iceberg-go é um blank import do consumidor: quem escreve
	// em disco não compila a AWS. O stoa escreve no SeaweedFS, então importa.
	_ "github.com/apache/iceberg-go/io/gocloud/s3"
)

func main() {
	cfg := lake.FromEnv()
	store, err := cfg.Store(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	every := 20_000
	if v := os.Getenv("STOA_FLUSH_EVERY"); v != "" {
		if every, err = strconv.Atoi(v); err != nil {
			log.Fatalf("STOA_FLUSH_EVERY: %v", err)
		}
	}
	yes := true

	sdk.Run(sdk.Pipeline{
		Name: "land",
		Source: sdk.Source{
			From: from.Files{Path: cfg.Landing() + "*.ndjson", Format: sdk.FormatNDJSON, Store: store},
		},
		// Nenhum transformador: o gateway já carimbou a identidade e a hora de
		// aceitação em cada linha, e a declaração abaixo exige as duas.
		Target: sdk.Target{
			To: iceberg.Table{
				Catalog:     cfg.Catalog(),
				Namespace:   "bronze",
				Name:        "events",
				PartitionBy: "occurred_at",
				CreateTable: &yes,
			},
			// Schema já é Columns com tipo em cada coluna; o SDK recusa os dois.
			Schema:     lake.Events,
			FlushEvery: every,
		},
	})
}
