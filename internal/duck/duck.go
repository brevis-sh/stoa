// Package duck is DuckDB as a process: the pinned CLI, called with -json,
// reading the lake by its metadata.json path. No cgo, on purpose: the
// scheduler's container is Alpine, and a statically linked CLI is what runs
// everywhere a step runs. When DuckDB becomes product in brevis-sql, this
// package goes away.
package duck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/brevis-sh/stoa/internal/lake"
)

// Binary finds the CLI: STOA_DUCKDB, then `duckdb` next to the executable,
// then the PATH. `make build` leaves one in bin/, beside the steps.
func Binary() (string, error) {
	if v := os.Getenv("STOA_DUCKDB"); v != "" {
		return v, nil
	}
	if exe, err := os.Executable(); err == nil {
		if p := filepath.Join(filepath.Dir(exe), "duckdb"); isExecutable(p) {
			return p, nil
		}
	}
	if p, err := exec.LookPath("duckdb"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("the DuckDB CLI was not found: STOA_DUCKDB, bin/duckdb or the PATH")
}

func isExecutable(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir() && st.Mode()&0o111 != 0
}

// Preamble is what every query against the lake opens with: the extensions
// and the S3 secret, in DuckDB's terms (host:port, path style, no local TLS).
func Preamble(cfg lake.Config) string {
	host := strings.TrimPrefix(strings.TrimPrefix(cfg.S3Endpoint, "http://"), "https://")
	ssl := "false"
	if strings.HasPrefix(cfg.S3Endpoint, "https://") {
		ssl = "true"
	}
	// UTC, always: a CAST(ts AS DATE) follows the process's timezone, and an
	// event's day must not depend on the machine DuckDB ran on.
	return fmt.Sprintf(`INSTALL iceberg; LOAD iceberg; INSTALL httpfs; LOAD httpfs; SET TimeZone='UTC';
CREATE SECRET lake (TYPE s3, KEY_ID %s, SECRET %s, REGION %s, ENDPOINT %s, URL_STYLE 'path', USE_SSL %s);`,
		quote(cfg.S3Key), quote(cfg.S3Secret), quote(cfg.S3Region), quote(host), ssl)
}

// quote is a SQL literal: single quotes doubled.
func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// Query runs the preamble and the query and gives back the query's rows. The
// CLI prints one JSON array per command that returns rows -- CREATE SECRET
// returns `[{"Success":true}]` -- and the query's is the last one; an array
// of more than one row is printed across several lines, so the output is read
// as a sequence of JSON values rather than line by line. Decimals have to
// come back as text (`::VARCHAR`) so no digits are lost.
func Query(ctx context.Context, cfg lake.Config, sql string) ([]map[string]any, error) {
	bin, err := Binary()
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, bin, "-json", "-cmd", Preamble(cfg), "-c", sql) // #nosec G204 -- the binary is our pinned CLI and the SQL is ours
	// HOME is where DuckDB keeps its extensions; the container user's may not
	// exist or may not be writable, and a temporary directory is both.
	home := filepath.Join(os.TempDir(), "stoa-duckdb")
	_ = os.MkdirAll(home, 0o700)
	cmd.Env = append(os.Environ(), "HOME="+home)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("duckdb: %w: %s", err, strings.TrimSpace(errb.String()))
	}
	return lastValue(out.Bytes())
}

// lastValue is the last JSON value in the output: the query's answer.
func lastValue(out []byte) ([]map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(out))
	dec.UseNumber()
	var last []map[string]any
	for dec.More() {
		var rows []map[string]any
		if err := dec.Decode(&rows); err != nil {
			return nil, fmt.Errorf("duckdb: the output is not JSON: %w", err)
		}
		last = rows
	}
	return last, nil
}
