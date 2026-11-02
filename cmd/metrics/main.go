// Command metrics é o `stoa-metrics` do episódio 3: o DuckDB computa, o
// iceberg-go comita. Lê o bronze pelo metadata.json corrente que o catálogo
// conhece, agrega receita e volume por dia e por host no DuckDB, e grava o
// resultado como uma versão nova de gold.daily_revenue, com `computed_at`.
// A tabela só anexa, então o ouro também tem história; a API lê a versão mais
// recente.
//
// É um sdk.Pipeline cuja fonte é o DuckDB: uma consulta vira linhas, e o SDK
// as leva ao Iceberg como leva qualquer outra. Roda igual na mão:
//
//	STOA_DUCKDB=.bin/duckdb go run ./cmd/metrics
package main

import (
	"context"
	"fmt"
	"iter"
	"log"
	"time"

	"brevis.sh/brevis/sdk"
	"brevis.sh/brevis/sdk/iceberg"
	"github.com/apache/iceberg-go/catalog"
	"github.com/brevis-sh/stoa/internal/duck"
	"github.com/brevis-sh/stoa/internal/lake"

	_ "github.com/apache/iceberg-go/io/gocloud/s3" // o backend S3 do iceberg-go, blank import do consumidor
)

func main() {
	cfg := lake.FromEnv()
	yes := true
	sdk.Run(sdk.Pipeline{
		Name:   "metrics",
		Source: sdk.Source{From: bronzeByDay{cfg: cfg, computedAt: time.Now().UTC().Truncate(time.Second)}},
		Target: sdk.Target{
			To: iceberg.Table{
				Catalog:     cfg.Catalog(),
				Namespace:   "gold",
				Name:        "daily_revenue",
				CreateTable: &yes,
			},
			Schema: lake.DailyRevenue,
		},
	})
}

// bronzeByDay é a fonte: a consulta de agregação no DuckDB sobre bronze.events,
// uma linha por dia e host, carimbada com a hora deste cálculo.
type bronzeByDay struct {
	cfg        lake.Config
	computedAt time.Time
}

func (b bronzeByDay) Describe() string { return "duckdb: bronze.events by day and host" }

func (b bronzeByDay) Read(ctx context.Context, _ sdk.ReadOptions) (iter.Seq2[sdk.Envelope, error], error) {
	cat, err := b.cfg.Catalog().Open(ctx)
	if err != nil {
		return nil, err
	}
	bronze, err := cat.LoadTable(ctx, catalog.ToIdentifier("bronze", "events"))
	if err != nil {
		return nil, fmt.Errorf("bronze.events: %w (rode o land antes)", err)
	}
	rows, err := duck.Query(ctx, b.cfg, lake.BronzeToDailyRevenue(bronze.MetadataLocation()))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		log.Printf("metrics: bronze.events is empty at %s", bronze.MetadataLocation())
	}
	at := b.computedAt.Format(time.RFC3339)
	return func(yield func(sdk.Envelope, error) bool) {
		for _, row := range rows {
			row["computed_at"] = at
			key := fmt.Sprintf("%v|%v", row["day"], row["host"])
			if !yield(sdk.Envelope{Provider: "stoa", Entity: "daily_revenue", SourceKey: key, RecordTS: at, Payload: row}, nil) {
				return
			}
		}
	}, nil
}
