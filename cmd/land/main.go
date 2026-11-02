// Command land is the step that makes the lake exist: it reads the NDJSON
// landing the gateway left on S3, already carrying ingestion_id and
// ingestion_loaded_at, and writes into the Iceberg table bronze.events
// through the SQL catalog in Postgres. The first run creates the table from
// the declaration; every batch of STOA_FLUSH_EVERY rows is one write and one
// snapshot.
//
// It is an sdk.Pipeline, and not a loose Extract and Load, because that is
// how the engine sees it: the phases, the counters and the `landed` line
// carrying iceberg://lake/bronze/events all come out of Run, and the
// console's /data is made of that line. It runs the same by hand:
//
//	go run ./cmd/land
//
// This is episode 1's `stoa-land`, already twenty lines instead of two
// hundred: the SDK learned Iceberg (brevis-sh/brevis#230) before stoa needed
// the glue.
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

	// iceberg-go's S3 backend is a blank import the consumer makes: whoever
	// writes to disk never compiles the AWS SDK. stoa writes to SeaweedFS.
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
		// No transformers: the gateway already stamped the identity and the
		// acceptance time on every row, and the declaration below requires both.
		Target: sdk.Target{
			To: iceberg.Table{
				Catalog:     cfg.Catalog(),
				Namespace:   "bronze",
				Name:        "events",
				PartitionBy: "occurred_at",
				CreateTable: &yes,
			},
			// Schema is already Columns with a type on each one; the SDK refuses both.
			Schema:     lake.Events,
			FlushEvery: every,
		},
	})
}
