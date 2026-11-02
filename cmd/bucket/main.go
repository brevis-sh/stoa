// Command bucket cria o bucket do lake se ele não existir. O iceberg-go e o
// gateway escrevem num bucket e nenhum dos dois o cria; um SeaweedFS recém-
// subido não tem nenhum. O `make up` chama isto depois que o S3 responde.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/brevis-sh/stoa/internal/lake"
)

func main() {
	cfg := lake.FromEnv()
	if err := cfg.EnsureBucket(context.Background()); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("bucket %s pronto em %s\n", cfg.Bucket, cfg.S3Endpoint)
}
