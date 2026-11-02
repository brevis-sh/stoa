// Command metrics is episode 3's `stoa-metrics`: DuckDB computes, iceberg-go
// commits. It reads the bronze by the current metadata.json the catalog
// knows, aggregates revenue and volume per day and per host in DuckDB, and
// writes the result as a new version of gold.daily_revenue, stamped with
// `computed_at`. The table only appends, so the gold has history too, and the
// API reads the most recent version.
//
// It is an sdk.Pipeline whose source is DuckDB: a query becomes rows, and the
// SDK carries them to Iceberg as it carries any others. It runs the same by
// hand:
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

	_ "github.com/apache/iceberg-go/io/gocloud/s3" // iceberg-go's S3 backend, a blank import the consumer makes
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

// bronzeByDay is the source: the aggregation query in DuckDB over
// bronze.events, one row per day and host, stamped with this computation's
// time.
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
		return nil, fmt.Errorf("bronze.events: %w (run land first)", err)
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
