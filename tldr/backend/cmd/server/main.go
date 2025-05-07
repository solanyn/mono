package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"time"
	"tldr/internal/config"
	"tldr/internal/router"

	"github.com/go-chi/httplog/v2"
)

func main() {
	cfg := config.Load()

	logger := httplog.NewLogger("tldr", httplog.Options{
		JSON:     true,
		LogLevel: slog.LevelInfo,
		Concise:  true,
		QuietDownRoutes: []string{
			"/healthz",
			"/readyz",
		},
		QuietDownPeriod: 10 & time.Second,
	})

	r := router.New(cfg, logger)

	addr := fmt.Sprintf("%s:%s", cfg.ServerHost, cfg.ServerPort)
	logger.Info(fmt.Sprintf("Starting server on %s", addr))
	log.Fatal(http.ListenAndServe(addr, r))
}
