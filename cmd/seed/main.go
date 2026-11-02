// Command seed é um produtor: inventa eventos e os envia ao gateway, em lotes
// NDJSON, exatamente como um serviço faria com um POST. O gateway responde
// 202, carimba a identidade e deixa a landing no S3; o `land` faz o resto.
//
//	go run ./cmd/seed -n 100000 -batch 20000     # cinco POSTs de vinte mil linhas
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
	n := flag.Int("n", 100_000, "quantos eventos")
	batch := flag.Int("batch", 20_000, "eventos por POST")
	gateway := flag.String("gateway", env("STOA_GATEWAY", "http://localhost:8081"), "o gateway")
	flag.Parse()

	rnd := rand.New(rand.NewPCG(2026, 11)) // #nosec G404 -- dados sintéticos, reprodutíveis
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
				"quantity":    i % 7,
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
		fmt.Printf("  202 · %d eventos (%d/%d)\n", size, sent, *n)
	}
	fmt.Printf("seed: %d eventos aceitos pelo gateway em %s\n", sent, time.Since(start).Round(time.Millisecond))
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
