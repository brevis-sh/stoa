// Command seed deixa eventos NDJSON na landing do lake, no S3, como o
// brevis-gateway deixa quando recebe um POST: um objeto por lote. É a matéria
// prima do `land`, e some do stoa no episódio em que o gateway entra em cena.
//
//	go run ./cmd/seed -n 100000 -every 20000     # cinco objetos de vinte mil
package main

import (
	"context"
	"flag"
	"fmt"
	"iter"
	"log"
	"math/rand/v2"
	"time"

	"brevis.sh/brevis/sdk"
	"brevis.sh/brevis/sdk/to"
	"github.com/brevis-sh/stoa/internal/lake"
)

func main() {
	n := flag.Int("n", 100_000, "quantos eventos")
	every := flag.Int("every", 20_000, "eventos por objeto na landing")
	flag.Parse()
	ctx := context.Background()
	cfg := lake.FromEnv()

	if err := cfg.EnsureBucket(ctx); err != nil {
		log.Fatal(err)
	}
	store, err := cfg.Store(ctx)
	if err != nil {
		log.Fatal(err)
	}

	data, err := sdk.Extract(ctx, sdk.Source{From: generator{n: *n}})
	if err != nil {
		log.Fatalf("extract: %v", err)
	}
	res, err := sdk.Load(ctx, data, sdk.Target{
		To:         to.Files{Path: cfg.Landing(), Format: sdk.FormatNDJSON, Store: store},
		FlushEvery: *every,
	})
	if err != nil {
		log.Fatalf("load: %v", err)
	}
	fmt.Printf("seed: %d eventos em %s, %d bytes, %s\n", res.Rows, cfg.Landing(), res.Bytes, res.LoadTime.Round(time.Millisecond))
}

// generator é uma fonte do SDK que inventa eventos: um e-commerce pequeno,
// vinte e oito dias de novembro de 2026, quantidades e valores que somam a
// um número conhecido, para o DuckDB conferir depois.
type generator struct{ n int }

func (g generator) Describe() string { return fmt.Sprintf("generator(%d events)", g.n) }

func (g generator) Read(_ context.Context, _ sdk.ReadOptions) (iter.Seq2[sdk.Envelope, error], error) {
	return func(yield func(sdk.Envelope, error) bool) {
		rnd := rand.New(rand.NewPCG(2026, 11)) // #nosec G404 -- dados sintéticos, reprodutíveis
		hosts := []string{"web-1", "web-2", "api-1", "api-2", "batch"}
		for i := 0; i < g.n; i++ {
			at := time.Date(2026, 11, 1+i%28, rnd.IntN(24), rnd.IntN(60), rnd.IntN(60), 0, time.UTC)
			id := fmt.Sprintf("e-%06d", i)
			ts := at.Format(time.RFC3339)
			row := map[string]any{
				// As quatro colunas de proveniência que o ingestion_id congela.
				"provider":   "stoa",
				"entity":     "events",
				"source_key": id,
				"record_ts":  ts,
				// O evento.
				"event_id":    id,
				"occurred_at": ts,
				"host":        hosts[rnd.IntN(len(hosts))],
				"quantity":    i % 7,
				"amount":      fmt.Sprintf("%d.%02d", rnd.IntN(500), rnd.IntN(100)),
			}
			if !yield(sdk.Envelope{Provider: "stoa", Entity: "events", SourceKey: id, RecordTS: ts, Payload: row}, nil) {
				return
			}
		}
	}, nil
}
