// Command seed is a producer: it invents events and sends them to the gateway
// in NDJSON batches, exactly as a service would with a POST. The gateway
// answers 202, stamps the identity and leaves the landing on S3; `land` does
// the rest.
//
//	go run ./cmd/seed -n 100000 -batch 20000     # five POSTs of twenty thousand lines
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"time"
)

func main() {
	n := flag.Int("n", 100_000, "how many events")
	batch := flag.Int("batch", 20_000, "events per POST")
	gateway := flag.String("gateway", env("STOA_GATEWAY", "http://localhost:8081"), "the gateway")
	flag.Parse()

	rnd := rand.New(rand.NewPCG(2026, 11)) // #nosec G404 -- synthetic data, meant to be reproducible
	hosts := []string{"web-1", "web-2", "api-1", "api-2", "batch"}
	client := &http.Client{Timeout: 60 * time.Second}
	start := time.Now()
	sent := 0
	for sent < *n {
		size := min(*batch, *n-sent)
		var body bytes.Buffer
		enc := json.NewEncoder(&body)
		for i := sent; i < sent+size; i++ {
			at := time.Date(2026, 11, 1+i%28, rnd.IntN(24), rnd.IntN(60), rnd.IntN(60), 0, time.UTC)
			_ = enc.Encode(map[string]any{
				"event_id":    fmt.Sprintf("e-%06d", i),
				"occurred_at": at.Format(time.RFC3339),
				"host":        hosts[rnd.IntN(len(hosts))],
				"quantity":    1 + rnd.IntN(5), // not `i % 7`: 28 days is a multiple of 7, which made the quantity constant per day
				"amount":      fmt.Sprintf("%d.%02d", rnd.IntN(500), rnd.IntN(100)),
			})
		}
		res, err := client.Post(*gateway+"/v1/events", "application/x-ndjson", &body)
		if err != nil {
			log.Fatalf("POST %s/v1/events: %v", *gateway, err)
		}
		msg, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if res.StatusCode != http.StatusAccepted {
			log.Fatalf("POST %s/v1/events: %s: %s", *gateway, res.Status, bytes.TrimSpace(msg))
		}
		sent += size
		fmt.Printf("  202 · %d events (%d/%d)\n", size, sent, *n)
	}
	fmt.Printf("seed: %d events accepted by the gateway in %s\n", sent, time.Since(start).Round(time.Millisecond))
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
