// Command api serves the lake's metrics: GET /v1/metrics/daily_revenue reads
// the most recent version of gold.daily_revenue through DuckDB, and the Redis
// cache is keyed on the gold table's SNAPSHOT ID. While the lake does not
// move, the answer comes from Redis in microseconds; when `metrics` writes a
// new snapshot the key changes on its own and the first read recomputes. It
// is episode 5's idea brought forward as glue: Iceberg already knows when it
// changed, and no TTL has to guess.
//
// It becomes product when brevis-sql learns the DuckDB dialect; until then,
// this is it.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/apache/iceberg-go/catalog"
	"github.com/brevis-sh/stoa/internal/duck"
	"github.com/brevis-sh/stoa/internal/lake"
	"github.com/redis/go-redis/v9"

	_ "github.com/apache/iceberg-go/io/gocloud/s3" // iceberg-go's S3 backend, a blank import the consumer makes
)

var isDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func main() {
	cfg := lake.FromEnv()
	addr := env("STOA_API_ADDR", ":8082")
	cache := redis.NewClient(&redis.Options{Addr: env("STOA_REDIS", "localhost:56379")})
	api := &api{cfg: cfg, cache: cache}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok\n")) })
	mux.HandleFunc("GET /v1/metrics/daily_revenue", api.dailyRevenue)
	log.Printf("stoa api on %s, redis %s, lake %s", addr, cache.Options().Addr, cfg.S3Endpoint)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(srv.ListenAndServe())
}

type api struct {
	cfg   lake.Config
	cache *redis.Client
}

func (a *api) dailyRevenue(w http.ResponseWriter, r *http.Request) {
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	for name, v := range map[string]string{"from": from, "to": to} {
		if v != "" && !isDate.MatchString(v) {
			http.Error(w, name+" must be YYYY-MM-DD", http.StatusBadRequest)
			return
		}
	}
	ctx := r.Context()
	start := time.Now()

	// The cache's generation is the gold's current snapshot: one metadata read
	// against the catalog, none against S3.
	cat, err := a.cfg.Catalog().Open(ctx)
	if err != nil {
		a.fail(w, err)
		return
	}
	gold, err := cat.LoadTable(ctx, catalog.ToIdentifier("gold", "daily_revenue"))
	if errors.Is(err, catalog.ErrNoSuchTable) || (err == nil && gold.CurrentSnapshot() == nil) {
		http.Error(w, "no gold yet: run the metrics step", http.StatusNotFound)
		return
	}
	if err != nil {
		a.fail(w, err)
		return
	}
	snapshot := strconv.FormatInt(gold.CurrentSnapshot().SnapshotID, 10)
	key := fmt.Sprintf("stoa:daily_revenue:%s:%s:%s", snapshot, from, to)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Lake-Snapshot", snapshot)

	if body, err := a.cache.Get(ctx, key).Bytes(); err == nil {
		w.Header().Set("X-Cache", "hit")
		w.Header().Set("Server-Timing", timing("total", start))
		_, _ = w.Write(body)
		return
	}
	queried := time.Now()
	rows, err := duck.Query(ctx, a.cfg, lake.LatestDailyRevenue(gold.MetadataLocation(), from, to))
	if err != nil {
		a.fail(w, err)
		return
	}
	body, _ := json.Marshal(map[string]any{"snapshot": snapshot, "rows": rows})
	a.cache.Set(ctx, key, body, 24*time.Hour) // the TTL is hygiene only: the key already moves with the lake
	w.Header().Set("X-Cache", "miss")
	w.Header().Set("Server-Timing", timing("duckdb", queried)+", "+timing("total", start))
	_, _ = w.Write(body)
}

func (a *api) fail(w http.ResponseWriter, err error) {
	log.Printf("daily_revenue: %v", err)
	http.Error(w, "the lake did not answer; see the api log", http.StatusBadGateway)
}

func timing(name string, since time.Time) string {
	return fmt.Sprintf("%s;dur=%.1f", name, float64(time.Since(since).Microseconds())/1000)
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
