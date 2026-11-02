// Package lake is the stoa lake's configuration, read from the environment
// once, with the defaults the docker-compose.yml in this repository brings up.
// A step reads from here instead of spreading addresses through the code.
package lake

import (
	"context"
	"fmt"
	"os"
	"strings"

	"brevis.sh/brevis/sdk"
	"brevis.sh/brevis/sdk/iceberg"
	s3store "brevis.sh/brevis/sdk/store/s3"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

// Config is the lake: an S3 and a Postgres.
type Config struct {
	S3Endpoint  string // STOA_S3_ENDPOINT
	S3Region    string // STOA_S3_REGION
	S3Key       string // STOA_S3_KEY
	S3Secret    string // STOA_S3_SECRET
	Bucket      string // STOA_BUCKET
	PostgresDSN string // STOA_PG_DSN: the Iceberg catalog
}

// FromEnv reads the environment, with the compose file's defaults.
func FromEnv() Config {
	return Config{
		S3Endpoint:  env("STOA_S3_ENDPOINT", "http://localhost:8333"),
		S3Region:    env("STOA_S3_REGION", "us-east-1"),
		S3Key:       env("STOA_S3_KEY", "stoa"),
		S3Secret:    env("STOA_S3_SECRET", "stoa-secret"),
		Bucket:      env("STOA_BUCKET", "lake"),
		PostgresDSN: env("STOA_PG_DSN", "postgres://stoa:stoa@localhost:55439/lake?sslmode=disable"),
	}
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

// Landing is where the gateway leaves the NDJSON it accepted.
func (c Config) Landing() string { return "s3://" + c.Bucket + "/landing/events/" }

// Warehouse is where Iceberg puts the tables' files.
func (c Config) Warehouse() string { return "s3://" + c.Bucket + "/warehouse" }

// Client is an S3 client pointed at the lake's endpoint, in path style,
// which is what SeaweedFS, MinIO and s3mock speak.
func (c Config) Client(ctx context.Context) (*awss3.Client, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(c.S3Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(c.S3Key, c.S3Secret, "")),
	)
	if err != nil {
		return nil, err
	}
	return awss3.NewFromConfig(cfg, func(o *awss3.Options) {
		o.BaseEndpoint = aws.String(c.S3Endpoint)
		o.UsePathStyle = true
	}), nil
}

// Store is the backend from.Files and to.Files are handed.
func (c Config) Store(ctx context.Context) (s3store.Store, error) {
	client, err := c.Client(ctx)
	if err != nil {
		return s3store.Store{}, err
	}
	return s3store.New(client), nil
}

// EnsureBucket creates the bucket when it does not exist. iceberg-go writes
// into a bucket and never creates one; a freshly started SeaweedFS has none.
func (c Config) EnsureBucket(ctx context.Context) error {
	client, err := c.Client(ctx)
	if err != nil {
		return err
	}
	_, err = client.CreateBucket(ctx, &awss3.CreateBucketInput{Bucket: aws.String(c.Bucket)})
	if err != nil && !strings.Contains(err.Error(), "BucketAlreadyOwnedByYou") && !strings.Contains(err.Error(), "BucketAlreadyExists") {
		return fmt.Errorf("creating bucket %s at %s: %w", c.Bucket, c.S3Endpoint, err)
	}
	return nil
}

// Catalog is the lake's Iceberg catalog: SQL over Postgres, opened once per
// process. The Name is the first segment of iceberg://lake/bronze/events and,
// in a SQL catalog, also the catalog_name recorded in Postgres.
func (c Config) Catalog() iceberg.Catalog {
	return iceberg.Catalog{
		Name:      "lake",
		Type:      iceberg.SQL,
		DSN:       c.PostgresDSN,
		Warehouse: c.Warehouse(),
		IO: map[string]string{
			"s3.endpoint":                 c.S3Endpoint,
			"s3.region":                   c.S3Region,
			"s3.access-key-id":            c.S3Key,
			"s3.secret-access-key":        c.S3Secret,
			"s3.force-virtual-addressing": "false",
		},
	}
}

// Events is the bronze.events table: the event as the producer sends it, plus
// the two columns the gateway writes on acceptance -- ingestion_id, the
// identity frozen over provider|entity|source_key|record_ts, and
// ingestion_loaded_at, the moment of acceptance. Declared, never inferred:
// this is what the first `land` creates the table from.
var Events = sdk.Schema{
	{Name: "event_id", Type: sdk.TypeString, Required: true},
	{Name: "occurred_at", Type: sdk.TypeTimestamp, Required: true},
	{Name: "host", Type: sdk.TypeString},
	{Name: "quantity", Type: sdk.TypeInt64},
	{Name: "amount", Type: sdk.TypeNumeric},
	{Name: sdk.ColumnIngestionID, Type: sdk.TypeString, Required: true},
	{Name: sdk.ColumnIngestionLoadedAt, Type: sdk.TypeTimestamp, Required: true},
}
