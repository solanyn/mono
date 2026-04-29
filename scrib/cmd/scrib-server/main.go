// scrib-server entrypoint — standalone binary for k8s deployment
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

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
		AudioProcessURL: envOr("SCRIB_AUDIO_PROCESS_URL", ""),
		STTModel:        envOr("SCRIB_STT_MODEL", "mlx-community/whisper-large-v3-turbo"),
		VADThreshold:    envOr("SCRIB_VAD_THRESHOLD", "0.5"),
		MergeGap:        envOr("SCRIB_MERGE_GAP", "0.5"),
		MaxAudioBytes:   envBytes("SCRIB_MAX_AUDIO_BYTES", 0),
	}

	if cfg.DatabaseURL == "" {
		fmt.Fprintln(os.Stderr, "SCRIB_DATABASE_URL required")
		os.Exit(1)
	}

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("init server: %v", err)
	}

	httpSrv := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: srv.Router(),
		// ReadTimeout intentionally 0: uploads can take minutes. Upload size is
		// capped by Server.handleUploadAudio via MaxBytesReader.
		WriteTimeout:      0,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("scrib-server listening on %s", cfg.ListenAddr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-stop
	log.Printf("shutting down")

	shutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutCtx); err != nil {
		log.Printf("http shutdown: %v", err)
	}
	srv.Shutdown(30 * time.Second)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBytes(key string, fallback int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		log.Printf("%s: invalid int64 %q, using fallback", key, v)
		return fallback
	}
	return n
}
