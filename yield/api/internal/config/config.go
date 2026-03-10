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
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
