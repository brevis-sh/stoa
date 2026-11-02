// Package duck é o DuckDB como processo: o CLI pinado, chamado com -json,
// lendo o lake pelo caminho do metadata.json. Sem cgo, de propósito: o
// container do scheduler é Alpine, e um CLI estático é o que roda em qualquer
// lugar em que o step rode. Quando o DuckDB virar produto no brevis-sql, este
// pacote some.
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

// Binary acha o CLI: STOA_DUCKDB, depois `duckdb` ao lado do executável,
// depois o PATH. O `make build` deixa um em bin/, ao lado dos steps.
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
	return "", fmt.Errorf("o CLI do DuckDB não foi encontrado: STOA_DUCKDB, bin/duckdb ou o PATH")
}

func isExecutable(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir() && st.Mode()&0o111 != 0
}

// Preamble é o que toda consulta ao lake começa dizendo: as extensões e o
// segredo S3, nos termos do DuckDB (host:port, path style, sem TLS local).
func Preamble(cfg lake.Config) string {
	host := strings.TrimPrefix(strings.TrimPrefix(cfg.S3Endpoint, "http://"), "https://")
	ssl := "false"
	if strings.HasPrefix(cfg.S3Endpoint, "https://") {
		ssl = "true"
	}
	// UTC, sempre: um CAST(ts AS DATE) segue o fuso do processo, e o dia de um
	// evento não pode depender da máquina em que o DuckDB rodou.
	return fmt.Sprintf(`INSTALL iceberg; LOAD iceberg; INSTALL httpfs; LOAD httpfs; SET TimeZone='UTC';
CREATE SECRET lake (TYPE s3, KEY_ID %s, SECRET %s, REGION %s, ENDPOINT %s, URL_STYLE 'path', USE_SSL %s);`,
		quote(cfg.S3Key), quote(cfg.S3Secret), quote(cfg.S3Region), quote(host), ssl)
}

// quote é um literal SQL: aspas simples dobradas.
func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// Query roda o preâmbulo e a consulta e devolve as linhas da consulta. O CLI
// imprime um array JSON por comando que devolve linhas -- o CREATE SECRET
// devolve `[{"Success":true}]` -- e o da consulta é o último; um array com
// mais de uma linha vem quebrado em várias linhas, então a saída é lida como
// uma sequência de valores JSON, não linha a linha. Decimais devem vir como
// texto (`::VARCHAR`) para não perder casas.
func Query(ctx context.Context, cfg lake.Config, sql string) ([]map[string]any, error) {
	bin, err := Binary()
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, bin, "-json", "-cmd", Preamble(cfg), "-c", sql) // #nosec G204 -- o binário é o nosso CLI pinado e o SQL é nosso
	// HOME é onde o DuckDB guarda as extensões; o do usuário do container pode
	// não existir ou não ser gravável, e um diretório temporário é dos dois.
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

// lastValue é o último valor JSON da saída: a resposta da consulta.
func lastValue(out []byte) ([]map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(out))
	dec.UseNumber()
	var last []map[string]any
	for dec.More() {
		var rows []map[string]any
		if err := dec.Decode(&rows); err != nil {
			return nil, fmt.Errorf("duckdb: saída não é JSON: %w", err)
		}
		last = rows
	}
	return last, nil
}
