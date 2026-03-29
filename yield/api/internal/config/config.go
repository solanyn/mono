package config

import "os"

type Config struct {
	Port           string
	DatabaseURL    string
	RedisURL       string
	DomainClientID string
	DomainSecret   string
	BlobStore      string
	BlobBucket     string
	BlobPath       string
	S3Endpoint     string
	S3AccessKey    string
	S3SecretKey    string
	S3Region       string
	S3Bucket       string
}

func Load() Config {
	return Config{
		Port:           envOr("PORT", "8080"),
		DatabaseURL:    envOr("DATABASE_URL", "postgres://localhost:5432/yield"),
		RedisURL:       envOr("REDIS_URL", "localhost:6379"),
		DomainClientID: os.Getenv("DOMAIN_CLIENT_ID"),
		DomainSecret:   os.Getenv("DOMAIN_CLIENT_SECRET"),
		BlobStore:      envOr("BLOB_STORE", "disk"),
		BlobBucket:     envOr("BLOB_BUCKET", "yield-blobs"),
		BlobPath:       envOr("BLOB_PATH", "/data/yield-blobs"),
		S3Endpoint:     envOr("S3_ENDPOINT", "http://localhost:3900"),
		S3AccessKey:    os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey:    os.Getenv("S3_SECRET_KEY"),
		S3Region:       envOr("S3_REGION", "us-east-1"),
		S3Bucket:       envOr("S3_BUCKET", "datalake"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
