// scrib-server entrypoint — standalone binary for k8s deployment
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/solanyn/mono/scrib/server"
)

func main() {
	cfg := server.Config{
		ListenAddr:  envOr("SCRIB_LISTEN", ":8090"),
		DatabaseURL: envOr("SCRIB_DATABASE_URL", ""),
		S3Endpoint:  envOr("SCRIB_S3_ENDPOINT", "s3.goyangi.io"),
		S3Bucket:    envOr("SCRIB_S3_BUCKET", "scrib"),
		S3AccessKey: envOr("SCRIB_S3_ACCESS_KEY", ""),
		S3SecretKey: envOr("SCRIB_S3_SECRET_KEY", ""),
		S3UseSSL:        envOr("SCRIB_S3_USE_SSL", "true") == "true",
		AudioServiceURL: envOr("SCRIB_AUDIO_URL", "http://localhost:8001"),
		STTModel:        envOr("SCRIB_STT_MODEL", "mlx-community/whisper-large-v3-turbo"),
		VADThreshold:    envOr("SCRIB_VAD_THRESHOLD", "0.5"),
	}

	if cfg.DatabaseURL == "" {
		fmt.Fprintln(os.Stderr, "SCRIB_DATABASE_URL required")
		os.Exit(1)
	}

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("init server: %v", err)
	}

	log.Fatal(srv.ListenAndServe())
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
