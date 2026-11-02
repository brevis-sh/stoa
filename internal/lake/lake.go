// Package lake é a configuração do lake do stoa, lida do ambiente uma vez, com
// os defaults que o docker-compose.yml deste repositório sobe. Um step lê
// daqui e não espalha endereços pelo código.
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

// Config é o lake: um S3 e um Postgres.
type Config struct {
	S3Endpoint  string // STOA_S3_ENDPOINT
	S3Region    string // STOA_S3_REGION
	S3Key       string // STOA_S3_KEY
	S3Secret    string // STOA_S3_SECRET
	Bucket      string // STOA_BUCKET
	PostgresDSN string // STOA_PG_DSN: o catálogo Iceberg
}

// FromEnv lê o ambiente, com os defaults do compose.
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

// Landing é onde o gateway deixa o NDJSON que aceitou.
func (c Config) Landing() string { return "s3://" + c.Bucket + "/landing/events/" }

// Warehouse é onde o Iceberg põe os arquivos das tabelas.
func (c Config) Warehouse() string { return "s3://" + c.Bucket + "/warehouse" }

// Client é um cliente S3 apontado para o endpoint do lake, em path style,
// que é o que SeaweedFS, MinIO e s3mock falam.
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

// Store é o backend que from.Files e to.Files recebem.
func (c Config) Store(ctx context.Context) (s3store.Store, error) {
	client, err := c.Client(ctx)
	if err != nil {
		return s3store.Store{}, err
	}
	return s3store.New(client), nil
}

// EnsureBucket cria o bucket se não existir. O iceberg-go escreve num bucket e
// nunca o cria; um SeaweedFS recém-subido não tem nenhum.
func (c Config) EnsureBucket(ctx context.Context) error {
	client, err := c.Client(ctx)
	if err != nil {
		return err
	}
	_, err = client.CreateBucket(ctx, &awss3.CreateBucketInput{Bucket: aws.String(c.Bucket)})
	if err != nil && !strings.Contains(err.Error(), "BucketAlreadyOwnedByYou") && !strings.Contains(err.Error(), "BucketAlreadyExists") {
		return fmt.Errorf("criando o bucket %s em %s: %w", c.Bucket, c.S3Endpoint, err)
	}
	return nil
}

// Catalog é o catálogo Iceberg do lake: SQL sobre o Postgres, aberto uma vez
// por processo. O Name é o primeiro segmento de iceberg://lake/bronze/events
// e, no catálogo SQL, também o catalog_name gravado no Postgres.
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

// Events é a tabela bronze.events: o evento como o produtor o envia, mais as
// duas colunas que o gateway escreve ao aceitar -- ingestion_id, a identidade
// congelada sobre provider|entity|source_key|record_ts, e ingestion_loaded_at,
// a hora da aceitação. Declarada, nunca inferida: é a partir daqui que a
// tabela é criada no primeiro `land`.
var Events = sdk.Schema{
	{Name: "event_id", Type: sdk.TypeString, Required: true},
	{Name: "occurred_at", Type: sdk.TypeTimestamp, Required: true},
	{Name: "host", Type: sdk.TypeString},
	{Name: "quantity", Type: sdk.TypeInt64},
	{Name: "amount", Type: sdk.TypeNumeric},
	{Name: sdk.ColumnIngestionID, Type: sdk.TypeString, Required: true},
	{Name: sdk.ColumnIngestionLoadedAt, Type: sdk.TypeTimestamp, Required: true},
}
