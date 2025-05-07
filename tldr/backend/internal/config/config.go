package config

import (
	"os"
	"strings"
)

type Config struct {
	MinIO       MinIOConfig
	ServerHost  string
	ServerPort  string
	AllowedCORS []string
}

type MinIOConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

func Load() Config {
	return Config{
		MinIO: MinIOConfig{
			Endpoint:  os.Getenv("MINIO_ENDPOINT"),
			AccessKey: os.Getenv("MINIO_ACCESS_KEY"),
			SecretKey: os.Getenv("MINIO_SECRET_KEY"),
			Bucket:    os.Getenv("MINIO_BUCKET"),
			UseSSL:    os.Getenv("MINIO_USE_SSL") == "true",
		},
		ServerHost:  getEnv("SERVER_HOST", "0.0.0.0"),
		ServerPort:  getEnv("SERVER_PORT", "8080"),
		AllowedCORS: parseCORS(os.Getenv("CORS_ALLOWED_ORIGINS")),
	}
}

func getEnv(key, defaultVal string) string {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	return val
}

func parseCORS(origins string) []string {
	if origins == "" {
		return []string{}
	}
	return strings.Split(origins, ",")
}
