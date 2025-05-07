package router

import (
	"net/http"
	"tldr/internal/config"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httplog/v2"
)

func New(cfg config.Config, logger *httplog.Logger) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(httplog.RequestLogger(logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Heartbeat("/healthz"))
	r.Use(middleware.Heartbeat("/readyz"))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedCORS,
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   []string{"Accept", "Content-Type"},
		AllowCredentials: false,
	}))

	r.Mount("/api", NewsRouter(cfg, logger))

	return r
}
