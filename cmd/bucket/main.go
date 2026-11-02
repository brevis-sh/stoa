// Command bucket creates the lake's bucket when it does not exist. iceberg-go
// and the gateway write into a bucket and neither of them creates one; a
// freshly started SeaweedFS has none. `make up` calls this once the S3
// answers.
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
	fmt.Printf("bucket %s ready at %s\n", cfg.Bucket, cfg.S3Endpoint)
}
